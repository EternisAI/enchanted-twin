import { useMutation } from '@apollo/client'
import { SendMessageDocument, Message, Role } from '@renderer/graphql/generated/graphql'

export function useSendMessage(
  chatId: string,
  onMessageSent: (msg: Message) => void,
  onError: (error: Message) => void
) {
  const [sendMessageMutation] = useMutation(SendMessageDocument, {
    update(cache, { data }) {
      if (!data?.sendMessage) return
      cache.modify({
        fields: {
          getChat(existing = {}) {
            return {
              ...existing,
              messages: [...(existing.messages || []), data.sendMessage]
            }
          }
        }
      })
    }
  })

  const sendMessage = async (text: string, deepMemory?: boolean) => {
    const optimisticMessage: Message = {
      id: crypto.randomUUID(),
      text,
      role: Role.User,
      imageUrls: [],
      toolCalls: [],
      toolResults: [],
      createdAt: new Date().toISOString(),
      deepMemory
    }

    onMessageSent(optimisticMessage)

    try {
      await sendMessageMutation({
        variables: {
          chatId,
          text,
          deepMemory
        }
      })
    } catch (error) {
      console.error('Error sending message', error)
      const errorMessage: Message = {
        id: `error-${Date.now()}`,
        text: error instanceof Error ? error.message : 'Error sending message',
        role: Role.Assistant,
        imageUrls: [],
        toolCalls: [],
        toolResults: [],
        createdAt: new Date().toISOString(),
        deepMemory
      }

      onError(errorMessage)
    }
  }

  return { sendMessage }
}
