import { useQuery } from '@apollo/client'
import { gql } from '@apollo/client'
import { OnboardingLayout } from './OnboardingLayout'
import { Button } from '../ui/button'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { CheckCircle2, FileText, MessageSquare, Search } from 'lucide-react'

const GET_PROFILE = gql`
  query GetProfile {
    profile {
      connectedDataSources {
        id
        name
        path
        isIndexed
        isProcessed
        hasError
      }
    }
  }
`

// Mock use cases based on data source types
const getUseCases = (dataSourceName: string) => {
  const useCases = {
    Documents: [
      {
        title: 'Search through your documents',
        description: 'Ask questions about any document in your collection',
        icon: Search
      },
      {
        title: 'Summarize documents',
        description: 'Get quick summaries of long documents',
        icon: FileText
      }
    ],
    'Chat History': [
      {
        title: 'Chat with your data',
        description: 'Have conversations about your indexed content',
        icon: MessageSquare
      },
      {
        title: 'Find past conversations',
        description: 'Search through your chat history',
        icon: Search
      }
    ],
    default: [
      {
        title: 'Search your data',
        description: 'Ask questions about your indexed content',
        icon: Search
      },
      {
        title: 'Chat with your data',
        description: 'Have conversations about your indexed content',
        icon: MessageSquare
      }
    ]
  }

  return useCases[dataSourceName as keyof typeof useCases] || useCases.default
}

export function FinishedStep() {
  const { completeOnboarding } = useOnboardingStore()
  const { data } = useQuery(GET_PROFILE)

  const indexedSources =
    data?.profile?.connectedDataSources?.filter((source) => source.isIndexed) || []

  return (
    <OnboardingLayout
      title="Indexing Complete!"
      subtitle="Your data has been successfully indexed and is ready to use."
    >
      <div className="space-y-8">
        <div className="space-y-4">
          <h3 className="text-lg font-medium">Indexed Sources</h3>
          <div className="grid gap-4">
            {indexedSources.map((source) => (
              <div key={source.id} className="flex items-center gap-3 p-4 border rounded-lg">
                <CheckCircle2 className="h-5 w-5 text-green-500" />
                <div>
                  <p className="font-medium">{source.name}</p>
                  <p className="text-sm text-muted-foreground">{source.path}</p>
                </div>
              </div>
            ))}
          </div>
        </div>

        <div className="space-y-4">
          <h3 className="text-lg font-medium">What you can do now</h3>
          <div className="grid gap-4">
            {indexedSources.map((source) => (
              <div key={source.id} className="space-y-3">
                <h4 className="font-medium">{source.name}</h4>
                <div className="grid gap-3">
                  {getUseCases(source.name).map((useCase, index) => (
                    <div key={index} className="flex items-start gap-3 p-4 border rounded-lg">
                      <useCase.icon className="h-5 w-5 text-primary mt-0.5" />
                      <div>
                        <p className="font-medium">{useCase.title}</p>
                        <p className="text-sm text-muted-foreground">{useCase.description}</p>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </div>

        <Button className="w-full" onClick={() => completeOnboarding()}>
          Get Started
        </Button>
      </div>
    </OnboardingLayout>
  )
}
