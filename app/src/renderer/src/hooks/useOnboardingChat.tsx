import { useEffect, useState } from 'react'
import { useMutation } from '@apollo/client'
import { useNavigate } from '@tanstack/react-router'

import {
  ChatCategory,
  CreateChatDocument,
  DeleteChatDocument,
  Message,
  Role,
  UpdateProfileDocument
} from '@renderer/graphql/generated/graphql'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { useMessageSubscription } from '@renderer/hooks/useMessageSubscription'
import { useToolCallUpdate } from '@renderer/hooks/useToolCallUpdate'

export const INITIAL_AGENT_MESSAGE = 'Hello there! Welcome to Enchanted, what is your name?'
export default function useOnboardingChat() {
  const navigate = useNavigate()
  const { completeOnboarding } = useOnboardingStore()

  const [lastMessage, setLastMessage] = useState<Message | null>(null)
  const [lastAgentMessage, setLastAgentMessage] = useState<Message | null>({
    id: '1',
    role: Role.Assistant,
    text: INITIAL_AGENT_MESSAGE,
    imageUrls: [],
    toolCalls: [],
    toolResults: [],
    createdAt: new Date().toISOString()
  })
  const [chatId, setChatId] = useState('')
  const [triggerAnimation, setTriggerAnimation] = useState(false)
  const [assistantMessageStack, setAssistantMessageStack] = useState<Set<string>>(
    new Set([INITIAL_AGENT_MESSAGE])
  )
  const [shouldFinalizeAfterSpeech, setShouldFinalizeAfterSpeech] = useState(false)

  const [createChat] = useMutation(CreateChatDocument)
  const [updateProfile] = useMutation(UpdateProfileDocument)
  const [deleteChat] = useMutation(DeleteChatDocument)

  const createOnboardingChat = async () => {
    const chat = await createChat({
      variables: {
        name: 'Onboarding Chat',
        category: ChatCategory.Voice
      }
    })
    const newChatId = chat.data?.createChat.id || ''
    setChatId(newChatId)
    return newChatId
  }

  const cleanupChat = async () => {
    if (chatId) {
      console.log('Cleaning up onboarding chat:', chatId)
      try {
        await deleteChat({
          variables: {
            chatId: chatId
          }
        })
      } catch (error) {
        console.error('Error deleting onboarding chat:', error)
      }
    }
  }

  useMessageSubscription(chatId, (message) => {
    if (message.role === Role.User) {
      setLastMessage(message)
      window.api.analytics.capture('onboarding_message_sent', {
        tools: message.toolCalls.map((tool) => tool.name)
      })
    } else {
      if (message.text && !assistantMessageStack.has(message.text)) {
        console.log('setting last agent message', message)
        setLastAgentMessage(message)
        setAssistantMessageStack((prev) => new Set([...prev, message.text || '']))
      }
    }
  })

  useToolCallUpdate(chatId, (toolCall) => {
    if (toolCall.name === 'finalize_onboarding' && toolCall.isCompleted) {
      console.log('finalize_onboarding', toolCall.result)

      const result = JSON.parse(toolCall.result?.content || '{}')

      window.api.analytics.capture('onboarding_completed', {
        name: result?.name || 'No name',
        context: result?.context || 'No context'
      })

      updateProfile({
        variables: {
          input: {
            name: result?.name || 'No name',
            bio: result?.context || 'No bio filled'
          }
        }
      })

      setShouldFinalizeAfterSpeech(true)
    }
  })

  const finalizeOnboarding = async () => {
    setTriggerAnimation(true)
    await cleanupChat()
    setTimeout(() => {
      completeOnboarding()
      navigate({ to: '/' })
    }, 1000)
  }

  const skipOnboarding = async () => {
    await cleanupChat()
    completeOnboarding()
    navigate({ to: '/' })
  }

  // Cleanup chat on unmount
  useEffect(() => {
    return () => {
      if (chatId) {
        cleanupChat()
      }
    }
  }, [chatId])

  return {
    lastMessage,
    lastAgentMessage,
    chatId,
    triggerAnimation,
    shouldFinalizeAfterSpeech,
    createOnboardingChat,
    skipOnboarding,
    cleanupChat,
    finalizeOnboarding
  }
}
