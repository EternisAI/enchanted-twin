import { useSubscription } from '@apollo/client'
import { IndexingState, IndexingStatusDocument } from '@renderer/graphql/generated/graphql'
import { Button } from './ui/button'
import { Loader2, RefreshCw } from 'lucide-react'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from './ui/tooltip'
import { useRouter } from '@tanstack/react-router'

export function GlobalIndexingStatus() {
  const { data: indexingData } = useSubscription(IndexingStatusDocument)
  const router = useRouter()

  // Workflow states
  const isIndexing = indexingData?.indexingStatus?.status === IndexingState.IndexingData
  const isProcessing = indexingData?.indexingStatus?.status === IndexingState.ProcessingData
  const isNotStarted = indexingData?.indexingStatus?.status === IndexingState.NotStarted
  const hasUnprocessedSources = indexingData?.indexingStatus?.dataSources?.some(
    (source) => !source.isIndexed || source.hasError
  )

  if (!isIndexing && !isProcessing && !isNotStarted && !hasUnprocessedSources) {
    return null
  }

  const getStatusText = () => {
    if (isIndexing) return 'Indexing...'
    if (isProcessing) return 'Processing...'
    if (isNotStarted) return 'Starting...'
    if (hasUnprocessedSources) return 'Pending sources'
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

  const handleClick = () => {
    router.navigate({ to: '/' })
  }

  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            variant="ghost"
            size="sm"
            className="h-8 px-3 text-xs font-medium backdrop-blur-md hover:bg-transparent transition-all duration-300"
            onClick={handleClick}
          >
            <div className="flex items-center gap-2">
              {isIndexing || isProcessing || isNotStarted ? (
                <Loader2 className="h-3 w-3 animate-spin" />
              ) : (
                <RefreshCw className="h-3 w-3" />
              )}
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
        </TooltipTrigger>
        <TooltipContent>
          <p>Click to view indexing details</p>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}
