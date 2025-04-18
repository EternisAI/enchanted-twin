import { useCallback, useEffect, useState } from 'react'
import { IndexingState, useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { OnboardingLayout } from './OnboardingLayout'
import { Loader2, RefreshCw } from 'lucide-react'
import { useSubscription, useMutation } from '@apollo/client'
import { gql } from '@apollo/client'
import { toast } from 'sonner'
import { Button } from '../ui/button'
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogTitle
} from '../ui/alert-dialog'

const START_INDEXING = gql`
  mutation StartIndexing {
    startIndexing
  }
`

const INDEXING_STATUS_SUBSCRIPTION = gql`
  subscription IndexingStatus {
    indexingStatus {
      status
      processingDataProgress
      indexingDataProgress
      dataSources {
        id
        name
        isProcessed
        isIndexed
      }
    }
  }
`

export function IndexingStep() {
  const { dataSources, updateDataSource, updateIndexingStatus, completeOnboarding } =
    useOnboardingStore()
  const [isRetrying, setIsRetrying] = useState(false)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)

  const { data, error: subscriptionError } = useSubscription(INDEXING_STATUS_SUBSCRIPTION)
  const [startIndexing, { error: mutationError }] = useMutation(START_INDEXING)

  const handleStartIndexing = useCallback(async () => {
    try {
      setIsRetrying(true)
      setErrorMessage(null)
      await startIndexing()
      toast.success('Indexing process restarted successfully')
    } catch (error) {
      console.error('Failed to start indexing:', error)
      toast.error('Failed to start indexing. Please try again.')
    } finally {
      setIsRetrying(false)
    }
  }, [startIndexing])

  useEffect(() => {
    // Start indexing when component mounts
    handleStartIndexing()
  }, [handleStartIndexing])

  useEffect(() => {
    if (subscriptionError) {
      console.error('Subscription error:', subscriptionError)
      setErrorMessage('Error receiving indexing updates. Please refresh the page.')
      toast.error('Error receiving indexing updates. Please refresh the page.')
    }
  }, [subscriptionError])

  useEffect(() => {
    if (mutationError) {
      console.error('Mutation error:', mutationError)
      setErrorMessage('Failed to start indexing. Please try again.')
      toast.error('Failed to start indexing. Please try again.')
    }
  }, [mutationError])

  useEffect(() => {
    if (data?.indexingStatus) {
      const {
        status,
        processingDataProgress,
        indexingDataProgress,
        dataSources: updatedSources
      } = data.indexingStatus

      // Update indexing status
      updateIndexingStatus({
        status,
        processingDataProgress,
        indexingDataProgress
      })

      // Update data sources
      updatedSources.forEach((source) => {
        updateDataSource(source.id, {
          isProcessed: source.isProcessed,
          isIndexed: source.isIndexed
        })
      })

      // Handle failed state
      if (status === 'FAILED') {
        setErrorMessage('Failed to process data. Please check your data sources and try again.')
      }

      // Check if indexing is completed
      if (status === IndexingState.Completed) {
        completeOnboarding()
      }
    }
  }, [data, updateDataSource, updateIndexingStatus, completeOnboarding])

  const getStatusIcon = (status: IndexingState) => {
    switch (status) {
      case IndexingState.NotStarted:
        return '⏳'
      case IndexingState.DownloadingModel:
      case IndexingState.ProcessingData:
      case IndexingState.IndexingData:
      case IndexingState.CleanUp:
        return <Loader2 className="h-4 w-4 animate-spin" />
      case IndexingState.Completed:
        return '✅'
      default:
        return '⏳'
    }
  }

  const getStatusText = (status: IndexingState) => {
    switch (status) {
      case IndexingState.NotStarted:
        return 'Not Started'
      case IndexingState.DownloadingModel:
        return 'Downloading Model'
      case IndexingState.ProcessingData:
        return 'Processing Data'
      case IndexingState.IndexingData:
        return 'Indexing Data'
      case IndexingState.CleanUp:
        return 'Cleaning Up'
      case IndexingState.Completed:
        return 'Completed'
      default:
        return 'Unknown'
    }
  }

  return (
    <OnboardingLayout
      title="Processing Your Data"
      subtitle="Please wait while we process your data sources. This may take a few minutes."
    >
      <div className="space-y-4">
        {errorMessage && (
          <AlertDialog open={!!errorMessage} onOpenChange={() => setErrorMessage(null)}>
            <AlertDialogContent>
              <AlertDialogTitle>Error</AlertDialogTitle>
              <AlertDialogDescription>{errorMessage}</AlertDialogDescription>
              <AlertDialogFooter>
                <AlertDialogCancel onClick={() => setErrorMessage(null)}>Close</AlertDialogCancel>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        )}

        {/* Overall Progress */}
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-2">
              <span>{getStatusIcon(data?.indexingStatus?.status || IndexingState.NotStarted)}</span>
              <span className="font-medium">Overall Progress</span>
            </div>
            <span className="text-sm text-muted-foreground">
              {getStatusText(data?.indexingStatus?.status || IndexingState.NotStarted)}
            </span>
          </div>
          <div className="h-1 bg-muted rounded-full overflow-hidden">
            <div
              className="h-full bg-primary transition-all duration-300 ease-in-out"
              style={{
                width: `${Math.max(
                  data?.indexingStatus?.processingDataProgress || 0,
                  data?.indexingStatus?.indexingDataProgress || 0
                )}%`
              }}
            />
          </div>
        </div>

        {/* Data Sources */}
        {dataSources.map((source) => (
          <div key={source.id} className="space-y-2">
            <div className="flex items-center justify-between">
              <div className="flex items-center space-x-2">
                <span>{source.isIndexed ? '✅' : source.isProcessed ? '🔄' : '⏳'}</span>
                <span className="font-medium">{source.name}</span>
              </div>
              <span className="text-sm text-muted-foreground">
                {source.isIndexed ? 'Indexed' : source.isProcessed ? 'Processing' : 'Pending'}
              </span>
            </div>
            {!source.isIndexed && (
              <div className="h-1 bg-muted rounded-full overflow-hidden">
                <div
                  className="h-full bg-primary transition-all duration-300 ease-in-out"
                  style={{
                    width: `${source.isProcessed ? 50 : 0}%`
                  }}
                />
              </div>
            )}
          </div>
        ))}

        {(mutationError || subscriptionError || data?.indexingStatus?.status === 'FAILED') && (
          <div className="flex justify-center mt-4">
            <Button
              variant="outline"
              onClick={handleStartIndexing}
              disabled={isRetrying}
              className="flex items-center gap-2"
            >
              {isRetrying ? (
                <>
                  <Loader2 className="h-4 w-4 animate-spin" />
                  Retrying...
                </>
              ) : (
                <>
                  <RefreshCw className="h-4 w-4" />
                  Retry Indexing
                </>
              )}
            </Button>
          </div>
        )}

        <p className="text-sm text-muted-foreground text-center mt-6">
          Your data is being processed locally on your device. This ensures maximum privacy and
          security.
        </p>
      </div>
    </OnboardingLayout>
  )
}
