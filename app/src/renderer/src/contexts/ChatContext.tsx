import {
  createContext,
  useContext,
  useMemo,
  useCallback,
  ReactNode,
  useState,
  useEffect
} from 'react'
import { Chat, Message, Role, ToolCall } from '@renderer/graphql/generated/graphql'
import { useSendMessage } from '@renderer/hooks/useChat'
import { useMessageSubscription } from '@renderer/hooks/useMessageSubscription'
import { useMessageStreamSubscription } from '@renderer/hooks/useMessageStreamSubscription'
import { useToolCallUpdate } from '@renderer/hooks/useToolCallUpdate'
import { usePrivacyDictUpdate } from '@renderer/hooks/usePrivacyDictUpdate'
import { useVoiceStore } from '@renderer/lib/stores/voice'

interface ChatState {
  isWaitingTwinResponse: boolean
  isReasonSelected: boolean
  error: string
  activeToolCalls: ToolCall[]
  historicToolCalls: ToolCall[]
  messages: Message[]
  privacyDict: string
}

interface ChatActions {
  sendMessage: (text: string, reasoning: boolean, voice: boolean) => void
  handleSendMessage: (message: Message) => void
  upsertMessage: (msg: Message) => void
  updateToolCallInMessage: (toolCallUpdate: ToolCall & { messageId: string }) => void
  setIsWaitingTwinResponse: (value: boolean) => void
  setIsReasonSelected: (value: boolean) => void
  setError: (error: string) => void
  setActiveToolCalls: (toolCalls: ToolCall[] | ((prev: ToolCall[]) => ToolCall[])) => void
  setHistoricToolCalls: (toolCalls: ToolCall[] | ((prev: ToolCall[]) => ToolCall[])) => void
  setMessages: (messages: Message[] | ((prev: Message[]) => Message[])) => void
  updatePrivacyDict: (privacyDict: string) => void
}

const ChatStateContext = createContext<ChatState | null>(null)
const ChatActionsContext = createContext<ChatActions | null>(null)

interface ChatProviderProps {
  children: ReactNode
  chat: Chat
  initialMessage?: string
  initialReasoningState?: boolean
}

