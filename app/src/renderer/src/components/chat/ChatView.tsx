import { useRef, useEffect, useState } from 'react'
import MessageList from './MessageList'
import MessageInput from './MessageInput'
import ChatSuggestions from './ChatSuggestions'
import { Chat, Message, Role, ToolCall } from '@renderer/graphql/generated/graphql'
import { useSendMessage } from '@renderer/hooks/useChat'
import { useMessageSubscription } from '@renderer/hooks/useMessageSubscription'
import { useToolCallUpdate } from '@renderer/hooks/useToolCallUpdate'

export default function ChatView({ chat }: { chat: Chat }) {
  const bottomRef = useRef<HTMLDivElement | null>(null)
  const [messages, setMessages] = useState<Message[]>(chat.messages)
  const [isWaitingTwinResponse, setIsWaitingTwinResponse] = useState(false)
  const [showSuggestions, setShowSuggestions] = useState(true)

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
    setShowSuggestions(false)
  })

  useMessageSubscription(chat.id, (msg) => {
    if (msg.role === Role.User) {
      return
    }
    upsertMessage(msg)
    setIsWaitingTwinResponse(false)
    setShowSuggestions(true)
  })

  useToolCallUpdate(chat.id, (toolCall) => {
    updateToolCallInMessage(toolCall as ToolCall & { messageId: string })
  })

  const [mounted, setMounted] = useState(false)

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: mounted ? 'smooth' : 'instant' })
    if (!mounted) {
      setMounted(true)
    }
  }, [messages, mounted])

  const handleSuggestionClick = (suggestion: string) => {
    sendMessage(suggestion)
  }

  return (
    <div className="flex flex-col h-full w-full">
      <div className="flex-1 overflow-y-auto p-6">
        <div className="flex flex-col items-center">
          <div className="flex flex-col max-w-4xl w-full">
            <MessageList messages={messages} isWaitingTwinResponse={isWaitingTwinResponse} />
            <div ref={bottomRef} />
          </div>
        </div>
      </div>
      <div className="flex flex-col px-6 py-4 w-full items-center justify-center">
        <div className="max-w-4xl mx-auto w-full flex justify-center items-center relative">
          <ChatSuggestions
            chatId={chat.id}
            visible={showSuggestions}
            onSuggestionClick={handleSuggestionClick}
            toggleVisibility={() => setShowSuggestions(!showSuggestions)}
          />
        </div>
        <div className="max-w-4xl mx-auto w-full flex justify-center items-center">
          <MessageInput
            isWaitingTwinResponse={isWaitingTwinResponse}
            onSend={sendMessage}
            onStop={() => {
              setIsWaitingTwinResponse(false)
            }}
          />
        </div>
      </div>
    </div>
  )
}
