import App from '@renderer/App'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { createFileRoute, redirect } from '@tanstack/react-router'

export const Route = createFileRoute('/')({
  component: App,
  beforeLoad: () => {
    const onboardingStore = useOnboardingStore.getState()
    if (!onboardingStore.isCompleted) {
      throw redirect({ to: '/onboarding' })
    }
  }
})
