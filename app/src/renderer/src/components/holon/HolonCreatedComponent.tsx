import { CheckCircle } from 'lucide-react'
import { Badge } from '../ui/badge'
import { Button } from '../ui/button'
import { useNavigate } from '@tanstack/react-router'
import { ToolCall } from '@renderer/graphql/generated/graphql'

type HolonCreatedResult = {
  id: string
  title: string
  content: string
}

export default function HolonCreatedComponent({ toolCall }: { toolCall: ToolCall }) {
  const toolCallResult = JSON.parse(toolCall.result?.content || '{}') as HolonCreatedResult
  console.log('HolonCreatedComponent toolCall result', toolCallResult)
  const navigate = useNavigate()

  return (
    <div className="flex flex-col gap-2">
      <Badge className="text-green-600 border-green-500" variant="outline">
        <CheckCircle className="h-4 w-4" />
        <span>Holon Created</span>
      </Badge>
      {toolCallResult.id && (
        <Button
          onClick={() => {
            navigate({ to: '/holon/$threadId', params: { threadId: toolCallResult.id } })
          }}
          variant="outline"
        >
          See Holon Thread
        </Button>
      )}
    </div>
  )
}
