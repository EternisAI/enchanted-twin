import { ToolCall } from '@renderer/graphql/generated/graphql'
import HolonThreadPreview from './HolonThreadPreview'

type HolonToolResult = {
  id: string
  title: string
  content: string
  actions: string[]
}

export default function HolonToolComponent({ toolCall }: { toolCall: ToolCall }) {
  const toolCallResult = JSON.parse(toolCall.result?.content || '{}') as HolonToolResult
  console.log('toolCall', toolCallResult)

  return (
    <HolonThreadPreview
      thread={{
        id: toolCallResult.id,
        title: toolCallResult.title,
        content: toolCallResult.content,
        actions: toolCallResult.actions,
        imageURLs: []
      }}
    />
  )
}
