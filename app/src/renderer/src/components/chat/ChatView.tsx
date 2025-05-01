import { useRef, useEffect, useState } from 'react'
import MessageList from './MessageList'
import MessageInput from './MessageInput'
import ChatSuggestions from './ChatSuggestions'
import { Chat, Message, Role, ToolCall } from '@renderer/graphql/generated/graphql'
import { useSendMessage } from '@renderer/hooks/useChat'
import { useToolCallUpdate } from '@renderer/hooks/useToolCallUpdate'
import { useMessageStreamSubscription } from '@renderer/hooks/useMessageStreamSubscription'

interface ChatViewProps {
  chat: Chat
  initialMessage?: string
}

export default function ChatView({ chat, initialMessage }: ChatViewProps) {
  const bottomRef = useRef<HTMLDivElement | null>(null)
  const [mounted, setMounted] = useState(false)
  const [isWaitingTwinResponse, setIsWaitingTwinResponse] = useState(false)
  const [showSuggestions, setShowSuggestions] = useState(true)
  const [error, setError] = useState<string>('')
  const [messages, setMessages] = useState<Message[]>(() => {
    // Handle first message optimistically
    if (initialMessage && chat.messages.length === 0) {
      setIsWaitingTwinResponse(true)
      return [
        {
          id: `temp-${Date.now()}`,
          text: initialMessage,
          imageUrls: [],
          role: Role.User,
          toolCalls: [],
          toolResults: [],
          createdAt: new Date().toISOString()
        }
      ]
    }
    return chat.messages
  })

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

  const { sendMessage } = useSendMessage(
    chat.id,
    (msg) => {
      upsertMessage(msg)
      setIsWaitingTwinResponse(true)
      setShowSuggestions(false)
      setError('')
    },
    (msg) => {
      console.error('SendMessage error', msg)
      setError(msg.text ?? 'Error sending message')
      setIsWaitingTwinResponse(false)
    }
  )

  useMessageStreamSubscription(chat.id, (messageId, chunk, isComplete, imageUrls) => {
    const existingMessage = messages.find((m) => m.id === messageId)
    if (!existingMessage) {
      upsertMessage({
        id: messageId,
        text: chunk ?? '',
        role: Role.Assistant,
        createdAt: new Date().toISOString(),
        imageUrls: imageUrls ?? [],
        toolCalls: [],
        toolResults: []
      })
    } else {
      const allImageUrls = existingMessage.imageUrls.concat(imageUrls ?? [])
      const updatedMessage = {
        ...existingMessage,
        text: (existingMessage.text ?? '') + (chunk ?? ''),
        imageUrls: allImageUrls
      }
      upsertMessage(updatedMessage)
    }

    // if (isComplete) {
    setIsWaitingTwinResponse(false)
    // }
  })

  useToolCallUpdate(chat.id, (toolCall) => {
    updateToolCallInMessage(toolCall)
  })

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
      <div className="flex-1 overflow-y-auto">
        <div className="flex flex-col items-center p-6">
          <div className="w-full">
            <MessageList messages={messages} isWaitingTwinResponse={isWaitingTwinResponse} />
            {error && (
              <div className="py-2 px-4 rounded-md border border-red-500 bg-red-500/10 text-red-500">
                Error: {error}
              </div>
            )}
            <div ref={bottomRef} />
          </div>
        </div>
      </div>
      <div className="flex flex-col px-6 py-4 pb-0 w-full items-center justify-center">
        <div className="w-full flex justify-center items-center relative">
          <ChatSuggestions
            chatId={chat.id}
            visible={showSuggestions}
            onSuggestionClick={handleSuggestionClick}
            toggleVisibility={() => setShowSuggestions(!showSuggestions)}
          />
        </div>
        <div className="w-full flex justify-center items-center">
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
