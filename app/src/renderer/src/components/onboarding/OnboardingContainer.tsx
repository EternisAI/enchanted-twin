import { memo, useEffect, useRef } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useOnboardingStore, OnboardingStep } from '@renderer/lib/stores/onboarding'
import { WelcomeStep } from './WelcomeStep'
import { ImportDataStep } from './ImportDataStep'
import { IndexingStep } from './IndexingStep'
import MCPServersStep from './MCPServersStep'
import { motion, AnimatePresence } from 'framer-motion'
import { Brain } from '../graphics/brain'

const OnboardingBackground = memo(function OnboardingBackground() {
  return (
    <div className="fixed inset-0 top-0 left-0 right-0 bottom-0 z-0 opacity-35 dark:opacity-100">
      <div className="w-full h-full bg-gradient-to-b from-background to-background/50 absolute inset-0 z-20" />
      <div className="w-full h-full relative z-0">
        <Brain />
      </div>
    </div>
  )
})

export function OnboardingContainer() {
  const { currentStep, isCompleted, nextStep } = useOnboardingStore()
  const navigate = useNavigate()

  const prev = useRef<OnboardingStep | null>(null)
  const direction = prev.current === null ? 0 : currentStep > (prev.current as number) ? 1 : -1

  useEffect(() => {
    prev.current = currentStep
  }, [currentStep])

  useEffect(() => {
    if (isCompleted) navigate({ to: '/chat' })
  }, [isCompleted, navigate])

  const renderStep = () => {
    switch (currentStep) {
      case OnboardingStep.Welcome:
        return <WelcomeStep onContinue={nextStep} />
      case OnboardingStep.DataSources:
        return <ImportDataStep />
      case OnboardingStep.MCPServers:
        return <MCPServersStep />
      case OnboardingStep.Indexing:
        return <IndexingStep />
      default:
        return <WelcomeStep onContinue={nextStep} />
    }
  }

  const variants = {
    enter: (dir: number) => ({
      x: dir > 0 ? 72 : -72,
      opacity: 0,
      scale: 0.97
    }),
    center: { x: 0, opacity: 1, scale: 1 },
    exit: (dir: number) => ({
      x: dir > 0 ? -72 : 72,
      opacity: 0,
      scale: 0.97
    })
  }

  const transition = {
    x: { type: 'tween', ease: [0.25, 0.46, 0.45, 0.94], duration: 0.4 },
    opacity: { duration: 0.3 },
    scale: { duration: 0.4 }
  }

  return (
    <>
      <div className="relative h-full w-full overflow-hidden">
        <AnimatePresence custom={direction} initial={false}>
          <motion.div
            key={currentStep}
            custom={direction}
            variants={variants}
            initial="enter"
            animate="center"
            exit="exit"
            transition={transition}
            className="absolute inset-0 will-change-transform z-10"
          >
            {renderStep()}
          </motion.div>
        </AnimatePresence>
      </div>

      <OnboardingBackground />
    </>
  )
}
