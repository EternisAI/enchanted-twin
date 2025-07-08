import { useTheme } from '@renderer/lib/theme'
import { Check, Loader } from 'lucide-react'
import React, { useEffect, useState } from 'react'

interface ModelDownloadItemProps {
  name: string
  description: string
  completed: boolean
  downloading: boolean
  percentage: number
  totalBytes: number
  downloadedBytes: number
}

type DownloadState = Record<
  'embeddings' | 'anonymizer',
  {
    downloading: boolean
    percentage: number
    completed: boolean
    totalBytes: number
    downloadedBytes: number
  }
>

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'

  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))

  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))}${sizes[i]}`
}

export default function ModelDownloadedGate({ children }: { children: React.ReactNode }) {
  const { theme } = useTheme()
  const [hasModelsDownloaded, setHasModelsDownloaded] = useState<{
    embeddings: boolean
    anonymizer: boolean
  }>({ embeddings: false, anonymizer: false })

  const [downloadState, setDownloadState] = useState<DownloadState>({
    embeddings: {
      downloading: false,
      percentage: 0,
      completed: false,
      totalBytes: 0,
      downloadedBytes: 0
    },
    anonymizer: {
      downloading: false,
      percentage: 0,
      completed: false,
      totalBytes: 0,
      downloadedBytes: 0
    }
  })

  useEffect(() => {
    const handleProgress = (data: {
      modelName: string
      pct: number
      totalBytes?: number
      downloadedBytes?: number
    }) => {
      setDownloadState((prev) => ({
        ...prev,
        [data.modelName]: {
          ...prev[data.modelName as keyof typeof prev],
          downloading: data.pct < 100,
          percentage: data.pct,
          completed: data.pct === 100,
          totalBytes: data.totalBytes || 0,
          downloadedBytes: data.downloadedBytes || 0
        }
      }))

      if (data.pct === 100) {
        window.api.models.hasModelsDownloaded().then((hasModelsDownloaded) => {
          setHasModelsDownloaded(hasModelsDownloaded)
        })
      }
    }

    const cleanup = window.api.models.onProgress(handleProgress)

    window.api.models.hasModelsDownloaded().then((hasModelsDownloaded) => {
      setHasModelsDownloaded(hasModelsDownloaded)

      if (!hasModelsDownloaded.embeddings) {
        window.api.models.downloadModels('embeddings')
        setDownloadState((prev) => ({
          ...prev,
          embeddings: {
            downloading: true,
            percentage: 0,
            completed: false,
            totalBytes: 0,
            downloadedBytes: 0
          }
        }))
      }

      if (!hasModelsDownloaded.anonymizer) {
        window.api.models.downloadModels('anonymizer')
        setDownloadState((prev) => ({
          ...prev,
          anonymizer: {
            downloading: true,
            percentage: 0,
            completed: false,
            totalBytes: 0,
            downloadedBytes: 0
          }
        }))
      }
    })

    return cleanup
  }, [])

  if (hasModelsDownloaded.anonymizer && hasModelsDownloaded.embeddings) {
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
            <h1 className="text-lg font-normal">Enchanted is getting ready for you to use.</h1>
            <p className="text-xs">Takes ~10 minutes of download</p>
          </div>

          <div className="flex flex-col gap-4">
            <ModelDownloadItem
              name="Embeddings model"
              description="Helps Enchanted make sense of your content"
              completed={hasModelsDownloaded.embeddings || downloadState.embeddings.completed}
              downloading={downloadState.embeddings.downloading}
              percentage={downloadState.embeddings.percentage}
              totalBytes={downloadState.embeddings.totalBytes}
              downloadedBytes={downloadState.embeddings.downloadedBytes}
            />

            <div className="h-px bg-white/35"></div>

            <ModelDownloadItem
              name="Anonymizer model"
              description="Helps you keep your things private"
              completed={downloadState.anonymizer.completed}
              downloading={downloadState.anonymizer.downloading}
              percentage={downloadState.anonymizer.percentage}
              totalBytes={downloadState.anonymizer.totalBytes}
              downloadedBytes={downloadState.anonymizer.downloadedBytes}
            />
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
  downloadedBytes
}: ModelDownloadItemProps) {
  const remainingBytes = totalBytes - downloadedBytes
  const remainingSize = formatBytes(remainingBytes)
  const totalSize = formatBytes(totalBytes)

  return (
    <div className="flex justify-between pb-4 px-3">
      <div className="flex flex-col">
        <h1 className="text-md font-normal">{name}</h1>
        <p className="text-xs text-white/75">{description}</p>
      </div>

      <div className="flex flex-col justify-center gap-1">
        {completed ? (
          <p className="text-md">
            <Check />
          </p>
        ) : downloading ? (
          <>
            <div className="flex items-center justify-end gap-2">
              <Loader className="animate-spin w-4 h-4" />
              <p className="text-md">{percentage}%</p>
            </div>
            <p className="text-xs text-white/70">
              {remainingSize} left of {totalSize}
            </p>
          </>
        ) : (
          <p className="text-md">Pending</p>
        )}
      </div>
    </div>
  )
}
