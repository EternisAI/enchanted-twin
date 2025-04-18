import { ReactNode } from 'react'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { ArrowLeft, ArrowRight, Lock } from 'lucide-react'
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
      <div className="w-full max-w-2xl flex flex-col gap-8">
        {/* Content */}
        <div className="flex flex-col gap-4">
          <div className="flex flex-col gap-1">
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
            <Lock className="inline-block w-4 h-4 mr-2" /> All your data is stored and processed
            locally on your device
          </p>
        </div>
      </div>
    </div>
  )
}
