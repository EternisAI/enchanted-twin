import { useSubscription } from '@apollo/client'
import { ToolCall, ToolCallUpdatedDocument } from '@renderer/graphql/generated/graphql'

export function useToolCallUpdate(chatId: string, onNewToolCall: (toolCall: ToolCall) => void) {
  useSubscription(ToolCallUpdatedDocument, {
    variables: { chatId },
    onData: ({ data }) => {
      const toolCall = data.data?.toolCallUpdated
      if (toolCall) onNewToolCall(toolCall)
    },
    skip: !chatId
  })
}
