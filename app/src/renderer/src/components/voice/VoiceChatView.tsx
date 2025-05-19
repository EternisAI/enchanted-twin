import MessageInput from '@renderer/components/chat/MessageInput'
import { UserMessageBubble } from '@renderer/components/chat/Message'
import { useSendMessage } from '@renderer/hooks/useChat'
import { useMessageSubscription } from '@renderer/hooks/useMessageSubscription'
import { useTTS } from '@renderer/hooks/useTTS'
import { useState } from 'react'
import { Chat, Message, Role } from '@renderer/graphql/generated/graphql'
import AudioPulse from './AudioPulse'

interface VoiceChatViewProps {
  chat: Chat
  initialMessage?: string
}

export default function VoiceChatView({ chat, initialMessage }: VoiceChatViewProps) {
  const { isSpeaking, speak } = useTTS()

  console.log('data', chat, initialMessage, isSpeaking)

  const [lastUserMessage, setLastUserMessage] = useState<Message | null>(() => {
    if (!chat) return null
    if (initialMessage && chat.messages.length === 0) {
      const msg: Message = {
        id: `temp-${Date.now()}`,
        text: initialMessage,
        imageUrls: [],
        role: Role.User,
        toolCalls: [],
        toolResults: [],
        createdAt: new Date().toISOString()
      }
      return msg
    }
    const msg = [...chat.messages].reverse().find((m) => m.role === Role.User) || null
    return msg
  })

  // const [assistantBuffer, setAssistantBuffer] = useState<{ id: string; text: string }>({
  //   id: '',
  //   text: ''
  // })

  const { sendMessage } = useSendMessage(
    chat.id,
    (msg) => {
      setLastUserMessage(msg)
    },
    () => {}
  )

  useMessageSubscription(chat.id, (message) => {
    if (message.role === Role.Assistant) {
      speak(message.text || '')
    }
  })

  // useMessageStreamSubscription(data?.id ?? '', (messageId, chunk, isComplete) => {
  //   // setAssistantBuffer((prev) => {
  //   //   const newText = prev.id === messageId ? prev.text + (chunk ?? '') : (chunk ?? '')
  //   //   if (isComplete) {
  //   //     speak(newText)
  //   //     return { id: '', text: '' }
  //   //   }
  //   //   return { id: messageId, text: newText }
  //   // })
  // })

  if (!chat) return <div className="p-4">Invalid chat ID.</div>
  // if (error) return <div className="p-4 text-red-500">Error loading chat.</div>

  return (
    <div className="flex flex-col h-full w-full items-center">
      <div className="flex-1 flex items-center justify-center w-full">
        <AudioPulse speaking={isSpeaking} />
      </div>
      <div className="w-full max-w-4xl flex flex-col gap-4 px-4 pb-4">
        {lastUserMessage && <UserMessageBubble message={lastUserMessage} />}
        <MessageInput isWaitingTwinResponse={false} onSend={sendMessage} hasReasoning={false} />
      </div>
    </div>
  )
}
