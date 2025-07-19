import React, { useEffect, useState, useRef, useCallback } from 'react'
import { Check, Loader, RefreshCw } from 'lucide-react'

import { useTheme } from '@renderer/lib/theme'
import { useGoServer } from '@renderer/hooks/useGoServer'
import { formatBytes, initialDownloadState, DEPENDENCY_CONFIG, DEPENDENCY_NAMES } from './util'
import { Button } from '../ui/button'
import FreysaLoading from '@renderer/assets/icons/freysaLoading.png'
import { useLlamaCpp } from '@renderer/hooks/useLlamaCpp'

export type DependencyName = 'embeddings' | 'anonymizer' | 'onnx' | 'LLAMACCP'

interface ModelDownloadItemProps {
  name: string
  description: string
  completed: boolean
  downloading: boolean
  percentage: number
  totalBytes: number
  downloadedBytes: number
  error?: string
  onRetry?: () => void
}

export type DownloadState = Record<
  DependencyName,
  {
    downloading: boolean
    percentage: number
    completed: boolean
    totalBytes: number
    downloadedBytes: number
    error?: string
  }
>
const handleDependencyDownload = (
  dependencyName: DependencyName,
  isDownloaded: boolean,
  setDownloadState: React.Dispatch<React.SetStateAction<DownloadState>>,
  onError?: (error: Error) => void
): boolean => {
  if (!isDownloaded) {
    window.api.models.downloadModels(dependencyName).catch((error) => {
      setDownloadState((prev) => ({
        ...prev,
        [dependencyName]: {
          ...prev[dependencyName as keyof typeof prev],
          downloading: false,
          error: error instanceof Error ? error.message : 'Download failed'
        }
      }))
      onError?.(error)
    })

    setDownloadState((prev) => ({
      ...prev,
      [dependencyName]: {
        downloading: true,
        percentage: 0,
        completed: false,
        totalBytes: 0,
        downloadedBytes: 0
      }
    }))

    return true
  } else {
    setDownloadState((prev) => ({
      ...prev,
      [dependencyName]: {
        ...prev[dependencyName as keyof typeof prev],
        completed: true,
        downloading: false,
        percentage: 100
      }
    }))

    return false
  }
}

