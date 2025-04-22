import { useNavigate } from '@tanstack/react-router'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { Button } from './ui/button'
import { Sparkles } from 'lucide-react'

export function SetupBanner() {
  const navigate = useNavigate()
  const { resetOnboarding } = useOnboardingStore()

  const handleContinueSetup = () => {
    resetOnboarding()
    navigate({ to: '/onboarding' })
  }

  return (
    <div className="bg-transparent px-4 py-1.5 fixed top-6 right-0 z-50">
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
  )
}
