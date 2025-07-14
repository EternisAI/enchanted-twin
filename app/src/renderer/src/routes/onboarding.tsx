import { createFileRoute } from '@tanstack/react-router'
import OnboardingContainer from '@renderer/components/onboarding/new/OnboardingContainer'

export const Route = createFileRoute('/onboarding')({
  component: OnboardingContainer
})
