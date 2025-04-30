import { OnboardingLayout } from './OnboardingLayout'
import { useMutation, useQuery } from '@apollo/client'
import { gql } from '@apollo/client'
import { Loader2, ArrowRight } from 'lucide-react'
import { Button } from '../ui/button'
import { useState } from 'react'
import { toast } from 'sonner'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { DataSourcesPanel } from '../DataSourcesPanel'
import { GetDataSourcesDocument } from '@renderer/graphql/generated/graphql'

const ADD_DATA_SOURCE = gql`
  mutation AddDataSource($name: String!, $path: String!) {
    addDataSource(name: $name, path: $path)
  }
`

export function ImportDataStep() {
  const { refetch } = useQuery(GetDataSourcesDocument)
  const [addDataSource] = useMutation(ADD_DATA_SOURCE)
  const [pendingDataSources, setPendingDataSources] = useState<
    Record<string, { name: string; path: string }>
  >({})
  const [isProcessing, setIsProcessing] = useState(false)
  const { nextStep, previousStep } = useOnboardingStore()

  const handleNext = async () => {
    if (Object.keys(pendingDataSources).length === 0) {
      nextStep()
      return
    }

    setIsProcessing(true)
    try {
      // Add all pending data sources
      for (const source of Object.values(pendingDataSources)) {
        await addDataSource({
          variables: {
            name: source.name,
            path: source.path
          }
        })
      }

      await refetch()
      toast.success('Data sources added successfully')
      nextStep()
    } catch (error) {
      console.error('Error adding data sources:', error)
      toast.error('Failed to add data sources. Please try again.')
    } finally {
      setIsProcessing(false)
    }
  }

  const handleDataSourceSelected = () => {
    // No-op, handled by DataSourcesPanel
  }

  const handleDataSourceRemoved = (name: string) => {
    setPendingDataSources((prev) => {
      const newState = { ...prev }
      delete newState[name]
      return newState
    })
  }

  return (
    <OnboardingLayout
      title="Import Your Data"
      subtitle="Select the data sources you'd like to import. You can always add more later."
      onClose={nextStep}
    >
      <DataSourcesPanel
        onDataSourceSelected={handleDataSourceSelected}
        onDataSourceRemoved={handleDataSourceRemoved}
      />
      <div className="flex justify-between pt-8">
        <Button variant="outline" onClick={previousStep}>
          Back
        </Button>
        <Button onClick={handleNext} disabled={isProcessing}>
          {isProcessing ? (
            <>
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              Processing...
            </>
          ) : (
            <>
              Next <ArrowRight className="ml-2 h-4 w-4" />
            </>
          )}
        </Button>
      </div>
    </OnboardingLayout>
  )
}
