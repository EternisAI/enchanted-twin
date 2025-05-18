// routes/voice/$chatId.tsx
import { createFileRoute } from '@tanstack/react-router'
import { client } from '@renderer/graphql/lib'
import { GetChatDocument, Chat, Message, Role } from '@renderer/graphql/generated/graphql'
import MessageInput from '@renderer/components/chat/MessageInput'
import { UserMessageBubble } from '@renderer/components/chat/Message'
import MovingBall from '@renderer/components/voice/MovingBall'
import { useSendMessage } from '@renderer/hooks/useChat'
import { useMessageStreamSubscription } from '@renderer/hooks/useMessageStreamSubscription'
import { useMessageSubscription } from '@renderer/hooks/useMessageSubscription'
import { useTTS } from '@renderer/hooks/useTTS'
import { useState } from 'react'

interface VoiceSearchParams {
  initialMessage?: string
}

export const Route = createFileRoute('/voice/$chatId')({
  component: VoiceRouteComponent,
  validateSearch: (search: Record<string, unknown>): VoiceSearchParams => {
    return {
      initialMessage: typeof search.initialMessage === 'string' ? search.initialMessage : undefined
    }
  },
  loader: async ({ params }) => {
    try {
      const { data } = await client.query({
        query: GetChatDocument,
        variables: { id: params.chatId },
        fetchPolicy: 'network-only'
      })

      return {
        data: data.getChat,
        loading: false,
        error: null
      }
    } catch (error: unknown) {
      return {
        data: null,
        loading: false,
        error: error instanceof Error ? error.message : 'An unknown error occurred'
      }
    }
  },
  pendingComponent: () => {
    return (
      <div className="flex flex-col items-center justify-center h-full">
        <div className="flex items-center justify-center gap-2 h-20">
          {[...Array(3)].map((_, i) => (
            <div
              key={i}
              className="h-3 w-3 bg-green-500 rounded-full animate-bounce"
              style={{ animationDelay: `${i * 0.15}s` }}
            />
          ))}
        </div>
      </div>
    )
  },
  pendingMs: 100,
  pendingMinMs: 300
})

function VoiceRouteComponent() {
  const { data, error } = Route.useLoaderData()
  const { initialMessage } = Route.useSearch()
  const { speak } = useTTS()

  const [lastUserMessage, setLastUserMessage] = useState<Message | null>(() => {
    if (!data) return null
    if (initialMessage && data.messages.length === 0) {
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
    const msg = [...data.messages].reverse().find((m) => m.role === Role.User) || null
    return msg
  })

  const [assistantBuffer, setAssistantBuffer] = useState<{ id: string; text: string }>({ id: '', text: '' })

  if (!data) return <div className="p-4">Invalid chat ID.</div>
  if (error) return <div className="p-4 text-red-500">Error loading chat.</div>

  const { sendMessage } = useSendMessage(
    data.id,
    (msg) => {
      setLastUserMessage(msg)
      setAssistantBuffer({ id: '', text: '' })
    },
    () => {}
  )

  useMessageSubscription(data.id, (message) => {
    if (message.role === Role.Assistant) {
      speak(message.text || '')
    }
  })

  useMessageStreamSubscription(data.id, (messageId, chunk, isComplete) => {
    setAssistantBuffer((prev) => {
      const newText = prev.id === messageId ? prev.text + (chunk ?? '') : (chunk ?? '')
      if (isComplete) {
        speak(newText)
        return { id: '', text: '' }
      }
      return { id: messageId, text: newText }
    })
  })

  return (
    <div className="flex flex-col h-full w-full items-center">
      <div className="flex-1 flex items-center justify-center w-full">
        <MovingBall />
      </div>
      <div className="w-full max-w-4xl flex flex-col gap-4 px-4 pb-4">
        {lastUserMessage && <UserMessageBubble message={lastUserMessage} />}
        <MessageInput isWaitingTwinResponse={false} onSend={sendMessage} />
      </div>
    </div>
  )
}
