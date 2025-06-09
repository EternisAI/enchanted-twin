import { motion } from 'framer-motion'
import { useNavigate } from '@tanstack/react-router'
import { useEffect, useRef, useState } from 'react'

import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { useTheme } from '@renderer/lib/theme'

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
          duration: 0.7,
          ease: 'linear'
        }}
      />

      <motion.div
        className="absolute bottom-0 left-0 w-full bg-gray-200"
        initial={{ height: 0, bottom: '50%' }}
        animate={{ height: '50%', bottom: 0 }}
        transition={{
          duration: 0.7,
          ease: 'linear'
          // delay: 0.2
        }}
        onAnimationComplete={() => {
          completeOnboarding()
          navigate({ to: '/' })
        }}
      />
    </motion.div>
  )
}

export function OnboardingVoiceAnimation({
  run = true,
  layerCount = 7,
  maxSize = 800,
  sizeStep = 80,
  getFreqData
}: {
  run?: boolean
  layerCount?: number
  maxSize?: number
  sizeStep?: number
  getFreqData: () => Uint8Array
}) {
  const { theme } = useTheme()
  const [rippleScales, setRippleScales] = useState<number[]>([])
  const [rippleOpacities, setRippleOpacities] = useState<number[]>([])
  const smoothedAmps = useRef<number[]>([])
  const animationFrameRef = useRef<number>(0)

  const sizes = Array.from({ length: layerCount }, (_, i) => maxSize - i * sizeStep)
    .filter((d) => d > 0)
    .reverse()

  const bgColor = theme === 'light' ? 'bg-white/25' : 'bg-white/25'

  useEffect(() => {
    smoothedAmps.current = new Array(layerCount).fill(0)
    setRippleScales(new Array(layerCount).fill(1))
    setRippleOpacities(new Array(layerCount).fill(0.3))
  }, [layerCount])

  useEffect(() => {
    if (!run || !getFreqData) {
      setRippleScales(new Array(layerCount).fill(1))
      setRippleOpacities(new Array(layerCount).fill(0.3))
      return
    }

    const updateAnimation = () => {
      if (!getFreqData) return

      const freqData = getFreqData()
      const newScales: number[] = []
      const newOpacities: number[] = []

      for (let i = 0; i < layerCount; i++) {
        const freqIndex = Math.floor((i / layerCount) * freqData.length)
        const rawAmp = (freqData[freqIndex] || 0) / 255

        const smoothingFactor = 0.15
        smoothedAmps.current[i] =
          smoothedAmps.current[i] * (1 - smoothingFactor) + rawAmp * smoothingFactor

        const baseScale = 1.0
        const maxScaleBoost = 0.8
        const scale = baseScale + smoothedAmps.current[i] * maxScaleBoost
        newScales.push(scale)

        const baseOpacity = 0.1
        const maxOpacityBoost = 0.5
        const opacity = baseOpacity + smoothedAmps.current[i] * maxOpacityBoost
        newOpacities.push(opacity)
      }

      setRippleScales(newScales)
      setRippleOpacities(newOpacities)

      animationFrameRef.current = requestAnimationFrame(updateAnimation)
    }

    animationFrameRef.current = requestAnimationFrame(updateAnimation)

    return () => {
      if (animationFrameRef.current) {
        cancelAnimationFrame(animationFrameRef.current)
      }
    }
  }, [run, getFreqData, layerCount])

  return (
    <div className="absolute top-[535px] left-0 w-screen h-screen flex justify-center items-center overflow-hidden pointer-events-none">
      {sizes.map((size, index) => (
        <div
          key={size}
          className={`absolute ${bgColor} rounded-full transition-all duration-75 ease-out`}
          style={{
            width: size,
            height: size,
            transform: `scale(${rippleScales[index] || 1})`,
            opacity: rippleOpacities[index] || 0.3
          }}
        />
      ))}
    </div>
  )
}
