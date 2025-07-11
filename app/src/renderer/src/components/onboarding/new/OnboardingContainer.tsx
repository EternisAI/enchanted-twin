import { useEffect, useState } from 'react'
import { useMutation } from '@apollo/client'
import { motion } from 'framer-motion'
import { useNavigate } from '@tanstack/react-router'

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
import { Mic } from 'lucide-react'
import { useMessageSubscription } from '@renderer/hooks/useMessageSubscription'
import { useToolCallUpdate } from '@renderer/hooks/useToolCallUpdate'
import useVoiceAgent from '@renderer/hooks/useVoiceAgent'
import { getMockFrequencyData } from '@renderer/lib/utils'
import { UserMessageBubble } from '@renderer/components/chat/messages/Message'
import { VoiceModeInput } from '@renderer/components/chat/voice/VoiceModeInput'
import useMicrophonePermission from '@renderer/hooks/useMicrophonePermission'
import EnableMicrophone from './EnableMicrophone'
import { auth } from '@renderer/lib/firebase'
import MessageInput from '@renderer/components/chat/MessageInput'
import { useProcessMessageHistoryStream } from '@renderer/hooks/useProcessMessageHistoryStream'

type OnboardingType = 'VOICE' | 'TEXT'

// Custom hook for shared onboarding chat logic
function useOnboardingChat() {
  const navigate = useNavigate()
  const { completeOnboarding } = useOnboardingStore()

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

  const [createChat] = useMutation(CreateChatDocument)
  const [updateProfile] = useMutation(UpdateProfileDocument)
  const [deleteChat] = useMutation(DeleteChatDocument)

  const createOnboardingChat = async () => {
    const chat = await createChat({
      variables: {
        name: 'Onboarding Chat',
        category: ChatCategory.Voice
      }
    })
    const newChatId = chat.data?.createChat.id || ''
    setChatId(newChatId)
    return newChatId
  }

  useMessageSubscription(chatId, (message) => {
    if (message.role === Role.User) {
      setLastMessage(message)
      window.api.analytics.capture('onboarding_message_sent', {
        tools: message.toolCalls.map((tool) => tool.name)
      })
    } else {
      if (message.text !== lastAgentMessage?.text) {
        setLastAgentMessage(message)
      }
    }
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

  const skipOnboarding = () => {
    completeOnboarding()
    navigate({ to: '/' })
  }

  return {
    lastMessage,
    lastAgentMessage,
    chatId,
    triggerAnimation,
    createOnboardingChat,
    skipOnboarding
  }
}

// Base component for shared onboarding UI
function OnboardingBase({
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

// Shared message display component
function MessageDisplay({
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

export default function OnboardingContainer() {
  const navigate = useNavigate()
  const { theme } = useTheme()
  const { isCompleted } = useOnboardingStore()
  const { updateTitlebarColor } = useTitlebarColor()
  const { stopVoiceMode } = useVoiceStore()
  const { microphoneStatus } = useMicrophonePermission()

  const [onboardingType, setOnboardingType] = useState<OnboardingType>('VOICE')

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

  useEffect(() => {
    return () => {
      stopVoiceMode()
    }
  }, [stopVoiceMode])

  const onSkipMicrophoneAccess = () => {
    setOnboardingType('TEXT')
  }

  return (
    <div
      className="w-full h-full flex flex-col justify-center items-center"
      style={{
        background:
          theme === 'light'
            ? 'linear-gradient(180deg, #6068E9 0%, #A5AAF9 100%)'
            : 'linear-gradient(180deg, #18181B 0%, #000 100%)'
      }}
    >
      {microphoneStatus === 'granted' ? (
        <VoiceOnboarding />
      ) : onboardingType === 'TEXT' ? (
        <TTSOnboarding />
      ) : (
        <EnableMicrophone onSkip={onSkipMicrophoneAccess} />
      )}
    </div>
  )
}

function TTSOnboarding() {
  const {
    lastMessage,
    lastAgentMessage,
    chatId,
    triggerAnimation,
    createOnboardingChat,
    skipOnboarding
  } = useOnboardingChat()
  const [isTTSPlaying, setIsTTSPlaying] = useState(false)
  const [messageHistory, setMessageHistory] = useState<Array<{ text: string; role: Role }>>([])
  const [streamingResponse, setStreamingResponse] = useState('')
  const [currentAudio, setCurrentAudio] = useState<HTMLAudioElement | null>(null)

  const handleSendMessage = async (text: string) => {
    console.log('[TTSOnboarding] Sending message:', text)
    const newMessageHistory = [...messageHistory, { text, role: Role.User }]
    setMessageHistory(newMessageHistory)
  }

  useEffect(() => {
    const initializeTTSOnboarding = async () => {
      generateTTSForResponse('Hello there! Welcome to Enchanted, what is your name?')
      await createOnboardingChat()
    }
    initializeTTSOnboarding()
  }, [])

  const handleResponseChunk = (messageId: string, chunk: string, isComplete: boolean) => {
    setStreamingResponse((prev) => prev + chunk)

    if (isComplete) {
      console.log('[TTSOnboarding] Response complete, generating TTS')
      const completeResponse = streamingResponse + chunk
      generateTTSForResponse(completeResponse)
      setStreamingResponse('')
    }
  }

  const stopTTS = () => {
    if (currentAudio) {
      console.log('[TTS] Stopping audio playback')
      currentAudio.pause()
      currentAudio.currentTime = 0
      setIsTTSPlaying(false)
      setCurrentAudio(null)
    }
  }

  const generateTTSForResponse = async (responseText: string) => {
    try {
      const firebaseToken = await auth.currentUser?.getIdToken()

      if (!firebaseToken) {
        console.error('[TTS] No Firebase token available')
        return
      }

      const ttsResult = await window.api.tts.generate(responseText, firebaseToken)

      if (!ttsResult.success) {
        console.error('[TTS] Failed to generate TTS:', ttsResult.error)
        return
      }

      if (!ttsResult.audioBuffer) {
        console.error('[TTS] No audio buffer returned')
        return
      }

      const audioBlob = new Blob([ttsResult.audioBuffer], { type: 'audio/mpeg' })

      if (audioBlob) {
        console.log('[TTS] Successfully generated audio, size:', audioBlob.size)
        const audioUrl = URL.createObjectURL(audioBlob)
        const audio = new Audio(audioUrl)

        setCurrentAudio(audio)

        audio.addEventListener('loadeddata', () => {
          console.log('[TTS] Audio loaded, duration:', audio.duration)
        })

        audio.addEventListener('ended', () => {
          console.log('[TTS] Audio playback finished')
          URL.revokeObjectURL(audioUrl)
          setIsTTSPlaying(false)
          setCurrentAudio(null)
        })

        audio.addEventListener('error', (e) => {
          console.error('[TTS] Audio playback error:', e)
          URL.revokeObjectURL(audioUrl)
          setIsTTSPlaying(false)
          setCurrentAudio(null)
        })

        await audio.play()
        setIsTTSPlaying(true)
        console.log('[TTS] Started audio playback')
      } else {
        console.error('[TTS] Failed to generate audio')
      }
    } catch (error) {
      console.error('[TTS] Error generating TTS:', error)
    }
  }

  useProcessMessageHistoryStream(chatId, messageHistory, true, handleResponseChunk)

  return (
    <OnboardingBase
      isAnimationRunning={isTTSPlaying}
      triggerAnimation={triggerAnimation}
      onSkip={skipOnboarding}
    >
      <MessageDisplay lastAgentMessage={lastAgentMessage} lastMessage={lastMessage}>
        <MessageInput
          onSend={handleSendMessage}
          isWaitingTwinResponse={isTTSPlaying}
          isReasonSelected={false}
          voiceMode
          onStop={stopTTS}
        />
      </MessageDisplay>
    </OnboardingBase>
  )
}

function VoiceOnboarding() {
  const { lastMessage, lastAgentMessage, triggerAnimation, createOnboardingChat, skipOnboarding } =
    useOnboardingChat()
  const { startVoiceMode, stopVoiceMode } = useVoiceStore()
  const { isAgentSpeaking, isSessionReady } = useVoiceAgent()

  useEffect(() => {
    const initializeVoiceOnboarding = async () => {
      const newChatId = await createOnboardingChat()
      startVoiceMode(newChatId, true)
    }
    initializeVoiceOnboarding()
  }, [])

  const handleSkip = () => {
    stopVoiceMode()
    skipOnboarding()
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
