import { ToolCall } from '@renderer/graphql/generated/graphql'
import { Button } from '../ui/button'

type HolonToolResult = {
  title: string
  description: string
  actions: string[]
}

export default function HolonToolComponent({ toolCall }: { toolCall: ToolCall }) {
  const toolCallResult = JSON.parse(toolCall.result?.content || '{}') as HolonToolResult
  console.log('toolCall', toolCallResult)

  return (
    <div className="flex flex-col gap-2">
      <p className="text-sm text-primary">{toolCallResult.title}</p>
      <p className="text-sm text-muted-foreground">{toolCallResult.description}</p>
      <div className="flex flex-col gap-2">
        {(toolCallResult?.actions || []).map((action, index) => (
          <Button key={index}>{action}</Button>
        ))}
      </div>
    </div>
  )
}
