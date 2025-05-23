import { motion } from 'framer-motion'
import { useNavigate } from '@tanstack/react-router'

import { useOnboardingStore } from '@renderer/lib/stores/onboarding'

export function OnboardingDoneAnimation() {
  const navigate = useNavigate()
  const { completeOnboarding } = useOnboardingStore()

  return (
    <motion.div
      className="fixed inset-0 z-50 pointer-events-none"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{ duration: 0.1 }}
    >
      <motion.div
        className="absolute top-0 left-0 w-full bg-gray-200"
        initial={{ height: 0, top: '50%' }}
        animate={{ height: '50%', top: 0 }}
        transition={{
          duration: 1.5,
          ease: 'easeInOut'
        }}
      />

      <motion.div
        className="absolute bottom-0 left-0 w-full bg-gray-200"
        initial={{ height: 0, bottom: '50%' }}
        animate={{ height: '50%', bottom: 0 }}
        transition={{
          duration: 1.5,
          ease: 'easeInOut',
          delay: 0.2
        }}
        onAnimationComplete={() => {
          completeOnboarding()
          navigate({ to: '/' })
        }}
      />
    </motion.div>
  )
}

export function Animation({
  run = true,
  layerCount = 10,
  maxSize = 600,
  sizeStep = 40
}: {
  run?: boolean
  layerCount?: number
  maxSize?: number
  sizeStep?: number
}) {
  const sizes = Array.from({ length: layerCount }, (_, i) => maxSize - i * sizeStep).filter(
    (d) => d > 0
  )

  return (
    <div className="absolute top-[270px] left-0 w-screen h-screen flex justify-center items-end overflow-hidden pointer-events-none">
      {sizes.map((size) => (
        <div
          key={size}
          className={`absolute bottom-0 bg-white/10 rounded-full ${
            run ? 'animate-subtle-scale' : ''
          }`}
          style={{ width: size, height: size }}
        />
      ))}
    </div>
  )
}
