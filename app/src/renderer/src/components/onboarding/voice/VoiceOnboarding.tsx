import { useEffect, useMemo, useState } from 'react'
import { motion } from 'framer-motion'

import MessageInput from '@renderer/components/chat/MessageInput'
import { UserMessageBubble } from '@renderer/components/chat/Message'
import { CreateChatDocument, Message, Role } from '@renderer/graphql/generated/graphql'
import { useTTS } from '@renderer/hooks/useTTS'
import { useMutation } from '@apollo/client'
import { toast } from 'sonner'

const QUESTIONS = [
  {
    question: 'What is your name?'
  },
  {
    question: 'What is your dog name?'
  },
  {
    question: 'What is your favorite color?'
  },
  {
    question: 'What is your favorite food?'
  },
  {
    question: 'What is your favorite animal?'
  }
]

const HAS_USER_ONBOARDED = false // we should store using electron-store

export default function VoiceOnboardingContainer() {
  const installationStatus = useKokoroInstallationStatus()
  console.log(installationStatus)
  //   const [createChat, { loading: isCreatingChat }] = useMutation(CreateChatDocument, {
  //     onCompleted: (data) => {
  //       toast.success('Chat created successfully')
  //       setChatId(data.createChat.id)
  //     }
  //   })
  //   const [chatId, setChatId] = useState<string | null>(null)

  //   useEffect(() => {
  //     if (HAS_USER_ONBOARDED || chatId) return
  //     createChat({
  //       variables: {
  //         name: 'Onboarding',
  //         voice: true
  //       }
  //     })
  //   }, [createChat, chatId])

  return (
    <div
      className="w-full h-full"
      style={{
        background: 'linear-gradient(180deg, #6068E9 0%, #A5AAF9 100%)'
      }}
    >
      <VoiceOnboarding />
    </div>
  )
}

function VoiceOnboarding() {
  // @TODO: We should check if kokoro is fully setup and ready to use

  const { speak, stop, isSpeaking, isLoading } = useTTS()

  const [currentQuestionIndex, setCurrentQuestionIndex] = useState(0)
  const [answers, setAnswers] = useState<string[]>([])

  const currentQuestion = QUESTIONS[currentQuestionIndex]
  const progress = answers.length / QUESTIONS.length

  useEffect(() => {
    console.log('speaking')
    speak(QUESTIONS[0].question)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // user message
  //   const { sendMessage } = useSendMessage(
  //     chatId,
  //     (msg) => {
  //       setAnswers([...answers, msg.text ?? ''])
  //       setCurrentQuestionIndex(currentQuestionIndex + 1)
  //     },
  //     (msg) => {
  //       console.error('SendMessage error', msg)
  //       toast.error('Error sending message')
  //       //   setError(msg.text ?? 'Error sending message')
  //       //   setIsWaitingTwinResponse(false)
  //     }
  //   )

  //   useMessageSubscription(chatId, (message) => {
  //     if (message.role !== Role.User) {
  //       window.api.analytics.capture('onboarding_message_received', {
  //         question: currentQuestion.question,
  //         answer: message.text ?? ''
  //       })
  //       speak(message.text ?? '')
  //     }
  //   })

  const handleSendMessage = async (text: string) => {
    setAnswers([...answers, text])
    await new Promise((resolve) => setTimeout(resolve, 1000))
    if (currentQuestionIndex === QUESTIONS.length - 1) {
      toast.success('Onboarding completed')
      return
    }
    await speak(QUESTIONS[currentQuestionIndex + 1].question)
    console.log('spoke')
    setCurrentQuestionIndex(currentQuestionIndex + 1)
  }

  const lastAnswer: Message | null = useMemo(() => {
    if (answers.length === 0) return null

    const message: Message = {
      id: Date.now().toString(),
      role: Role.User,
      text: answers[answers.length - 1],
      imageUrls: [],
      toolCalls: [],
      toolResults: [],
      createdAt: new Date().toISOString()
    }

    return message
  }, [answers])

  return (
    <div className="w-full h-full flex flex-col justify-between items-center">
      {/* <Animation /> */}
      <div></div>
      <div className="w-full flex flex-col items-center gap-6">
        <p className="text-white text-lg text-center max-w-xl break-words">
          {currentQuestion.question}
        </p>
        <MiddleProgressBar progress={progress} />
      </div>
      <div className="w-xl pb-12 flex flex-col gap-4">
        {lastAnswer && (
          <motion.div
            key={lastAnswer.id}
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.3, ease: 'easeOut' }}
          >
            <UserMessageBubble message={lastAnswer} />
          </motion.div>
        )}
        <MessageInput
          onSend={handleSendMessage}
          isWaitingTwinResponse={isSpeaking}
          isReasonSelected={false}
          voiceMode
          onStop={stop}
        />
      </div>
    </div>
  )
}

function Animation() {
  return (
    <div className="absolute top-[200px] left-0 w-screen h-screen flex justify-center items-end overflow-hidden">
      <div className="absolute bottom-0 w-[600px] h-[600px] bg-white/10 rounded-full animate-subtle-scale"></div>
      <div className="absolute bottom-0 w-[500px] h-[500px] bg-white/10 rounded-full animate-subtle-scale"></div>
      <div className="absolute bottom-0 w-[400px] h-[400px] bg-white/10 rounded-full animate-subtle-scale"></div>
      <div className="absolute bottom-0 w-[300px] h-[300px] bg-white/10 rounded-full animate-subtle-scale"></div>
      <div className="absolute bottom-0 w-[200px] h-[200px] bg-white/10 rounded-full animate-subtle-scale"></div>
      <div className="absolute bottom-0 w-[100px] h-[100px] bg-white/10 rounded-full animate-subtle-scale"></div>
    </div>
  )
}

interface InstallationStatus {
  dependency: string
  progress: number
  status: string
  error?: string
}

function useKokoroInstallationStatus() {
  const [installationStatus, setInstallationStatus] = useState<InstallationStatus>({
    dependency: 'Kokoro',
    progress: 0,
    status: 'Not started'
  })

  const fetchCurrentState = async () => {
    try {
      const currentState = await window.api.launch.getCurrentState()
      if (currentState) {
        setInstallationStatus(currentState)
      }
    } catch (error) {
      console.error('Failed to fetch current state:', error)
    }
  }

  useEffect(() => {
    fetchCurrentState()

    const removeListener = window.api.launch.onProgress((data) => {
      console.log('Launch progress update received:', data)
      setInstallationStatus(data)
    })

    window.api.launch.notifyReady()

    return () => {
      removeListener()
    }
  }, [])

  return installationStatus
}

// Progress is 0 to 1
function MiddleProgressBar({ progress = 0.2 }: { progress: number }) {
  const progressPercent = `${progress * 50}%`

  return (
    <div className="w-full h-[2px] bg-white/8 relative">
      <motion.div
        className="absolute top-0 left-1/2 h-full bg-white/70 origin-left"
        style={{ transform: 'translateX(-100%)' }}
        initial={{ width: 0 }}
        animate={{ width: progressPercent }}
        transition={{ duration: 0.5, ease: 'easeOut' }}
      />
      <motion.div
        className="absolute top-0 left-1/2 h-full bg-white/70 origin-left"
        initial={{ width: 0 }}
        animate={{ width: progressPercent }}
        transition={{ duration: 0.5, ease: 'easeOut' }}
      />
    </div>
  )
}
