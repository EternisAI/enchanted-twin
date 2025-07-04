import { createContext, useContext, useMemo, useCallback, ReactNode, useState } from 'react'
import { Chat, Message, Role, ToolCall } from '@renderer/graphql/generated/graphql'
import { useSendMessage } from '@renderer/hooks/useChat'

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
}

export function ChatProvider({ children, chat, initialMessage }: ChatProviderProps) {
  const [isWaitingTwinResponse, setIsWaitingTwinResponse] = useState(false)
  const [isReasonSelected, setIsReasonSelected] = useState(false)
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
    },
    []
  )

  const updatePrivacyDict = useCallback((privacyDict: string) => {
    setPrivacyDict(privacyDict)
  }, [])

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
      window.api.analytics.capture('message_sent', {
        reasoning: isReasonSelected
      })
    },
    [upsertMessage, activeToolCalls, isReasonSelected]
  )

  const { sendMessage: sendMessageHook } = useSendMessage(chat.id, handleSendMessage, (msg) => {
    console.error('SendMessage error', msg)
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
