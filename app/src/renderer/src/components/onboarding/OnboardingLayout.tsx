import { ReactNode, memo } from 'react'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { Lock } from 'lucide-react'
import { Brain } from '../graphics/brain'
import { motion } from 'framer-motion'
import { OnboardingStep } from '@renderer/lib/stores/onboarding'
import { cn } from '@renderer/lib/utils'

interface OnboardingLayoutProps {
  children: ReactNode
  title: string
  subtitle?: string
  className?: string
}

const OnboardingBackground = memo(function OnboardingBackground() {
  return (
    <div className="absolute bottom-0 right-0 w-full z-0 h-full opacity-35 dark:opacity-100">
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
      <h1 className="text-5xl tracking-normal">{title}</h1>
      {subtitle && <p className="text-muted-foreground text-balance">{subtitle}</p>}
    </div>
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

export function OnboardingLayout({ children, title, subtitle, className }: OnboardingLayoutProps) {
  const { currentStep } = useOnboardingStore()

  return (
    <div className="min-h-screen flex flex-col items-center justify-center bg-background p-8">
      <OnboardingBackground />
      <div
        className={cn(
          'w-full max-w-xl flex flex-col gap-12 z-10 relative bg-transparent',
          className
        )}
      >
        <div className="flex flex-col gap-8">
          <OnboardingTitle title={title} subtitle={subtitle} />
          {children}
        </div>

        {currentStep !== OnboardingStep.Welcome && <OnboardingPrivacyNotice />}
      </div>
    </div>
  )
}
