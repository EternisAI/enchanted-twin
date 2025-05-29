import { useEffect, useMemo, useState } from 'react'
import { motion } from 'framer-motion'

import MessageInput from '@renderer/components/chat/MessageInput'
import { UserMessageBubble } from '@renderer/components/chat/Message'
import { Message, Role, UpdateProfileDocument } from '@renderer/graphql/generated/graphql'
import { useTTS } from '@renderer/hooks/useTTS'
import { Animation, OnboardingDoneAnimation } from './Animations'
import { useMutation } from '@apollo/client'
import useKokoroInstallationStatus from '@renderer/hooks/useDepencyStatus'
import { useTheme } from '@renderer/lib/theme'

type Ask = (answers: string[]) => string

type Step = {
  ask: Ask
  key: string
}

const STEPS: Step[] = [
  {
    key: 'name',
    ask: () => 'Hello! What is your name?'
  },
  {
    key: 'intro',
    ask: (a) =>
      `Nice to meet you, ${a[0] || 'friend'}. ` +
      'Tell me one thing that captures who you are—hobby, passion, fun fact... your call!'
  },
  {
    key: 'sport',
    ask: () => `Great! And what is your favourite sport?`
  },
  {
    key: 'end',
    ask: (a) => `Awesome. Thank you, ${a[0] || 'friend'}, we're done and ready to go!`
  }
]

export default function VoiceOnboardingContainer() {
  const { installationStatus } = useKokoroInstallationStatus()
  const { theme } = useTheme()

  const areDependenciesReady =
    installationStatus.status?.toLowerCase() === 'completed' || installationStatus.progress === 100

  console.log('areDependenciesReady', areDependenciesReady)

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
      {areDependenciesReady ? (
        <VoiceOnboarding />
      ) : (
        <div className="flex flex-col justify-center items-center h-full gap-4">
          <div className="w-24 h-24 border-4 border-white border-t-transparent rounded-full animate-spin " />
          <p className="text-white text-lg text-center">Adding dependencies...</p>
          <p className="text-white text-md text-center">
            {installationStatus.status} - {installationStatus.progress}%
          </p>
        </div>
      )}
    </div>
  )
}

function VoiceOnboarding() {
  const { speak, stop, isSpeaking } = useTTS()

  const [stepIdx, setStepIdx] = useState(0)
  const [answers, setAnswers] = useState<string[]>([])
  const [triggerAnimation, setTriggerAnimation] = useState(false)

  const [updateProfile] = useMutation(UpdateProfileDocument)

  const currentPrompt = useMemo(() => STEPS[stepIdx].ask(answers), [stepIdx, answers])

  const progress = (answers.length + 1) / STEPS.length

  useEffect(() => {
    speak(currentPrompt)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const handleSendMessage = async (text: string) => {
    const nextAnswers = [...answers, text]
    setAnswers(nextAnswers)

    const nextIdx = stepIdx + 1
    if (nextIdx === STEPS.length - 1) {
      const finalPrompt = STEPS[nextIdx].ask(nextAnswers)
      speak(finalPrompt)

      await updateProfile({
        variables: {
          input: {
            name: nextAnswers[0],
            bio: answers.map((a) => a.trim()).join(', ') // @TODO: Improve this structuring it better after we have the final questions
          }
        }
      })
      setStepIdx(nextIdx)
      setTimeout(() => {
        setTriggerAnimation(true)
      }, 5000) // Some time to let the user hear the final message as we dont have a way to know when the message is done yet
      return
    }

    const nextPrompt = STEPS[nextIdx].ask(nextAnswers)
    speak(nextPrompt)
    setStepIdx(nextIdx)
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
      {triggerAnimation && <OnboardingDoneAnimation />}

      <motion.div
        className="w-full relative"
        initial={{ opacity: 0, scale: 0.8 }}
        animate={{ opacity: 1, scale: 1 }}
        transition={{ duration: 0.8, ease: 'easeOut', delay: 0.4 }}
      >
        <Animation run={isSpeaking} />
      </motion.div>

      <div></div>

      <div className="w-full flex flex-col items-center gap-6">
        <motion.p
          key={stepIdx}
          className="text-white text-lg text-center max-w-xl break-words"
          initial={{ opacity: 0, y: 30 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, ease: 'easeOut', delay: stepIdx === 0 ? 1.2 : 0.2 }}
        >
          {currentPrompt}
        </motion.p>

        <motion.div
          className="w-full relative"
          initial={{ opacity: 0, scaleX: 0 }}
          animate={{ opacity: 1, scaleX: 1 }}
          transition={{ duration: 0.6, ease: 'easeOut', delay: 0 }}
        >
          <MiddleProgressBar progress={progress} />
        </motion.div>
      </div>

      <div className="w-xl pb-12 flex flex-col gap-4">
        {lastAnswer && (
          <motion.div
            key={lastAnswer.id}
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.3, ease: 'easeOut' }}
            className="z-1"
          >
            <UserMessageBubble message={lastAnswer} />
          </motion.div>
        )}

        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, ease: 'easeOut', delay: 0.8 }}
          className="z-1"
        >
          <MessageInput
            onSend={handleSendMessage}
            isWaitingTwinResponse={isSpeaking}
            isReasonSelected={false}
            voiceMode
            onStop={stop}
          />
        </motion.div>
      </div>
    </div>
  )
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
