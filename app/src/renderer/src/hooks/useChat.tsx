import { useMutation } from '@apollo/client'
import { SendMessageDocument, Message, Role } from '@renderer/graphql/generated/graphql'

export function useSendMessage(chatId: string, onMessageSent: (msg: Message) => void) {
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

  const sendMessage = async (text: string) => {
    const optimisticMessage: Message = {
      id: crypto.randomUUID(),
      text,
      role: Role.User,
      imageUrls: [],
      toolCalls: [],
      toolResult: null,
      createdAt: new Date().toISOString()
    }

    onMessageSent(optimisticMessage)

    await new Promise((resolve) => setTimeout(resolve, 1500))
    // await sendMessageMutation({
    //   variables: {
    //     chatId,
    //     text
    //   }
    // })
  }

  return { sendMessage }
}
