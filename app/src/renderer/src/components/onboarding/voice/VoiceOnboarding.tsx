import { useEffect, useState } from 'react'
import { useMutation } from '@apollo/client'
import { motion } from 'framer-motion'
import { useNavigate } from '@tanstack/react-router'

import { UserMessageBubble } from '@renderer/components/chat/Message'
import {
  ChatCategory,
  CreateChatDocument,
  DeleteChatDocument,
  Message,
  Role,
  UpdateProfileDocument
} from '@renderer/graphql/generated/graphql'
import { OnboardingVoiceAnimation, OnboardingDoneAnimation } from './Animations'
import { useTheme } from '@renderer/lib/theme'
import { Button } from '@renderer/components/ui/button'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { useTitlebarColor } from '@renderer/hooks/useTitlebarColor'
import { useVoiceStore } from '@renderer/lib/stores/voice'
import useDependencyStatus from '@renderer/hooks/useDependencyStatus'
import { Mic } from 'lucide-react'
import { useMessageSubscription } from '@renderer/hooks/useMessageSubscription'
import { useToolCallUpdate } from '@renderer/hooks/useToolCallUpdate'
import useVoiceAgent from '@renderer/hooks/useVoiceAgent'

const getMockFrequencyData = (): Uint8Array => {
  const arraySize = 128
  const freqData = new Uint8Array(arraySize)

  const time = Date.now() * 0.001

  for (let i = 0; i < arraySize; i++) {
    const lowFreq = Math.sin(time * 2 + i * 0.1) * 40 + 60
    const midFreq = Math.sin(time * 3 + i * 0.05) * 30 + 80
    const highFreq = Math.sin(time * 1.5 + i * 0.15) * 20 + 40

    let amplitude = 0
    if (i < arraySize * 0.3) {
      amplitude = lowFreq
    } else if (i < arraySize * 0.7) {
      amplitude = midFreq
    } else {
      amplitude = highFreq
    }
    amplitude += (Math.random() - 0.5) * 15
    amplitude *= 0.8 + 0.2 * Math.sin(time * 0.5)
    freqData[i] = Math.max(0, Math.min(255, Math.round(amplitude)))
  }

  return freqData
}

export default function VoiceOnboardingContainer() {
  const navigate = useNavigate()
  const { theme } = useTheme()
  const { isCompleted } = useOnboardingStore()
  const { updateTitlebarColor } = useTitlebarColor()

  useEffect(() => {
    if (isCompleted) {
      console.log('isCompleted and pushing', isCompleted)
      navigate({ to: '/' })
    }
  }, [isCompleted, navigate])

  useEffect(() => {
    updateTitlebarColor('onboarding')

    return () => {
      updateTitlebarColor('app')
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  return (
    <div
      className="w-full h-full"
      style={{
        background:
          theme === 'light'
            ? 'linear-gradient(180deg, #6068E9 0%, #A5AAF9 100%)'
            : 'linear-gradient(180deg, #18181B 0%, #000 100%)'
      }}
    >
      <VoiceOnboarding />
    </div>
  )
}

function VoiceOnboarding() {
  const navigate = useNavigate()
  // const { speak, stop, isSpeaking, getFreqData, speakWithEvents } = useTTS()

  // const [stepIdx, setStepIdx] = useState(0)
  // const [answers, setAnswers] = useState<string[]>([])

  const [lastMessage, setLastMessage] = useState<Message | null>(null)
  const [lastAgentMessage, setLastAgentMessage] = useState<Message | null>({
    id: '1',
    role: Role.Assistant,
    text: 'Hello there! Welcome to Enchanted, what is your name?',
    imageUrls: [],
    toolCalls: [],
    toolResults: [],
    createdAt: new Date().toISOString()
  })
  const [chatId, setChatId] = useState('')
  const [triggerAnimation, setTriggerAnimation] = useState(false)
  const { completeOnboarding } = useOnboardingStore()
  const { startVoiceMode, stopVoiceMode } = useVoiceStore()

  const [createChat] = useMutation(CreateChatDocument)
  const [updateProfile] = useMutation(UpdateProfileDocument)
  const [deleteChat] = useMutation(DeleteChatDocument)
  const { isLiveKitSessionReady } = useDependencyStatus()
  const { isAgentSpeaking } = useVoiceAgent()

  useEffect(() => {
    const initiateVoiceOnboarding = async () => {
      const chat = await createChat({
        variables: {
          name: 'Onboarding Chat',
          category: ChatCategory.Voice
        }
      })
      setChatId(chat.data?.createChat.id || '')
      startVoiceMode(chat.data?.createChat.id || '', true)
    }
    initiateVoiceOnboarding()
  }, [])

  useMessageSubscription(chatId, (message) => {
    console.log('message', message)
    if (message.role === Role.User) {
      setLastMessage(message)
    } else {
      if (message.text !== lastAgentMessage?.text) {
        setLastAgentMessage(message)
      }
    }

    // // Messages on voice mode are sent by python code via livekit
    // if (message.role === Role.User && isVoiceMode) {
    //   upsertMessage(message)
    //   window.api.analytics.capture('voice_message_sent', {
    //     tools: message.toolCalls.map((tool) => tool.name)
    //   })
    // }
  })

  useToolCallUpdate(chatId, (toolCall) => {
    if (toolCall.name === 'finalize_onboarding' && toolCall.isCompleted) {
      console.log('finalize_onboarding', toolCall.result)

      const result = JSON.parse(toolCall.result?.content || '{}')

      updateProfile({
        variables: {
          input: {
            name: result?.name || 'No name',
            bio: result?.context || 'No bio filled'
          }
        }
      })

      setTimeout(() => {
        setTriggerAnimation(true)
        deleteChat({
          variables: {
            chatId: chatId
          }
        })
      }, 14000)
    }
  })

  return (
    <div className="w-full h-full flex flex-col justify-between items-center relative">
      <Button
        onClick={() => {
          completeOnboarding()
          navigate({ to: '/' })
          stopVoiceMode()
        }}
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
        <OnboardingVoiceAnimation run={isAgentSpeaking} getFreqData={getMockFrequencyData} />
      </motion.div>

      {isLiveKitSessionReady ? (
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
                <UserMessageBubble message={lastMessage} />
              </motion.div>
            )}

            <motion.div
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.6, ease: 'easeOut', delay: 0.8 }}
              className="z-1 relative"
            ></motion.div>
          </div>
        </>
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
          {/* <Button onClick={onStop} variant="outline">
            Exit
          </Button> */}
        </motion.div>
      )}
    </div>
  )
}

// Progress is 0 to 1
// function MiddleProgressBar({ progress = 0.2 }: { progress: number }) {
//   const progressPercent = `${progress * 50}%`

//   return (
//     <div className="w-full h-[2px] bg-white/8 relative">
//       <motion.div
//         className="absolute top-0 left-1/2 h-full bg-white/70 origin-left"
//         style={{ transform: 'translateX(-100%)' }}
//         initial={{ width: 0 }}
//         animate={{ width: progressPercent }}
//         transition={{ duration: 0.5, ease: 'easeOut' }}
//       />
//       <motion.div
//         className="absolute top-0 left-1/2 h-full bg-white/70 origin-left"
//         initial={{ width: 0 }}
//         animate={{ width: progressPercent }}
//         transition={{ duration: 0.5, ease: 'easeOut' }}
//       />
//     </div>
//   )
// }
