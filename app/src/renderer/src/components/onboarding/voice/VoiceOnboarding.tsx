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

// type Ask = (answers: string[]) => string

// type Step = {
//   ask: Ask
//   key: string
// }

// const STEPS: Step[] = [
//   {
//     key: 'name',
//     ask: () => 'Hello! What is your name?'
//   },
//   {
//     key: 'intro',
//     ask: (a) =>
//       `Nice to meet you, ${a[0] || 'friend'}. ` +
//       'Tell me one thing that captures who you are—hobby, passion, fun fact... your call!'
//   },
//   {
//     key: 'sport',
//     ask: () => `Great! And what is your favourite sport?`
//   },
//   {
//     key: 'end',
//     ask: (a) => `Awesome. Thank you, ${a[0] || 'friend'}, we're done and ready to go!`
//   }
// ]

export default function VoiceOnboardingContainer() {
  const navigate = useNavigate()
  const { theme } = useTheme()
  const { isCompleted } = useOnboardingStore()
  const { updateTitlebarColor } = useTitlebarColor()

  useEffect(() => {
    if (isCompleted) {
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

  // const currentPrompt = useMemo(() => STEPS[stepIdx].ask(answers), [stepIdx, answers])
  // const progress = (answers.length + 1) / STEPS.length

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
    console.log('toolCall', toolCall)

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
      }, 10000)
    }
  })

  // useEffect(() => {
  //   speak(currentPrompt)
  //   // eslint-disable-next-line react-hooks/exhaustive-deps
  // }, [])

  // const handleSendMessage = async (text: string) => {
  //   const nextAnswers = [...answers, text]
  //   setAnswers(nextAnswers)

  //   const nextIdx = stepIdx + 1
  //   if (nextIdx === STEPS.length - 1) {
  //     const finalPrompt = STEPS[nextIdx].ask(nextAnswers)
  //     speak(finalPrompt)

  //     await updateProfile({
  //       variables: {
  //         input: {
  //           name: nextAnswers[0],
  //           bio: answers.map((a) => a.trim()).join(', ') // @TODO: Improve this structuring it better after we have the final questions
  //         }
  //       }
  //     })
  //     setStepIdx(nextIdx)
  //     setTimeout(() => {
  //       setTriggerAnimation(true)
  //     }, 5000) // Some time to let the user hear the final message as we dont have a way to know when the message is done yet
  //     return
  //   }

  //   const nextPrompt = STEPS[nextIdx].ask(nextAnswers)
  //   const { started } = speakWithEvents(nextPrompt)
  //   await started
  //   setStepIdx(nextIdx)
  // }

  // const lastAnswer: Message | null = useMemo(() => {
  //   if (answers.length === 0) return null

  //   const message: Message = {
  //     id: Date.now().toString(),
  //     role: Role.User,
  //     text: answers[answers.length - 1],
  //     imageUrls: [],
  //     toolCalls: [],
  //     toolResults: [],
  //     createdAt: new Date().toISOString()
  //   }

  //   return message
  // }, [answers])

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
        <OnboardingVoiceAnimation run={false} getFreqData={() => []} />
      </motion.div>
      {/* <div></div> */}

      {isLiveKitSessionReady ? (
        <>
          {lastAgentMessage && (
            <div className="w-full flex flex-col items-center gap-6">
              <motion.p
                key={lastAgentMessage.text}
                className="text-white text-lg text-center max-w-xl break-words"
                initial={{ opacity: 0, y: 30 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.6, ease: 'easeOut' }}
                // transition={{ duration: 0.6, ease: 'easeOut', delay: stepIdx === 0 ? 1.2 : 0.2 }}
              >
                {lastAgentMessage.text}
              </motion.p>

              {/* <motion.div
          className="w-full relative"
          initial={{ opacity: 0, scaleX: 0 }}
          animate={{ opacity: 1, scaleX: 1 }}
          transition={{ duration: 0.6, ease: 'easeOut', delay: 0 }}
        >
          <MiddleProgressBar progress={progress} />
        </motion.div> */}
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
            >
              {/* <MessageInput
            onSend={handleSendMessage}
            isWaitingTwinResponse={isSpeaking}
            isReasonSelected={false}
            voiceMode
            onStop={stop}
          /> */}
            </motion.div>
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
