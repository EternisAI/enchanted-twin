import { ReactNode, memo } from 'react'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { ArrowLeft, ArrowRight, Lock } from 'lucide-react'
import { Button } from '../ui/button'
import { Brain } from '../graphics/brain'
import { motion } from 'framer-motion'
import { OnboardingStep, IndexingState } from '@renderer/lib/stores/onboarding'

interface OnboardingLayoutProps {
  children: ReactNode
  title: string
  subtitle?: string
}

const OnboardingBackground = memo(function OnboardingBackground() {
  return (
    <div className="absolute bottom-0 right-0 w-full z-0 h-full opacity-50 dark:opacity-100">
      <div className="w-full h-full bg-gradient-to-b from-background to-background/50 absolute inset-0 z-20" />
      <div className="w-full h-full relative z-10">
        <Brain />
      </div>
    </div>
  )
})

function OnboardingTitle({ title, subtitle }: { title: string; subtitle?: string }) {
  return (
    <div className="flex flex-col gap-3 text-center">
      <h1 className="text-4xl tracking-normal">{title}</h1>
      {subtitle && <p className="text-muted-foreground text-balance">{subtitle}</p>}
    </div>
  )
}

function OnboardingNavigation() {
  const { currentStep, totalSteps, nextStep, previousStep, stepValidation, indexingStatus } =
    useOnboardingStore()

  const isIndexing =
    currentStep === OnboardingStep.Indexing &&
    indexingStatus.status !== IndexingState.Completed &&
    indexingStatus.status !== IndexingState.Failed

  return (
    <motion.div
      className="flex justify-between items-center"
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.3, delay: 0.2 }}
    >
      {currentStep > 0 ? (
        <Button
          variant="outline"
          onClick={previousStep}
          disabled={!stepValidation.canGoBack() || isIndexing}
        >
          <ArrowLeft className="mr-2 h-4 w-4" />
          Back
        </Button>
      ) : (
        <div className="w-8" />
      )}
      <div className="flex gap-2">
        <Button onClick={nextStep} disabled={!stepValidation.canProceed() || isIndexing}>
          {currentStep === totalSteps - 1 ? 'Finish' : 'Next'}
          {currentStep < totalSteps - 1 && <ArrowRight className="ml-2 h-4 w-4" />}
        </Button>
      </div>
    </motion.div>
  )
}

function OnboardingPrivacyNotice() {
  return (
    <motion.div
      className="text-center"
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.3, delay: 0.3 }}
    >
      <p className="text-sm text-muted-foreground">
        <Lock className="inline-block w-4 h-4 mr-2" /> All your data is stored and processed locally
        on your device
      </p>
    </motion.div>
  )
}

export function OnboardingLayout({ children, title, subtitle }: OnboardingLayoutProps) {
  return (
    <div className="min-h-screen flex flex-col items-center justify-center bg-background p-8">
      <OnboardingBackground />
      <div className="w-full max-w-md flex flex-col gap-12 z-10 relative bg-transparent">
        <div className="flex flex-col gap-8">
          <OnboardingTitle title={title} subtitle={subtitle} />
          {children}
        </div>

        <OnboardingNavigation />
        <OnboardingPrivacyNotice />
      </div>
    </div>
  )
}
