import { ReactNode } from 'react'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { Lock, X } from 'lucide-react'
import { motion } from 'framer-motion'
import { OnboardingStep } from '@renderer/lib/stores/onboarding'
import { cn } from '@renderer/lib/utils'
import { Button } from '../ui/button'

interface OnboardingLayoutProps {
  children: ReactNode
  title: string
  subtitle?: string
  className?: string
  onClose?: () => void
}

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

export function OnboardingLayout({
  children,
  title,
  subtitle,
  className,
  onClose
}: OnboardingLayoutProps) {
  const { currentStep } = useOnboardingStore()

  return (
    <div className="min-h-screen flex flex-col items-center justify-center bg-background p-8">
      {onClose && (
        <Button variant="ghost" size="icon" className="absolute top-4 right-4" onClick={onClose}>
          <X className="h-4 w-4" />
        </Button>
      )}
      <div
        className={cn(
          'w-full max-w-3xl flex flex-col gap-12 z-10 relative bg-transparent',
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
