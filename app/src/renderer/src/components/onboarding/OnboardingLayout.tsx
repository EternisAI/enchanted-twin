import { ReactNode } from 'react'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { ArrowLeft, ArrowRight } from 'lucide-react'
import { Button } from '../ui/button'
import { useNavigate } from '@tanstack/react-router'

interface OnboardingLayoutProps {
  children: ReactNode
  title: string
  subtitle?: string
}

export function OnboardingLayout({ children, title, subtitle }: OnboardingLayoutProps) {
  const {
    currentStep,
    totalSteps,
    nextStep,
    previousStep,
    canGoNext,
    canGoPrevious,
    completeOnboarding
  } = useOnboardingStore()
  const navigate = useNavigate()

  const handleSkip = () => {
    completeOnboarding()
    navigate({ to: '/chat' })
  }

  return (
    <div className="min-h-screen flex flex-col items-center justify-center bg-background p-8">
      <div className="w-full max-w-2xl">
        {/* Progress indicator */}
        <div className="mb-8">
          <div className="flex justify-between mb-2">
            <span className="text-sm text-muted-foreground">
              Step {currentStep + 1} of {totalSteps}
            </span>
            <span className="text-sm text-muted-foreground">
              {Math.round(((currentStep + 1) / totalSteps) * 100)}%
            </span>
          </div>
          <div className="h-1 bg-muted rounded-full overflow-hidden">
            <div
              className="h-full bg-primary transition-all duration-300 ease-in-out"
              style={{ width: `${((currentStep + 1) / totalSteps) * 100}%` }}
            />
          </div>
        </div>

        {/* Content */}
        <div className="space-y-6">
          <div className="space-y-2">
            <h1 className="text-3xl font-bold tracking-tighter">{title}</h1>
            {subtitle && <p className="text-muted-foreground">{subtitle}</p>}
          </div>
          {children}
        </div>

        {/* Navigation */}
        <div className="mt-8 flex justify-between items-center">
          <Button onClick={previousStep} disabled={!canGoPrevious()}>
            <ArrowLeft className="mr-2 h-4 w-4" />
            Back
          </Button>
          <div className="flex gap-2">
            <Button variant="ghost" onClick={handleSkip}>
              Skip setup
            </Button>
            <Button onClick={nextStep} disabled={!canGoNext()}>
              {currentStep === totalSteps - 1 ? 'Finish' : 'Next'}
              {currentStep < totalSteps - 1 && <ArrowRight className="ml-2 h-4 w-4" />}
            </Button>
          </div>
        </div>

        {/* Privacy notice */}
        <div className="mt-8 text-center">
          <p className="text-sm text-muted-foreground">
            🔒 All your data is stored and processed locally on your device
          </p>
        </div>
      </div>
    </div>
  )
}
