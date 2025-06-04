import { ToolCall } from '@renderer/graphql/generated/graphql'
import { Button } from '../ui/button'
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

  return (
    <div className="flex flex-col gap-2 border border-gray-200 p-3 rounded-lg w-full">
      <p className="text-sm text-primary">{toolCallResult.title}</p>
      <p className="text-sm text-muted-foreground">{toolCallResult.content}</p>
      <div className="flex flex-col gap-2">
        {(toolCallResult?.actions || []).map((action, index) => (
          <Button key={index}>{action}</Button>
        ))}
      </div>
    </div>
  )
}
