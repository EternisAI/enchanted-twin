import { useNavigate } from '@tanstack/react-router'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { Button } from './ui/button'
import { Sparkles } from 'lucide-react'

export function ContinueSetupButton() {
  const navigate = useNavigate()
  const { resetOnboarding } = useOnboardingStore()

  const handleContinueSetup = () => {
    resetOnboarding()
    navigate({ to: '/onboarding' })
  }

  return (
    <Button
      onClick={handleContinueSetup}
      size="sm"
      variant="ghost"
      className="flex items-center gap-2 h-7 bg-transparent"
    >
      <Sparkles className="w-3.5 h-3.5" />
      Continue Setup
    </Button>
  )
}
