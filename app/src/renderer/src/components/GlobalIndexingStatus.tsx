import { IndexingState } from '@renderer/graphql/generated/graphql'
import { useIndexingStatus } from '@renderer/hooks/useIndexingStatus'
import { Button } from './ui/button'
import { useNavigate } from '@tanstack/react-router'
import { useTimeRemaining } from '@renderer/hooks/useTimeRemaining'

export function GlobalIndexingStatus() {
  const { data: indexingData } = useIndexingStatus()
  const navigate = useNavigate()

  // Workflow states
  const isIndexing = indexingData?.indexingStatus?.status === IndexingState.IndexingData
  const isProcessing = indexingData?.indexingStatus?.status === IndexingState.ProcessingData
  const isNotStarted = indexingData?.indexingStatus?.status === IndexingState.NotStarted
  const isDownloadingModel = indexingData?.indexingStatus?.status === IndexingState.DownloadingModel

  // Check if there's any active indexing operation
  const hasActiveOperation = isIndexing || isProcessing || isNotStarted || isDownloadingModel

  const handleClick = () => {
    navigate({ to: '/settings/data-sources' })
  }

  const getStatusText = () => {
    if (isIndexing || isProcessing) return 'Processing data...'
    if (isDownloadingModel) return 'Downloading model...'
    if (isNotStarted) return 'Starting import...'
    if (isCalculating) return 'Calculating...'
    return ''
  }

  const getProgress = () => {
    if (!indexingData?.indexingStatus?.dataSources?.length) return 0
    const totalProgress = indexingData.indexingStatus.dataSources.reduce(
      (sum, source) => sum + (source.indexProgress || 0),
      0
    )
    return totalProgress / indexingData.indexingStatus.dataSources.length
  }

  const progressValue = getProgress()
  const { isCalculating } = useTimeRemaining(
    progressValue,
    indexingData?.indexingStatus?.globalStartTime
  )
  // Only show if there's an active indexing operation
  if (!hasActiveOperation) {
    return null
  }
  return (
    <Button
      variant="ghost"
      size="sm"
      className="!bg-transparent h-8 px-3 text-xs font-medium backdrop-blur-md hover:bg-transparent transition-all duration-300"
      onClick={handleClick}
    >
      <div className="flex items-center gap-2">
        <span>{getStatusText()}</span>
        <div className="w-16 bg-secondary rounded-full h-1">
          <div
            className="bg-primary h-1 rounded-full transition-all duration-300"
            style={{
              width: `${progressValue}%`
            }}
          />
        </div>
      </div>
    </Button>
  )
}
