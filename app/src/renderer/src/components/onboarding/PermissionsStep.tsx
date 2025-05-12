import { OnboardingLayout } from './OnboardingLayout'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { Button } from '../ui/button'
import { ArrowRight } from 'lucide-react'
import PermissionsCard from '../settings/permissions/PermissionsCard'

export default function PermissionsStep() {
  const { nextStep, previousStep } = useOnboardingStore()

  return (
    <OnboardingLayout title="Permissions" subtitle="Allow the app to access your data">
      <PermissionsCard />

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
