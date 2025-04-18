import { useRef, useEffect, useState } from 'react'
import MessageList from './MessageList'
import MessageInput from './MessageInput'
import { Chat, Message, Role, ToolCall } from '@renderer/graphql/generated/graphql'
import { useSendMessage } from '@renderer/hooks/useChat'
import { useMessageSubscription } from '@renderer/hooks/useMessageSubscription'
import { useToolCallUpdate } from '@renderer/hooks/useToolCallUpdate'

const INPUT_HEIGHT = '130px'

export default function ChatView({ chat }: { chat: Chat }) {
  const bottomRef = useRef<HTMLDivElement | null>(null)
  const [messages, setMessages] = useState<Message[]>(chat.messages)
  const [isWaitingTwinResponse, setIsWaitingTwinResponse] = useState(false)

  const upsertMessage = (msg: Message) => {
    setMessages((prev) => {
      const index = prev.findIndex((m) => m.id === msg.id)
      if (index !== -1) {
        const newMessages = [...prev]
        newMessages[index] = msg
        return newMessages
      } else {
        return [...prev, msg]
      }
    })
  }

  const updateToolCallInMessage = (toolCallUpdate: ToolCall & { messageId: string }) => {
    setMessages((prev) => {
      const existingMessageIndex = prev.findIndex((m) => m.id === toolCallUpdate.messageId)

      if (existingMessageIndex !== -1) {
        const updatedMessages = [...prev]
        const msg = updatedMessages[existingMessageIndex]
        const toolCallIndex = msg.toolCalls.findIndex((tc) => tc.id === toolCallUpdate.id)
        const updatedToolCalls = [...msg.toolCalls]

        if (toolCallIndex !== -1) {
          updatedToolCalls[toolCallIndex] = {
            ...updatedToolCalls[toolCallIndex],
            ...toolCallUpdate
          }
        } else {
          updatedToolCalls.push(toolCallUpdate as ToolCall)
        }
        updatedMessages[existingMessageIndex] = { ...msg, toolCalls: updatedToolCalls }
        return updatedMessages
      } else {
        // No message found, create a new one to display the tool call
        const newMessage: Message = {
          id: toolCallUpdate.messageId,
          text: null,
          imageUrls: [],
          role: Role.Assistant,
          toolCalls: [toolCallUpdate],
          toolResults: [],
          createdAt: new Date().toISOString()
        }
        return [...prev, newMessage]
      }
    })
  }

  const { sendMessage } = useSendMessage(chat.id, (msg) => {
    upsertMessage(msg)
    setIsWaitingTwinResponse(true)
  })

  useMessageSubscription(chat.id, (msg) => {
    console.log('message subscription', msg)
    if (msg.role === Role.User) {
      return
    }
    upsertMessage(msg)
    setIsWaitingTwinResponse(false)
  })

  useToolCallUpdate(chat.id, (toolCall) => {
    updateToolCallInMessage(toolCall as ToolCall & { messageId: string })
  })

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  return (
    <div className="flex flex-col flex-1 min-h-full w-full justify-between">
      <div
        className="p-6 flex flex-col items-center overflow-y-auto scrollbar-thin scrollbar-thumb-gray-300 scrollbar-track-transparent"
        style={{ maxHeight: `calc(100vh - ${INPUT_HEIGHT})` }}
      >
        <div
          className="flex flex-col max-w-3xl w-full"
          style={{
            viewTransitionName: 'page-content'
          }}
        >
          <MessageList messages={messages} isWaitingTwinResponse={isWaitingTwinResponse} />
          <div ref={bottomRef} />
        </div>
      </div>
      <div
        className="px-6 py-6 border-t border-gray-200"
        style={{ height: INPUT_HEIGHT } as React.CSSProperties}
      >
        <MessageInput
          isWaitingTwinResponse={isWaitingTwinResponse}
          onSend={sendMessage}
          onStop={() => {
            setIsWaitingTwinResponse(false)
          }}
        />
      </div>
    </div>
  )
}
