import { useEffect, useState } from 'react'
import { AnimatePresence, motion } from 'framer-motion'

import { useVoiceStore } from '@renderer/lib/stores/voice'
import { Mic } from 'lucide-react'
import useVoiceAgent from '@renderer/hooks/useVoiceAgent'
import { VoiceModeInput } from '@renderer/components/chat/voice/VoiceModeInput'
import useOnboardingChat from '@renderer/hooks/useOnboardingChat'
import { MessageDisplay, OnboardingBase } from './contexts/OnboardingBase'
import { usePrevious } from '@renderer/lib/hooks/usePrevious'

export default function VoiceOnboarding() {
  const {
    lastMessage,
    lastAgentMessage,
    triggerAnimation,
    shouldFinalizeAfterSpeech,
    createOnboardingChat,
    skipOnboarding,
    finalizeOnboarding
  } = useOnboardingChat()
  const { startVoiceMode, stopVoiceMode } = useVoiceStore()
  const { isAgentSpeaking, isSessionReady } = useVoiceAgent()
  const previousIsAgentSpeaking = usePrevious(isAgentSpeaking)
  const [livekitReady, setLivekitReady] = useState(false)

  console.log('livekitReady', livekitReady, isSessionReady)

  useEffect(() => {
    const waitForLiveKit = async () => {
      let attempts = 0
      const maxAttempts = 30 // 30 seconds timeout

      while (attempts < maxAttempts) {
        try {
          const state = await window.api.livekit.getState()
          if (state?.progress === 100 && state?.status === 'Ready') {
            setLivekitReady(true)
            break
          }
        } catch (error) {
          console.error('Failed to get LiveKit state:', error)
        }

        attempts++
        await new Promise((resolve) => setTimeout(resolve, 1000))
      }
    }

    waitForLiveKit()
  }, [])

  useEffect(() => {
    if (!livekitReady) return

    const initializeVoiceOnboarding = async () => {
      window.api.analytics.capture('onboarding_started', {
        type: 'VOICE'
      })

      const newChatId = await createOnboardingChat()
      startVoiceMode(newChatId, true)
    }
    initializeVoiceOnboarding()
  }, [livekitReady])

  useEffect(() => {
    return () => {
      console.log('VoiceOnboarding unmounting, stopping voice mode')
      stopVoiceMode()
    }
  }, [stopVoiceMode])

  useEffect(() => {
    // Tool finalize_onboarding is called and we wait for the agent to finish speaking last message
    if (shouldFinalizeAfterSpeech && previousIsAgentSpeaking && !isAgentSpeaking) {
      finalizeOnboarding()
    }
  }, [shouldFinalizeAfterSpeech, previousIsAgentSpeaking, isAgentSpeaking, finalizeOnboarding])

  const handleSkip = async () => {
    stopVoiceMode()
    await skipOnboarding()
  }

  return (
    <OnboardingBase
      isAnimationRunning={isAgentSpeaking}
      triggerAnimation={triggerAnimation}
      onSkip={handleSkip}
    >
      <AnimatePresence mode="wait">
        {isSessionReady && livekitReady ? (
          <MessageDisplay lastAgentMessage={lastAgentMessage} lastMessage={lastMessage}>
            <VoiceModeInput />
          </MessageDisplay>
        ) : (
          <motion.div
            key="loading-display"
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.3, ease: 'easeOut' }}
            className="flex flex-col h-full justify-center gap-4 items-center pb-4 w-full"
          >
            <div className="flex flex-col items-center gap-1.5 px-4 py-3">
              <Mic className="w-5 h-5 flex-shrink-0 text-white" />
              <span className="text-lg font-medium text-white">
                {!livekitReady ? 'Setting up voice system...' : 'Starting voice conversation'}
              </span>
              <div className="w-32 h-1 bg-neutral-200 dark:bg-neutral-700 rounded-full overflow-hidden">
                <motion.div
                  className="h-full bg-neutral-500 dark:bg-neutral-400"
                  initial={{ width: '0%' }}
                  animate={{ width: '100%' }}
                  transition={{
                    duration: 5,
                    ease: 'linear',
                    repeat: Infinity,
                    repeatType: 'loop'
                  }}
                />
              </div>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </OnboardingBase>
  )
}
