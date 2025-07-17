import { createFileRoute } from '@tanstack/react-router'
import OnboardingContainer from '@renderer/components/onboarding/new/OnboardingContainer'
import { SyncedThemeProvider } from '@renderer/components/SyncedThemeProvider'

export const Route = createFileRoute('/onboarding')({
  component: () => (
    <SyncedThemeProvider>
      <OnboardingContainer />
    </SyncedThemeProvider>
  )
})