export function ChatProvider({
  children,
  chat,
  initialMessage,
  initialReasoningState
}: ChatProviderProps) {
  const [isWaitingTwinResponse, setIsWaitingTwinResponse] = useState(false)
  const [isReasonSelected, setIsReasonSelected] = useState(initialReasoningState || false)
  const [error, setError] = useState<string>('')
  const [activeToolCalls, setActiveToolCalls] = useState<ToolCall[]>([]) // current message
  const [privacyDict, setPrivacyDict] = useState<string>(chat.privacyDictJson)
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

  // Ensure messages are updated when chat data changes (fixes cache issues)
  useEffect(() => {
    if (chat.messages && chat.messages.length > 0) {
      // Only update if the messages are actually different to avoid unnecessary re-renders
      setMessages((prevMessages) => {
        // Compare message IDs to see if there are new messages
        const prevIds = prevMessages.map((m) => m.id).sort()
        const newIds = chat.messages.map((m) => m.id).sort()

        if (JSON.stringify(prevIds) !== JSON.stringify(newIds)) {
          console.log('Updating messages due to cache refresh', {
            prevCount: prevMessages.length,
            newCount: chat.messages.length,
            chatId: chat.id
          })
          return chat.messages
        }

        return prevMessages
      })
    }
  }, [chat.messages, chat.id])

  const [lastMessageStartTime, setLastMessageStartTime] = useState<number | null>(null)

  const { isVoiceMode } = useVoiceStore()

  const upsertMessage = useCallback((msg: Message) => {
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
  }, [])

  const updateToolCallInMessage = useCallback(
    (toolCallUpdate: ToolCall & { messageId: string }) => {
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

          const toolCallImages = updatedToolCalls
            .filter((tc) => tc.name === 'generate_image')
            .filter((tc) => tc.isCompleted && tc.result?.imageUrls)
            .flatMap((tc) => tc.result!.imageUrls)

          const allImageUrls = [...new Set([...msg.imageUrls, ...toolCallImages])]

          updatedMessages[existingMessageIndex] = {
            ...msg,
            toolCalls: updatedToolCalls,
            imageUrls: allImageUrls
          }
          return updatedMessages
        } else {
          const toolCallImages =
            toolCallUpdate.isCompleted && toolCallUpdate.result?.imageUrls
              ? toolCallUpdate.result.imageUrls
              : []

          const newMessage: Message = {
            id: toolCallUpdate.messageId,
            text: null,
            imageUrls: toolCallImages,
            role: Role.Assistant,
            toolCalls: [toolCallUpdate],
            toolResults: [],
            createdAt: new Date().toISOString()
          }
          return [...prev, newMessage]
        }
      })
    },
    []
  )

  const updatePrivacyDict = useCallback((privacyDict: string) => {
    setPrivacyDict(privacyDict)
  }, [])

  useMessageSubscription(chat.id, (message) => {
    if (!isVoiceMode) return

    if (message.role !== Role.User) {
      upsertMessage(message)
      window.api.analytics.capture('message_received', {
        tools: message.toolCalls.map((tool) => tool.name)
      })
    }

    if (message.role === Role.User) {
      upsertMessage(message)
      window.api.analytics.capture('voice_message_sent', {
        tools: message.toolCalls.map((tool) => tool.name)
      })
    }
  })

  useMessageStreamSubscription(chat.id, (data) => {
    const { messageId, accumulatedMessage, deanonymizedAccumulatedMessage, imageUrls } = data
    const existingMessage = messages.find((m) => m.id === messageId)

    // Use deanonymized content for display, fallback to accumulated if not available
    const messageText = deanonymizedAccumulatedMessage || accumulatedMessage || ''

    if (!existingMessage) {
      if (lastMessageStartTime) {
        window.api.analytics.capture('message_response_time', {
          duration: Date.now() - lastMessageStartTime
        })

        setLastMessageStartTime(null)
      }

      upsertMessage({
        id: messageId,
        text: messageText,
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
        text: messageText,
        imageUrls: allImageUrls
      }
      upsertMessage(updatedMessage)
    }

    setIsWaitingTwinResponse(false)
  })

  useToolCallUpdate(chat.id, (toolCall) => {
    updateToolCallInMessage(toolCall)

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

  usePrivacyDictUpdate(chat.id, (privacyDict) => {
    updatePrivacyDict(privacyDict)
  })

  const state = useMemo<ChatState>(
    () => ({
      isWaitingTwinResponse,
      isReasonSelected,
      error,
      activeToolCalls,
      historicToolCalls,
      messages,
      privacyDict
    }),
    [
      isWaitingTwinResponse,
      isReasonSelected,
      error,
      activeToolCalls,
      historicToolCalls,
      messages,
      privacyDict
    ]
  )

  const handleSendMessage = useCallback(
    (message: Message) => {
      upsertMessage(message)
      setIsWaitingTwinResponse(true)
      setError('')
      setHistoricToolCalls((prev) => [...activeToolCalls, ...prev])
      setActiveToolCalls([])
      setLastMessageStartTime(Date.now())

      window.api.analytics.capture('message_sent', {
        reasoning: isReasonSelected
      })
    },
    [upsertMessage, activeToolCalls, isReasonSelected]
  )

  const { sendMessage: sendMessageHook } = useSendMessage(chat.id, handleSendMessage, (msg) => {
    console.error('SendMessage error', msg)

    window.api.analytics.capture('message_error_occurred', {
      error_type: 'message_send_failed',
      component: 'ChatContext',
      message: msg.text ?? 'Error sending message'
    })

    setError(msg.text ?? 'Error sending message')
    setIsWaitingTwinResponse(false)
  })

  const actions = useMemo<ChatActions>(
    () => ({
      sendMessage: sendMessageHook,
      handleSendMessage,
      upsertMessage,
      updateToolCallInMessage,
      setIsWaitingTwinResponse,
      setIsReasonSelected,
      setError,
      setActiveToolCalls,
      setHistoricToolCalls,
      setMessages,
      updatePrivacyDict
    }),
    [
      sendMessageHook,
      handleSendMessage,
      upsertMessage,
      updateToolCallInMessage,
      setIsWaitingTwinResponse,
      setIsReasonSelected,
      setError,
      setActiveToolCalls,
      setHistoricToolCalls,
      setMessages,
      updatePrivacyDict
    ]
  )

  return (
    <ChatStateContext.Provider value={state}>
      <ChatActionsContext.Provider value={actions}>{children}</ChatActionsContext.Provider>
    </ChatStateContext.Provider>
  )
}

export function useChatState(): ChatState {
  const context = useContext(ChatStateContext)
  if (!context) {
    throw new Error('useChatState must be used within a ChatProvider')
  }
  return context
}

export function useChatActions(): ChatActions {
  const context = useContext(ChatActionsContext)
  if (!context) {
    throw new Error('useChatActions must be used within a ChatProvider')
  }
  return context
}

export function useChat() {
  return {
    ...useChatState(),
    ...useChatActions()
  }
}
