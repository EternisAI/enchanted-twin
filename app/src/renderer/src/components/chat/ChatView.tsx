import { useRef, useEffect, useState } from 'react'
import MessageList from './MessageList'
import MessageInput from './MessageInput'
import { Chat, Message, Role, ToolCall } from '@renderer/graphql/generated/graphql'
import { useSendMessage } from '@renderer/hooks/useChat'
import { useToolCallUpdate } from '@renderer/hooks/useToolCallUpdate'
import { useMessageStreamSubscription } from '@renderer/hooks/useMessageStreamSubscription'
import { useMessageSubscription } from '@renderer/hooks/useMessageSubscription'
import VoiceModeChatView, { VoiceModeSwitch } from './voice/ChatVoiceModeView'
import { useVoiceStore } from '@renderer/lib/stores/voice'

interface ChatViewProps {
  chat: Chat
  initialMessage?: string
}

export default function ChatView({ chat, initialMessage }: ChatViewProps) {
  const bottomRef = useRef<HTMLDivElement | null>(null)
  const { isVoiceMode, toggleVoiceMode } = useVoiceStore()
  const [mounted, setMounted] = useState(false)
  const [isWaitingTwinResponse, setIsWaitingTwinResponse] = useState(false)
  // const [showSuggestions, setShowSuggestions] = useState(false)
  const [isReasonSelected, setIsReasonSelected] = useState(false)
  const [error, setError] = useState<string>('')
  const [activeToolCalls, setActiveToolCalls] = useState<ToolCall[]>([]) // current message
  const [historicToolCalls, setHistoricToolCalls] = useState<ToolCall[]>(() => {
    return chat.messages
      .map((message) => message.toolCalls)
      .flat()
      .reverse()
  })

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

  const handleSendMessage = (message: Message) => {
    upsertMessage(message)
    setIsWaitingTwinResponse(true)
    // setShowSuggestions(false)
    setError('')
    setHistoricToolCalls((prev) => [...activeToolCalls, ...prev])
    setActiveToolCalls([])
    window.api.analytics.capture('message_sent', {
      reasoning: isReasonSelected
    })
  }

  const { sendMessage } = useSendMessage(
    chat.id,
    (msg) => handleSendMessage(msg),
    (msg) => {
      console.error('SendMessage error', msg)
      setError(msg.text ?? 'Error sending message')
      setIsWaitingTwinResponse(false)
    }
  )

  useMessageSubscription(chat.id, (message) => {
    if (message.role !== Role.User) {
      upsertMessage(message)
      window.api.analytics.capture('message_received', {
        tools: message.toolCalls.map((tool) => tool.name)
      })
    }
  })

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

    // Update active tool calls
    setActiveToolCalls((prev) => {
      const existingIndex = prev.findIndex((tc) => tc.id === toolCall.id)
      if (existingIndex !== -1) {
        const updated = [...prev]
        updated[existingIndex] = { ...updated[existingIndex], ...toolCall }
        return updated
      }
      return [...prev, toolCall]
    })
  })

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: mounted ? 'smooth' : 'instant' })
    if (!mounted) {
      setMounted(true)
    }
  }, [messages, mounted, isVoiceMode])

  // const handleSuggestionClick = (suggestion: string) => {
  //   sendMessage(suggestion, false, isVoiceMode)
  // }

  if (isVoiceMode) {
    return (
      <VoiceModeChatView
        chat={chat}
        toggleVoiceMode={toggleVoiceMode}
        messages={messages}
        activeToolCalls={activeToolCalls}
        historicToolCalls={historicToolCalls}
        onSendMessage={sendMessage}
        isWaitingTwinResponse={isWaitingTwinResponse}
        error={error}
      />
    )
  }

  return (
    <div className="flex flex-col h-full w-full items-center">
      <div className="flex flex-1 flex-col w-full overflow-y-auto ">
        <div className="flex w-full justify-center">
          <div className="flex flex-col max-w-4xl items-center p-4 w-full">
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
      </div>

      <div className="flex flex-col w-full items-center justify-center px-2">
        {/* <div className="w-full flex max-w-4xl justify-center items-center relative">
          <ChatSuggestions
            chatId={chat.id}
            visible={showSuggestions}
            onSuggestionClick={handleSuggestionClick}
            toggleVisibility={() => setShowSuggestions(!showSuggestions)}
          />
        </div> */}
        <div className="pb-4 w-full max-w-4xl flex flex-col gap-4 justify-center items-center ">
          <VoiceModeSwitch voiceMode={isVoiceMode} setVoiceMode={() => toggleVoiceMode(false)} />
          <MessageInput
            isWaitingTwinResponse={isWaitingTwinResponse}
            onSend={sendMessage}
            onStop={() => {
              setIsWaitingTwinResponse(false)
            }}
            isReasonSelected={isReasonSelected}
            onReasonToggle={setIsReasonSelected}
          />
        </div>
      </div>
    </div>
  )
}
