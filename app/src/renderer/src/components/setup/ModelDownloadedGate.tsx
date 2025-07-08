import { useTheme } from '@renderer/lib/theme'
import { Check, Loader } from 'lucide-react'
import React, { useEffect, useState } from 'react'

interface ModelDownloadItemProps {
  name: string
  description: string
  completed: boolean
  downloading: boolean
  percentage: number
  size: string
}

type DownloadState = Record<
  'embeddings' | 'anonymizer',
  { downloading: boolean; percentage: number; completed: boolean }
>

export default function ModelDownloadedGate({ children }: { children: React.ReactNode }) {
  const { theme } = useTheme()
  const [hasModelsDownloaded, setHasModelsDownloaded] = useState<{
    embeddings: boolean
    anonymizer: boolean
  }>({ embeddings: false, anonymizer: false })

  const [downloadState, setDownloadState] = useState<DownloadState>({
    embeddings: { downloading: false, percentage: 0, completed: false },
    anonymizer: { downloading: false, percentage: 0, completed: false }
  })

  useEffect(() => {
    const handleProgress = (data: { modelName: string; pct: number }) => {
      console.log('Download progress:', data)
      setDownloadState((prev) => ({
        ...prev,
        [data.modelName]: {
          ...prev[data.modelName as keyof typeof prev],
          downloading: data.pct < 100,
          percentage: data.pct,
          completed: data.pct === 100
        }
      }))

      if (data.pct === 100) {
        window.api.models.hasModelsDownloaded().then((hasModelsDownloaded) => {
          console.log('Re-checking hasModelsDownloaded after completion:', hasModelsDownloaded)
          setHasModelsDownloaded(hasModelsDownloaded)
        })
      }
    }

    const cleanup = window.api.models.onProgress(handleProgress)

    window.api.models.hasModelsDownloaded().then((hasModelsDownloaded) => {
      console.log('hasModelsDownloaded', hasModelsDownloaded)
      setHasModelsDownloaded(hasModelsDownloaded)

      if (!hasModelsDownloaded.embeddings) {
        console.log('Starting embeddings download')
        window.api.models.downloadModels('embeddings')
        setDownloadState((prev) => ({
          ...prev,
          embeddings: { downloading: true, percentage: 0, completed: false }
        }))
      }

      if (!hasModelsDownloaded.anonymizer) {
        console.log('Starting anonymizer download')
        window.api.models.downloadModels('anonymizer')
        setDownloadState((prev) => ({
          ...prev,
          anonymizer: { downloading: true, percentage: 0, completed: false }
        }))
      }
    })

    return cleanup
  }, [])

  if (hasModelsDownloaded.anonymizer && hasModelsDownloaded.embeddings) {
    return <>{children}</>
  }

  return (
    <div
      className="w-full h-full flex items-center justify-center"
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
            size="1.2GB"
          />

          <div className="h-px bg-white/35"></div>

          <ModelDownloadItem
            name="Anonymizer model"
            description="Helps you keep your things private"
            completed={downloadState.anonymizer.completed}
            downloading={downloadState.anonymizer.downloading}
            percentage={downloadState.anonymizer.percentage}
            size="1.2GB"
          />
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
  size
}: ModelDownloadItemProps) {
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
            <p className="text-xs text-white/70">{size} left...</p>
          </>
        ) : (
          <p className="text-md">Pending</p>
        )}
      </div>
    </div>
  )
}
