import { useEffect } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { WelcomeStep } from './WelcomeStep'
import { ImportDataStep } from './ImportDataStep'
import { IndexingStep } from './IndexingStep'
import { motion, AnimatePresence } from 'framer-motion'
import { usePrevious } from '@renderer/lib/hooks/usePrevious'

export function OnboardingContainer() {
  const { currentStep, isCompleted } = useOnboardingStore()
  const navigate = useNavigate()
  const prevStep = usePrevious(currentStep)
  const direction = prevStep !== undefined ? (currentStep > prevStep ? 1 : -1) : 0

  useEffect(() => {
    if (isCompleted) {
      navigate({ to: '/' })
    }
  }, [isCompleted, navigate])

  const renderStep = () => {
    switch (currentStep) {
      case 0:
        return <WelcomeStep />
      case 1:
        return <ImportDataStep />
      case 2:
        return <IndexingStep />
      default:
        return <WelcomeStep />
    }
  }

  return (
    <AnimatePresence mode="wait" initial={false}>
      <motion.div
        key={currentStep}
        initial={{ x: direction * 100, opacity: 0 }}
        animate={{ x: 0, opacity: 1 }}
        exit={{ x: -direction * 100, opacity: 0 }}
        transition={{ duration: 0.3, ease: 'easeInOut' }}
      >
        {renderStep()}
      </motion.div>
    </AnimatePresence>
  )
}
