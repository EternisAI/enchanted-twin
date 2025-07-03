import { useSubscription } from '@apollo/client'
import { IndexingState, IndexingStatusDocument } from '@renderer/graphql/generated/graphql'
import { Button } from './ui/button'
import { useNavigate } from '@tanstack/react-router'

export function GlobalIndexingStatus() {
  const { data: indexingData } = useSubscription(IndexingStatusDocument)
  const navigate = useNavigate()

  // Workflow states
  const isIndexing = indexingData?.indexingStatus?.status === IndexingState.IndexingData
  const isProcessing = indexingData?.indexingStatus?.status === IndexingState.ProcessingData
  const isNotStarted = indexingData?.indexingStatus?.status === IndexingState.NotStarted
  const isDownloadingModel = indexingData?.indexingStatus?.status === IndexingState.DownloadingModel

  // Check if there's any active indexing operation
  const hasActiveOperation = isIndexing || isProcessing || isNotStarted || isDownloadingModel

  const handleClick = () => {
    navigate({ to: '/settings/import-data' })
  }

  // Only show if there's an active indexing operation
  if (!hasActiveOperation) {
    return null
  }

  const getStatusText = () => {
    if (isIndexing) return 'Indexing...'
    if (isProcessing) return 'Processing...'
    if (isDownloadingModel) return 'Downloading model...'
    if (isNotStarted) return 'Starting...'
    return ''
  }

  const getProgress = () => {
    if (!indexingData?.indexingStatus?.dataSources?.length) return 0

    const totalSources = indexingData.indexingStatus.dataSources.length
    const processedSources = indexingData.indexingStatus.dataSources.filter(
      (source) => source.isProcessed
    ).length
    const indexedSources = indexingData.indexingStatus.dataSources.filter(
      (source) => source.isIndexed
    ).length

    // Calculate overall progress:
    // - 10% for processing
    // - 90% for indexing
    const processingProgress = (processedSources / totalSources) * 10
    const indexingProgress = (indexedSources / totalSources) * 90
    return processingProgress + indexingProgress
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
              width: `${getProgress()}%`
            }}
          />
        </div>
      </div>
    </Button>
  )
}
