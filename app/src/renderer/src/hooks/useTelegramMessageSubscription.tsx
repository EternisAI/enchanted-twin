import { useSubscription } from '@apollo/client'
import { TelegramMessageAddedDocument, Message } from '@renderer/graphql/generated/graphql'

export function useTelegramMessageSubscription(chatUUID: string, onNewMessage: (msg: Message) => void) {
  useSubscription(TelegramMessageAddedDocument, {
    variables: { chatUUID },
    onData: ({ data }) => {
      const message = data.data?.telegramMessageAdded
      if (message) onNewMessage(message)
    },
    skip: !chatUUID
  })
}
