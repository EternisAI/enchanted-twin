import React, { useEffect, useState, useRef, useCallback } from 'react'
import { Check, Loader, RefreshCw } from 'lucide-react'

import { useTheme } from '@renderer/lib/theme'
import { useGoServer } from '@renderer/hooks/useGoServer'
import { formatBytes, initialDownloadState } from './util'
import { Button } from '../ui/button'

const handleModelDownload = (
  modelName: 'embeddings' | 'anonymizer' | 'onnx',
  isDownloaded: boolean,
  setDownloadState: React.Dispatch<React.SetStateAction<DownloadState>>,
  onError?: (error: Error) => void
): boolean => {
  if (!isDownloaded) {
    window.api.models.downloadModels(modelName).catch((error) => {
      setDownloadState((prev) => ({
        ...prev,
        [modelName]: {
          ...prev[modelName],
          downloading: false,
          error: error instanceof Error ? error.message : 'Download failed'
        }
      }))
      onError?.(error)
    })

    setDownloadState((prev) => ({
      ...prev,
      [modelName]: {
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
      [modelName]: {
        ...prev[modelName],
        completed: true,
        downloading: false,
        percentage: 100
      }
    }))

    return false
  }
}

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

type DownloadState = Record<
  'embeddings' | 'anonymizer' | 'onnx',
  {
    downloading: boolean
    percentage: number
    completed: boolean
    totalBytes: number
    downloadedBytes: number
    error?: string
  }
>

export default function AppSetupGate({ children }: { children: React.ReactNode }) {
  const { theme } = useTheme()
  const [hasModelsDownloaded, setHasModelsDownloaded] = useState<{
    embeddings: boolean
    anonymizer: boolean
    onnx: boolean
  }>({ embeddings: false, anonymizer: false, onnx: false })

  const [downloadState, setDownloadState] = useState<DownloadState>(initialDownloadState)

  const { state: goServerState, initializeIfNeeded, retry: retryGoServer } = useGoServer()
  const hasInitializedGoServer = useRef(false)

  const retryModel = useCallback(async (modelName: 'embeddings' | 'anonymizer' | 'onnx') => {
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

        const allCompleted =
          newState.embeddings.completed && newState.anonymizer.completed && newState.onnx.completed

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
      setHasModelsDownloaded(hasModelsDownloaded)

      let needsDownload = false

      const needsEmbeddings = handleModelDownload(
        'embeddings',
        hasModelsDownloaded.embeddings,
        setDownloadState
      )
      const needsAnonymizer = handleModelDownload(
        'anonymizer',
        hasModelsDownloaded.anonymizer,
        setDownloadState
      )
      const needsOnnx = handleModelDownload('onnx', hasModelsDownloaded.onnx, setDownloadState)

      needsDownload = needsEmbeddings || needsAnonymizer || needsOnnx

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

  const allDependenciesCompleted =
    downloadState.anonymizer.completed &&
    downloadState.embeddings.completed &&
    downloadState.onnx.completed

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
          <div className="flex flex-col gap-1 text-center">
            <h1 className="text-lg font-normal text-white">
              {allDependenciesCompleted
                ? 'Starting Enchanted'
                : 'Enchanted is getting ready for you to use.'}
            </h1>
            {!allDependenciesCompleted && (
              <p className="text-xs text-white">Takes ~5 minutes of download</p>
            )}
          </div>

          <div className="flex flex-col gap-4">
            {!allDependenciesCompleted ? (
              <>
                <ModelDownloadItem
                  name="Embeddings model"
                  description="Helps Enchanted make sense of your content"
                  completed={hasModelsDownloaded.embeddings || downloadState.embeddings.completed}
                  downloading={downloadState.embeddings.downloading}
                  percentage={downloadState.embeddings.percentage}
                  totalBytes={downloadState.embeddings.totalBytes}
                  downloadedBytes={downloadState.embeddings.downloadedBytes}
                  error={downloadState.embeddings.error}
                  onRetry={() => retryModel('embeddings')}
                />

                <div className="h-px bg-white/35"></div>

                <ModelDownloadItem
                  name="Anonymizer model"
                  description="Helps you keep your things private"
                  completed={hasModelsDownloaded.anonymizer || downloadState.anonymizer.completed}
                  downloading={downloadState.anonymizer.downloading}
                  percentage={downloadState.anonymizer.percentage}
                  totalBytes={downloadState.anonymizer.totalBytes}
                  downloadedBytes={downloadState.anonymizer.downloadedBytes}
                  error={downloadState.anonymizer.error}
                  onRetry={() => retryModel('anonymizer')}
                />

                <div className="h-px bg-white/35"></div>

                <ModelDownloadItem
                  name="Inference engine"
                  description=""
                  completed={hasModelsDownloaded.onnx || downloadState.onnx.completed}
                  downloading={downloadState.onnx.downloading}
                  percentage={downloadState.onnx.percentage}
                  totalBytes={downloadState.onnx.totalBytes}
                  downloadedBytes={downloadState.onnx.downloadedBytes}
                  error={downloadState.onnx.error}
                  onRetry={() => retryModel('onnx')}
                />
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
