import { useCallback, useEffect, useState } from 'react'
import { OnboardingLayout } from './OnboardingLayout'
import { CheckCircle, RefreshCw } from 'lucide-react'
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
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'

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
        hasError
      }
    }
  }
`

export function IndexingStep() {
  const { completeOnboarding, previousStep } = useOnboardingStore()
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

  return (
    <OnboardingLayout
      title="Processing data…"
      subtitle="We're processing your data to make it searchable. This may take a while."
    >
      <div className="flex flex-col gap-6">
        <div className="flex flex-col gap-4">
          {data?.indexingStatus?.dataSources?.map((source) => (
            <div key={source.id} className="flex flex-col gap-2">
              <div className="flex items-center justify-between">
                <span className="font-medium">{source.name}</span>
                <span className="text-sm text-muted-foreground">
                  {source.isIndexed ? 'Indexed' : source.isProcessed ? 'Processing' : 'Pending'}
                </span>
              </div>
              <div className="w-full bg-secondary rounded-full h-2">
                <div
                  className="bg-primary h-2 rounded-full transition-all duration-300"
                  style={{
                    width: `${source.isIndexed ? 100 : source.isProcessed ? 50 : 0}%`
                  }}
                />
              </div>
            </div>
          ))}
        </div>

        <div className="flex flex-col gap-2">
          <Button size="lg" onClick={handleStartIndexing} disabled={isRetrying}>
            <RefreshCw className="mr-2 h-4 w-4" />
            Start indexing
          </Button>
        </div>

        {data?.indexingStatus?.status === 'FAILED' && (
          <div className="flex flex-col gap-4">
            <div className="text-destructive text-sm">
              Failed to index your data. Please try again.
            </div>
          </div>
        )}
        <div className="flex flex-row justify-between">
          <Button variant="outline" onClick={previousStep}>
            Back
          </Button>
          {!isRetrying && (
            <Button onClick={completeOnboarding}>
              <CheckCircle className="mr-2 h-4 w-4" />
              Finish
            </Button>
          )}
        </div>
      </div>

      <AlertDialog open={!!errorMessage} onOpenChange={() => setErrorMessage(null)}>
        <AlertDialogContent>
          <AlertDialogTitle>Error</AlertDialogTitle>
          <AlertDialogDescription>{errorMessage}</AlertDialogDescription>
          <AlertDialogFooter>
            <AlertDialogCancel>Close</AlertDialogCancel>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </OnboardingLayout>
  )
}
