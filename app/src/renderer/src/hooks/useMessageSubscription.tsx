import { useSubscription } from '@apollo/client'
import { MessageAddedDocument, Message } from '@renderer/graphql/generated/graphql'

export function useMessageSubscription(chatId: string, onNewMessage: (msg: Message) => void) {
  useSubscription(MessageAddedDocument, {
    variables: { chatId },
    onData: ({ data }) => {
      const message = data.data?.messageAdded
      if (message) onNewMessage(message)
    },
    skip: !chatId
  })
}
