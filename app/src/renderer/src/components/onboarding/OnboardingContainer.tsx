import { useEffect } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { WelcomeStep } from './WelcomeStep'
import { ImportDataStep } from './ImportDataStep'
import { IndexingStep } from './IndexingStep'

export function OnboardingContainer() {
  const { currentStep, isCompleted } = useOnboardingStore()
  const navigate = useNavigate()

  useEffect(() => {
    if (isCompleted) {
      navigate({ to: '/' })
    }
  }, [isCompleted, navigate])

  const renderStep = () => {
    switch (currentStep) {
      case 0:
        return <WelcomeStep />
      case 1:
        return <ImportDataStep />
      case 2:
        return <IndexingStep />
      default:
        return <WelcomeStep />
    }
  }

  return renderStep()
}
