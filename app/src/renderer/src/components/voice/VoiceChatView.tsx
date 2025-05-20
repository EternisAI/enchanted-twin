import { useEffect, useRef, useState } from 'react'
import MessageInput from '@renderer/components/chat/MessageInput'
import { UserMessageBubble } from '@renderer/components/chat/Message'
import { useSendMessage } from '@renderer/hooks/useChat'
import { useMessageSubscription } from '@renderer/hooks/useMessageSubscription'
import { useTTS } from '@renderer/hooks/useTTS'
import { Chat, Message, Role } from '@renderer/graphql/generated/graphql'
import VoiceVisualizer from './VoiceVisualizer' // ⬅️  ① import

interface VoiceChatViewProps {
  chat: Chat
  initialMessage?: string
}

export default function VoiceChatView({ chat, initialMessage }: VoiceChatViewProps) {
  const { isSpeaking, speak, getFreqData } = useTTS()

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

  /* ---------- send message mutation ---------- */
  const { sendMessage: sendMessageRaw } = useSendMessage(chat.id, setLastUserMessage, () => {})

  const sendMessage = (text: string, reasoning: boolean) => {
    sendMessageRaw(text, reasoning, true)
  }

  /* ---------- speech state machine ---------- */
  const [pendingSpeech, setPendingSpeech] = useState(false) // true until isSpeaking flips on
  const triggeredRef = useRef(false)

  useMessageSubscription(chat.id, (message) => {
    if (message.role === Role.Assistant) {
      triggeredRef.current = true
      setPendingSpeech(true)
      speak(message.text ?? '')
    }
  })

  /* when audio actually starts, drop loading state */
  useEffect(() => {
    if (isSpeaking && triggeredRef.current) {
      setPendingSpeech(false)
      triggeredRef.current = false
    }
  }, [isSpeaking])

  const visualState: 0 | 1 | 2 = isSpeaking ? 2 : pendingSpeech ? 1 : 0

  useEffect(() => {
    console.log('visualState', visualState)
  }, [visualState])

  /* ---------- render ---------- */
  if (!chat) return <div className="p-4">Invalid chat ID.</div>

  return (
    <div className="flex flex-col h-full w-full items-center">
      {/* particle visual */}
      <div className="relative flex-1 w-full">
        <VoiceVisualizer
          className="absolute inset-0"
          visualState={visualState}
          getFreqData={getFreqData}
        />
      </div>

      {/* chat footer */}
      <div className="w-full max-w-4xl flex flex-col gap-4 px-4 pb-4">
        {lastUserMessage && <UserMessageBubble message={lastUserMessage} />}
        <MessageInput
          isWaitingTwinResponse={false}
          onSend={sendMessage}
          hasReasoning={false}
          voice={true}
        />
      </div>
    </div>
  )
}
