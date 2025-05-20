import { useEffect, useRef, useState } from 'react'

import { Chat, Message, Role } from '@renderer/graphql/generated/graphql'
import MessageInput from './MessageInput'
import { Switch } from '../ui/switch'
import VoiceVisualizer from '../voice/VoiceVisualizer'
import { useSendMessage } from '@renderer/hooks/useChat'
import { useMessageSubscription } from '@renderer/hooks/useMessageSubscription'
import { useTTS } from '@renderer/hooks/useTTS'
import { UserMessageBubble } from './Message'
import { motion } from 'framer-motion'

interface VoiceModeChatViewProps {
  chat: Chat
  initialMessage?: string
  toggleVoiceMode: () => void
}

export default function VoiceModeChatView({
  chat,
  initialMessage,
  toggleVoiceMode
}: VoiceModeChatViewProps) {
  const { isSpeaking, speak, getFreqData, isLoading } = useTTS()

  /* ---------- last user message (for UI) ---------- */
  const [lastUserMessage, setLastUserMessage] = useState<Message | null>(() => {
    if (!chat) return null
    if (initialMessage && chat.messages.length === 0) {
      return {
        id: `temp-${Date.now()}`,
        text: initialMessage,
        imageUrls: [],
        role: Role.User,
        toolCalls: [],
        toolResults: [],
        createdAt: new Date().toISOString()
      }
    }
    return [...chat.messages].reverse().find((m) => m.role === Role.User) ?? null
  })

  const { sendMessage: sendMessageRaw } = useSendMessage(chat.id, setLastUserMessage, () => {})

  const sendMessage = (text: string, reasoning: boolean) => {
    sendMessageRaw(text, reasoning, true)
  }

  useEffect(() => {
    console.log({ isSpeaking, isLoading })
  }, [isSpeaking, isLoading])

  useEffect(() => {
    console.log('isLoading', isLoading)
  }, [isLoading])

  const triggeredRef = useRef(false)

  useMessageSubscription(chat.id, (message) => {
    if (message.role === Role.Assistant) {
      triggeredRef.current = true
      speak(message.text ?? '')
    }
  })

  /* when audio actually starts, drop loading state */
  useEffect(() => {
    if (isSpeaking && triggeredRef.current) {
      triggeredRef.current = false
    }
  }, [isSpeaking])

  const visualState: 0 | 1 | 2 = isSpeaking ? 2 : isLoading ? 1 : 0

  useEffect(() => {
    console.log('visualState', visualState)
  }, [visualState])

  return (
    <div className="flex flex-col h-full w-full items-center">
      <motion.div
        className="relative flex-1 w-full"
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        transition={{ duration: 1 }}
      >
        <VoiceVisualizer
          className="absolute inset-0"
          visualState={visualState}
          getFreqData={getFreqData}
        />
      </motion.div>

      <div className="w-full max-w-4xl flex flex-col gap-4 px-2 pb-4">
        {lastUserMessage && <UserMessageBubble message={lastUserMessage} />}
        <VoiceModeSwitch voiceMode setVoiceMode={toggleVoiceMode} />
        <MessageInput
          isWaitingTwinResponse={false}
          onSend={sendMessage}
          hasReasoning={false}
          voice
        />
      </div>
    </div>
  )
}

export function VoiceModeSwitch({
  voiceMode,
  setVoiceMode
}: {
  voiceMode: boolean
  setVoiceMode: (voiceMode: boolean) => void
}) {
  return (
    <div className="flex justify-end w-full gap-2">
      <Switch id="voiceMode" checked={voiceMode} onCheckedChange={() => setVoiceMode(!voiceMode)}>
        Voice Mode
      </Switch>
      <label className="text-sm" htmlFor="voiceMode">
        Voice Mode
      </label>
    </div>
  )
}
