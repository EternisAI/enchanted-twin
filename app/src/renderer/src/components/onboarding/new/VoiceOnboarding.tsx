import { useEffect } from 'react'
import { motion } from 'framer-motion'

import { useVoiceStore } from '@renderer/lib/stores/voice'
import { Mic } from 'lucide-react'
import useVoiceAgent from '@renderer/hooks/useVoiceAgent'
import { VoiceModeInput } from '@renderer/components/chat/voice/VoiceModeInput'
import useOnboardingChat from '@renderer/hooks/useOnboardingChat'
import { MessageDisplay, OnboardingBase } from './contexts/OnboardingBase'

export default function VoiceOnboarding() {
  const { lastMessage, lastAgentMessage, triggerAnimation, createOnboardingChat, skipOnboarding } =
    useOnboardingChat()
  const { startVoiceMode, stopVoiceMode } = useVoiceStore()
  const { isAgentSpeaking, isSessionReady } = useVoiceAgent()

  useEffect(() => {
    const initializeVoiceOnboarding = async () => {
      window.api.analytics.capture('onboarding_started', {
        type: 'VOICE'
      })

      const newChatId = await createOnboardingChat()
      startVoiceMode(newChatId, true)
    }
    initializeVoiceOnboarding()
  }, [])

  useEffect(() => {
    return () => {
      console.log('VoiceOnboarding unmounting, stopping voice mode')
      stopVoiceMode()
    }
  }, [stopVoiceMode])

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
      {isSessionReady ? (
        <MessageDisplay lastAgentMessage={lastAgentMessage} lastMessage={lastMessage}>
          <VoiceModeInput />
        </MessageDisplay>
      ) : (
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.3, ease: 'easeOut' }}
          className="flex flex-col h-full justify-center gap-4 items-center pb-4"
        >
          <div className="flex flex-col items-center gap-1.5 px-4 py-3">
            <Mic className="w-5 h-5 flex-shrink-0 text-white" />
            <span className="text-lg font-medium text-white">Initializing voice session</span>
            <div className="w-32 h-1 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
              <motion.div
                className="h-full bg-gray-500 dark:bg-gray-400"
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
    </OnboardingBase>
  )
}
