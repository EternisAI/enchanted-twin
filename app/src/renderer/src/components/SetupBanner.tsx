import { useNavigate } from '@tanstack/react-router'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { Button } from './ui/button'
import { Sparkles } from 'lucide-react'

export function SetupBanner() {
  const navigate = useNavigate()
  const { resetOnboarding, lastCompletedStep, totalSteps } = useOnboardingStore()
  const setupProgress = ((lastCompletedStep + 1) / totalSteps) * 100

  // Only hide when we've completed all steps
  if (lastCompletedStep >= totalSteps - 1) return null

  const handleContinueSetup = () => {
    resetOnboarding()
    navigate({ to: '/onboarding' })
  }

  return (
    <div className="bg-card border-b px-4 py-1.5">
      <div className="flex items-center justify-between max-w-screen-2xl mx-auto">
        <div className="flex items-center gap-4">
          <p className="text-sm">
            <span className="font-medium">Setup in progress:</span> {Math.round(setupProgress)}%
            complete
          </p>
          <div className="w-32 h-1 bg-muted rounded-full overflow-hidden">
            <div
              className="h-full bg-primary transition-all duration-300 ease-in-out"
              style={{ width: `${setupProgress}%` }}
            />
          </div>
        </div>
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
