import { OnboardingLayout } from './OnboardingLayout'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { Button } from '../ui/button'
import { ArrowRight } from 'lucide-react'
import MCPPanel from '../oauth/MCPPanel'

export default function MCPServersStep() {
  const { nextStep, previousStep } = useOnboardingStore()

  return (
    <OnboardingLayout
      title="Connections"
      subtitle="Connect your accounts to continually update your data"
    >
      <MCPPanel header={false} />

      <div className="flex justify-between">
        <Button variant="outline" onClick={previousStep}>
          Back
        </Button>
        <Button onClick={nextStep}>
          Next
          <ArrowRight className="ml-2 h-4 w-4" />
        </Button>
      </div>
    </OnboardingLayout>
  )
}
