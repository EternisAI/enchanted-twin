import { useSubscription } from '@apollo/client'
import { ProcessMessageHistoryStreamDocument, Role } from '@renderer/graphql/generated/graphql'

export function useProcessMessageHistoryStream(
  chatId: string,
  messages: Array<{ text: string; role: Role }>,
  isOnboarding: boolean,
  onChunk: (messageId: string, chunk: string, isComplete: boolean, imageUrls: string[]) => void
) {
  useSubscription(ProcessMessageHistoryStreamDocument, {
    variables: { chatId, messages, isOnboarding },
    onData: ({ data }) => {
      const payload = data.data?.processMessageHistoryStream
      if (payload && payload.role === Role.Assistant) {
        onChunk(payload.messageId, payload.chunk, payload.isComplete, payload.imageUrls)
      }
    },
    skip: !chatId || messages.length === 0
  })
}
