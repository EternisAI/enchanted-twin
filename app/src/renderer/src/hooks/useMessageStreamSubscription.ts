import { useSubscription } from '@apollo/client'
import { MessageStreamDocument, Role } from '@renderer/graphql/generated/graphql'

export interface StreamChunkData {
  messageId: string
  chunk: string
  isComplete: boolean
  imageUrls: string[]
  accumulatedMessage: string
  deanonymizedAccumulatedMessage: string
}

export function useMessageStreamSubscription(
  chatId: string,
  onChunk: (data: StreamChunkData) => void
) {
  useSubscription(MessageStreamDocument, {
    variables: { chatId },
    onData: ({ data }) => {
      const payload = data.data?.messageStream
      if (payload && payload.role === Role.Assistant) {
        onChunk({
          messageId: payload.messageId,
          chunk: payload.chunk,
          isComplete: payload.isComplete,
          imageUrls: payload.imageUrls,
          accumulatedMessage: payload.accumulatedMessage,
          deanonymizedAccumulatedMessage: payload.deanonymizedAccumulatedMessage
        })
      }
    }
  })
}