export default function DependenciesGate({ children }: { children: React.ReactNode }) {
  const { theme } = useTheme()
  const { start: startLlamaCpp } = useLlamaCpp()
  const [hasModelsDownloaded, setHasModelsDownloaded] = useState<Record<DependencyName, boolean>>(
    DEPENDENCY_NAMES.reduce(
      (acc, dep) => {
        acc[dep] = false
        return acc
      },
      {} as Record<DependencyName, boolean>
    )
  )
  const [downloadState, setDownloadState] = useState<DownloadState>(initialDownloadState)

  const { state: goServerState, initializeIfNeeded, retry: retryGoServer } = useGoServer()
  const hasInitializedGoServer = useRef(false)

  const retryDownload = useCallback(async (modelName: DependencyName) => {
    setDownloadState((prev) => ({
      ...prev,
      [modelName]: {
        ...prev[modelName],
        error: undefined,
        downloading: true,
        percentage: 0,
        completed: false
      }
    }))

    try {
      await window.api.models.downloadModels(modelName)
    } catch (error) {
      setDownloadState((prev) => ({
        ...prev,
        [modelName]: {
          ...prev[modelName],
          downloading: false,
          error: error instanceof Error ? error.message : 'Download failed'
        }
      }))
    }
  }, [])

  useEffect(() => {
    const handleProgress = (data: {
      modelName: string
      pct: number
      totalBytes?: number
      downloadedBytes?: number
      error?: string
    }) => {
      setDownloadState((prev) => {
        const newState = {
          ...prev,
          [data.modelName]: {
            ...prev[data.modelName as keyof typeof prev],
            downloading: data.pct < 100,
            percentage: data.pct,
            completed: data.pct === 100,
            totalBytes: data.totalBytes || 0,
            downloadedBytes: data.downloadedBytes || 0,
            error: data.error
          }
        }

        const allCompleted = Object.values(newState).every((dependency) => dependency.completed)

        if (allCompleted && !hasInitializedGoServer.current) {
          hasInitializedGoServer.current = true
          initializeIfNeeded()
        }

        return newState
      })

      if (data.pct === 100 && !data.error) {
        setHasModelsDownloaded((prev) => ({
          ...prev,
          [data.modelName]: true
        }))
      }
    }

    const cleanup = window.api.models.onProgress(handleProgress)

    window.api.models.hasModelsDownloaded().then((hasModelsDownloaded) => {
      console.log('hasModelsDownloaded', hasModelsDownloaded)

      setHasModelsDownloaded(hasModelsDownloaded)

      let needsDownload = false

      const downloadNeeds = DEPENDENCY_NAMES.map((dependencyName) =>
        handleDependencyDownload(
          dependencyName,
          hasModelsDownloaded[dependencyName],
          setDownloadState
        )
      )

      needsDownload = downloadNeeds.some(Boolean)

      if (!needsDownload && !hasInitializedGoServer.current) {
        hasInitializedGoServer.current = true
        initializeIfNeeded()
      }
    })

    return () => {
      cleanup()
      hasInitializedGoServer.current = false
    }
  }, [])

  useEffect(() => {
    const interval = setInterval(async () => {
      if (!hasModelsDownloaded.LLAMACCP) return
      const result = await window.api.llamacpp.getStatus()
      if (result.success) {
        if (!result.isRunning && !result.setupInProgress) {
          console.log('[LlamaCpp] Server was not running, automatically starting it...')
          startLlamaCpp()
        }
      }
    }, 15000)

    return () => clearInterval(interval)
  }, [startLlamaCpp, hasModelsDownloaded.LLAMACCP])

  const allDependenciesCompleted =
    Object.values(hasModelsDownloaded).every((dependency) => dependency) ||
    Object.values(downloadState).every((dependency) => dependency.completed)

  if (allDependenciesCompleted && goServerState.isRunning) {
    return <>{children}</>
  }

  return (
    <div className="flex flex-col h-screen w-screen">
      <div className="titlebar text-center fixed top-0 left-0 right-0 text-muted-foreground text-xs h-8 z-20 flex items-center justify-center backdrop-blur-sm" />
      <div
        className="flex-1 flex items-center justify-center"
        style={{
          background:
            theme === 'light'
              ? 'linear-gradient(180deg, #6068E9 0%, #A5AAF9 100%)'
              : 'linear-gradient(180deg, #18181B 0%, #000 100%)'
        }}
      >
        <div className="flex flex-col gap-12 text-primary-foreground p-10 border border-white/50 rounded-lg bg-white/5 min-w-2xl">
          <div className="flex flex-col gap-1 text-center items-center">
            <img src={FreysaLoading} alt="Enchanted" className="w-16 h-16" />
            <h1 className="text-lg font-normal text-white">
              {allDependenciesCompleted ? 'It begins with Freysa' : 'Enchanted is loading'}
            </h1>
            {!allDependenciesCompleted && <p className="text-xs text-white">~5 minutes</p>}
          </div>

          <div className="flex flex-col gap-4">
            {!allDependenciesCompleted ? (
              <>
                {DEPENDENCY_NAMES.map((dependencyName, index) => (
                  <React.Fragment key={dependencyName}>
                    {index > 0 && <div className="h-px bg-white/35"></div>}
                    <ModelDownloadItem
                      name={DEPENDENCY_CONFIG[dependencyName].name}
                      description={DEPENDENCY_CONFIG[dependencyName].description}
                      completed={
                        hasModelsDownloaded[dependencyName] ||
                        downloadState[dependencyName].completed
                      }
                      downloading={downloadState[dependencyName].downloading}
                      percentage={downloadState[dependencyName].percentage}
                      totalBytes={downloadState[dependencyName].totalBytes}
                      downloadedBytes={downloadState[dependencyName].downloadedBytes}
                      error={downloadState[dependencyName].error}
                      onRetry={() => retryDownload(dependencyName)}
                    />
                  </React.Fragment>
                ))}
              </>
            ) : (
              <GoServerSetup
                completed={goServerState.isRunning}
                error={goServerState.error}
                onRetry={retryGoServer}
              />
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

function ModelDownloadItem({
  name,
  description,
  completed,
  downloading,
  percentage,
  totalBytes,
  downloadedBytes,
  error,
  onRetry
}: ModelDownloadItemProps) {
  const remainingBytes = totalBytes - downloadedBytes
  const remainingSize = formatBytes(remainingBytes)
  const totalSize = formatBytes(totalBytes)

  return (
    <div className="flex justify-between pb-2 px-3">
      <div className="flex flex-col">
        <h1 className="text-md font-normal text-white">{name}</h1>
        <p className="text-xs text-white/75">{description}</p>
        {error && <p className="text-xs text-red-300 mt-1">{error}</p>}
      </div>

      <div className="flex flex-col justify-center gap-1">
        {completed ? (
          <p className="text-md">
            <Check className="text-white" />
          </p>
        ) : downloading ? (
          <>
            <div className="flex items-center justify-end gap-2 text-white">
              <Loader className="animate-spin w-4 h-4 " />
              <p className="text-md">{percentage}%</p>
            </div>
            <p className="text-xs text-white/70">
              {remainingSize} left of {totalSize}
            </p>
          </>
        ) : error ? (
          <button
            onClick={onRetry}
            className="flex items-center gap-2 text-xs text-white/75 hover:text-white transition-colors"
          >
            <RefreshCw className="w-3 h-3 text-white" />
            Retry
          </button>
        ) : (
          <p className="text-md text-white">Pending</p>
        )}
      </div>
    </div>
  )
}

function GoServerSetup({
  completed,
  error,
  onRetry
}: {
  completed: boolean
  error?: string
  onRetry?: () => void
}) {
  return (
    <div className="flex flex-col  items-center justify-center gap-3">
      <div className="flex flex-col gap-2 items-center max-w-sm">
        {error && <p className="text-sm text-red-300 text-center">{error}</p>}

        {error ? (
          <Button
            onClick={onRetry}
            variant="outline"
            className="flex items-center gap-2 text-xs text-white/75 hover:text-white transition-colors"
          >
            <RefreshCw className="w-3 h-3 text-white" />
            Retry
          </Button>
        ) : completed ? (
          <p className="text-md text-white">
            <Check className="text-white" />
          </p>
        ) : (
          <Loader className="animate-spin w-8 h-8 text-white" />
        )}
      </div>
    </div>
  )
}
