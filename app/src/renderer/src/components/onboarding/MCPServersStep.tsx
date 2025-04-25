import { OnboardingLayout } from './OnboardingLayout'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { Button } from '../ui/button'
import { ArrowRight } from 'lucide-react'
import MCPPanel from '../oauth/MCPPanel'

export default function MCPServersStep() {
  const { nextStep, previousStep } = useOnboardingStore()

  return (
    <OnboardingLayout title="MCP Servers" subtitle="Connect with your favorite platforms">
      <div className="flex flex-col w-full gap-8">
        <div className="bg-card p-6 rounded-lg border">
          {/* <p className="text-muted-foreground mb-6">
            Configure your MCP servers to connect with external services like Google, Slack, and
            more. This allows you to integrate your data and enable AI tools to work with these
            platforms.
          </p> */}

          <MCPPanel />
        </div>

        <div className="flex justify-between">
          <Button variant="outline" onClick={previousStep}>
            Back
          </Button>
          <Button onClick={nextStep}>
            Next
            <ArrowRight className="ml-2 h-4 w-4" />
          </Button>
        </div>
      </div>
    </OnboardingLayout>
  )
}
