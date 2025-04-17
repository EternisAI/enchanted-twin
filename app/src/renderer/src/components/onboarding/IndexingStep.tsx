import { useEffect } from 'react'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { OnboardingLayout } from './OnboardingLayout'
import { Loader2 } from 'lucide-react'

export function IndexingStep() {
  const { dataSources, updateDataSource, completeOnboarding } = useOnboardingStore()

  // TODO: Replace with actual GraphQL subscription
  useEffect(() => {
    const interval = setInterval(() => {
      dataSources.forEach((source) => {
        if (source.status === 'pending') {
          updateDataSource(source.type, { status: 'processing', progress: 0 })
        } else if (source.status === 'processing') {
          const currentProgress = source.progress || 0
          if (currentProgress < 100) {
            updateDataSource(source.type, { progress: currentProgress + 10 })
          } else {
            updateDataSource(source.type, { status: 'completed' })
          }
        }
      })

      // Check if all sources are completed
      if (dataSources.every((source) => source.status === 'completed')) {
        completeOnboarding()
        clearInterval(interval)
      }
    }, 1000)

    return () => clearInterval(interval)
  }, [dataSources, updateDataSource, completeOnboarding])

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'pending':
        return '⏳'
      case 'processing':
        return <Loader2 className="h-4 w-4 animate-spin" />
      case 'completed':
        return '✅'
      case 'error':
        return '❌'
      default:
        return '⏳'
    }
  }

  return (
    <OnboardingLayout
      title="Processing Your Data"
      subtitle="Please wait while we process your data sources. This may take a few minutes."
    >
      <div className="space-y-4">
        {dataSources.map((source) => (
          <div key={source.type} className="space-y-2">
            <div className="flex items-center justify-between">
              <div className="flex items-center space-x-2">
                <span>{getStatusIcon(source.status)}</span>
                <span className="font-medium">{source.type}</span>
              </div>
              <span className="text-sm text-muted-foreground">
                {source.status === 'completed'
                  ? 'Completed'
                  : source.progress
                    ? `${source.progress}%`
                    : 'Pending'}
              </span>
            </div>
            {source.status === 'processing' && (
              <div className="h-1 bg-muted rounded-full overflow-hidden">
                <div
                  className="h-full bg-primary transition-all duration-300 ease-in-out"
                  style={{ width: `${source.progress}%` }}
                />
              </div>
            )}
          </div>
        ))}

        <p className="text-sm text-muted-foreground text-center mt-6">
          Your data is being processed locally on your device. This ensures maximum privacy and
          security.
        </p>
      </div>
    </OnboardingLayout>
  )
}
