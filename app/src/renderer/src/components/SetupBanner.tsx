import { useNavigate } from '@tanstack/react-router'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { Button } from './ui/button'
import { Sparkles } from 'lucide-react'

export function SetupBanner() {
  const navigate = useNavigate()
  const { resetOnboarding } = useOnboardingStore()

  // Only hide when we've completed all steps

  const handleContinueSetup = () => {
    resetOnboarding()
    navigate({ to: '/onboarding' })
  }

  return (
    <div className="bg-muted border-b px-4 py-1.5">
      <div className="flex items-center justify-between max-w-screen-2xl mx-auto">
        DEV
        <Button
          onClick={handleContinueSetup}
          size="sm"
          variant="ghost"
          className="flex items-center gap-2 h-7"
        >
          <Sparkles className="w-3.5 h-3.5" />
          Continue Setup
        </Button>
      </div>
    </div>
  )
}
