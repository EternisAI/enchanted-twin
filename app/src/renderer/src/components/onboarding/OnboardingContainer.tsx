import { memo, useEffect, useRef } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useOnboardingStore, OnboardingStep } from '@renderer/lib/stores/onboarding'
import { WelcomeStep } from './WelcomeStep'
import { ImportDataStep } from './ImportDataStep'
import { IndexingStep } from './IndexingStep'
import { motion, AnimatePresence } from 'framer-motion'
import { Brain } from '../graphics/brain'

const OnboardingBackground = memo(function OnboardingBackground() {
  return (
    <div className="fixed inset-0 top-0 left-0 right-0 bottom-0 z-0 opacity-35 dark:opacity-100">
      <div className="w-full h-full bg-gradient-to-b from-background to-background/50 absolute inset-0 z-20" />
      <div className="w-full h-full relative z-10">
        <Brain />
      </div>
    </div>
  )
})

export function OnboardingContainer() {
  const { currentStep, isCompleted, nextStep } = useOnboardingStore()
  const navigate = useNavigate()
  const prevStepRef = useRef<OnboardingStep | undefined>(undefined)
  const direction =
    prevStepRef.current !== undefined ? (currentStep > prevStepRef.current ? 1 : -1) : 0

  console.log('currentStep', currentStep)
  console.log('prevStep', prevStepRef.current)
  console.log('direction', direction === 1 ? 'right' : direction === -1 ? 'left' : 'none')

  useEffect(() => {
    prevStepRef.current = currentStep
  }, [currentStep])

  useEffect(() => {
    if (isCompleted) {
      navigate({ to: '/chat' })
    }
  }, [isCompleted, navigate])

  const renderStep = () => {
    switch (currentStep) {
      case OnboardingStep.Welcome:
        return <WelcomeStep onContinue={nextStep} />
      case OnboardingStep.DataSources:
        return <ImportDataStep />
      case OnboardingStep.Indexing:
        return <IndexingStep />
      default:
        return <WelcomeStep onContinue={nextStep} />
    }
  }

  return (
    <>
      <AnimatePresence mode="popLayout" initial={false}>
        <motion.div
          key={currentStep}
          initial={{ x: direction * 100, opacity: 0 }}
          animate={{ x: 0, opacity: 1 }}
          exit={{ x: direction * 100, opacity: 0 }}
          transition={{ duration: 0.3, ease: 'easeInOut' }}
        >
          {renderStep()}
        </motion.div>
      </AnimatePresence>
      <OnboardingBackground />
    </>
  )
}
