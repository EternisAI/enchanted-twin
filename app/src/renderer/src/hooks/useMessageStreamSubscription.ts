import { useSubscription } from '@apollo/client'
import { MessageStreamDocument, Role } from '@renderer/graphql/generated/graphql'

export function useMessageStreamSubscription(
  chatId: string,
  onChunk: (messageId: string, chunk: string, isComplete: boolean) => void
) {
  useSubscription(MessageStreamDocument, {
    variables: { chatId },
    onData: ({ data }) => {
      const payload = data.data?.messageStream
      if (payload && payload.role === Role.Assistant) {
        onChunk(payload.messageId, payload.chunk, payload.isComplete)
      }
    }
  })
}
