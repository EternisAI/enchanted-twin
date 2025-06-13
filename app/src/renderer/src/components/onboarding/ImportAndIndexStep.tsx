import { OnboardingLayout } from './OnboardingLayout'
import { Button } from '../ui/button'
import { DataSourcesPanel } from '../data-sources/DataSourcesPanel'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { ArrowRight } from 'lucide-react'
import { IndexingState, IndexingStatusDocument } from '@renderer/graphql/generated/graphql'
import { useSubscription } from '@apollo/client'
import { useNavigate } from '@tanstack/react-router'

export function ImportAndIndexStep() {
  const { nextStep, previousStep, completeOnboarding } = useOnboardingStore()
  const navigate = useNavigate()

  const handleIndexingComplete = () => {
    nextStep()
  }

  const handleComplete = () => {
    completeOnboarding()
    navigate({ to: '/', replace: true })
  }

  const { data: indexingData } = useSubscription(IndexingStatusDocument)

  const isIndexing =
    indexingData?.indexingStatus?.status === IndexingState.IndexingData ||
    indexingData?.indexingStatus?.status === IndexingState.DownloadingModel ||
    indexingData?.indexingStatus?.status === IndexingState.ProcessingData ||
    indexingData?.indexingStatus?.status === IndexingState.NotStarted

  return (
    <OnboardingLayout
      title="Import Your Data"
      subtitle="Select the data sources you'd like to import. You can always add more later."
    >
      <div className="flex flex-col gap-6">
        <DataSourcesPanel header={false} onIndexingComplete={handleIndexingComplete} />

        <div className="flex justify-between pt-8">
          <Button variant="outline" onClick={previousStep}>
            Back
          </Button>
          {isIndexing ? (
            <Button variant="outline" onClick={handleComplete}>
              Continue in the background
              <ArrowRight className="ml-2 h-4 w-4" />
            </Button>
          ) : (
            <Button onClick={handleComplete}>
              Chat with your twin
              <ArrowRight className="ml-2 h-4 w-4" />
            </Button>
          )}
        </div>
      </div>
    </OnboardingLayout>
  )
}
