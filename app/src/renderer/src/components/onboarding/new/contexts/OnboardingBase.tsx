import { motion } from 'framer-motion'

import { Message } from '@renderer/graphql/generated/graphql'
import { OnboardingVoiceAnimation, OnboardingDoneAnimation } from '../Animations'
import { Button } from '@renderer/components/ui/button'

import { getMockFrequencyData } from '@renderer/lib/utils'
import { UserMessageBubble } from '@renderer/components/chat/messages/Message'

export function OnboardingBase({
  children,
  isAnimationRunning,
  triggerAnimation,
  onSkip
}: {
  children: React.ReactNode
  isAnimationRunning: boolean
  triggerAnimation: boolean
  onSkip: () => void
}) {
  return (
    <div className="w-full h-full flex flex-col justify-between items-center relative">
      <Button
        onClick={onSkip}
        variant="outline"
        size="sm"
        className="absolute bottom-4 right-4 text-white hover:text-black"
      >
        Skip
      </Button>
      {triggerAnimation && <OnboardingDoneAnimation />}
      <motion.div
        className="w-full relative"
        initial={{ opacity: 0, scale: 0.8 }}
        animate={{ opacity: 1, scale: 1 }}
        transition={{ duration: 0.8, ease: 'easeOut', delay: 0.4 }}
      >
        <OnboardingVoiceAnimation run={isAnimationRunning} getFreqData={getMockFrequencyData} />
      </motion.div>
      {children}
    </div>
  )
}

export function MessageDisplay({
  lastAgentMessage,
  lastMessage,
  children
}: {
  lastAgentMessage: Message | null
  lastMessage: Message | null
  children: React.ReactNode
}) {
  return (
    <>
      {lastAgentMessage && (
        <div className="w-full flex flex-col items-center gap-6">
          <motion.p
            key={lastAgentMessage.id}
            className="text-white text-lg text-center max-w-xl break-words"
            initial={{ opacity: 0, y: 30 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.6, ease: 'easeOut' }}
          >
            {lastAgentMessage.text}
          </motion.p>
        </div>
      )}
      <div className="w-xl pb-12 flex flex-col gap-4">
        {lastMessage && (
          <motion.div
            key={lastMessage.id}
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.3, ease: 'easeOut' }}
            className="z-1"
          >
            <UserMessageBubble message={lastMessage} chatPrivacyDict={null} />
          </motion.div>
        )}

        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, ease: 'easeOut', delay: 0.8 }}
          className="z-1 relative"
        >
          {children}
        </motion.div>
      </div>
    </>
  )
}
