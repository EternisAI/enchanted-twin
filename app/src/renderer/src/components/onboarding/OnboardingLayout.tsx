import { ReactNode, memo } from 'react'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { ArrowLeft, ArrowRight, Lock } from 'lucide-react'
import { Button } from '../ui/button'
import { useNavigate } from '@tanstack/react-router'
import { Brain } from '../graphics/brain'

interface OnboardingLayoutProps {
  children: ReactNode
  title: string
  subtitle?: string
}

const OnboardingBackground = memo(function OnboardingBackground() {
  return (
    <div className="absolute bottom-0 left-0 w-full z-0 h-[66%]">
      <div className="w-full h-full bg-background/50 bg-gradient-to-b from-background to-background/50 absolute inset-0" />
      <Brain />
    </div>
  )
})

function OnboardingTitle({ title, subtitle }: { title: string; subtitle?: string }) {
  return (
    <div className="flex flex-col gap-1 text-center">
      <h1 className="text-3xl font-bold tracking-tighter">{title}</h1>
      {subtitle && <p className="text-muted-foreground">{subtitle}</p>}
    </div>
  )
}

function OnboardingNavigation() {
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
    navigate({ to: '/' })
  }

  return (
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
  )
}

function OnboardingPrivacyNotice() {
  return (
    <div className="mt-8 text-center">
      <p className="text-sm text-muted-foreground">
        <Lock className="inline-block w-4 h-4 mr-2" /> All your data is stored and processed locally
        on your device
      </p>
    </div>
  )
}

export function OnboardingLayout({ children, title, subtitle }: OnboardingLayoutProps) {
  return (
    <div className="min-h-screen flex flex-col items-center justify-center bg-background p-8">
      <OnboardingBackground />
      <div className="w-full max-w-md flex flex-col gap-8 z-10 relative bg-transparent">
        {/* Content */}
        <div className="flex flex-col gap-4">
          <OnboardingTitle title={title} subtitle={subtitle} />
          {children}
        </div>

        <OnboardingNavigation />
        <OnboardingPrivacyNotice />
      </div>
    </div>
  )
}
