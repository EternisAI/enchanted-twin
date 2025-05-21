import { motion } from 'framer-motion'
import { ToolCall } from '@renderer/graphql/generated/graphql'
import { Badge } from '../../ui/badge'
import { CheckCircle, LoaderIcon } from 'lucide-react'
import { cn } from '@renderer/lib/utils'
import { formatToolName } from '../config'

interface ToolCallProcessingProps {
  toolCalls: ToolCall[]
}

export default function ToolCallProcessing({ toolCalls }: ToolCallProcessingProps) {
  return (
    <div className="flex flex-col gap-2 w-fit">
      {toolCalls.map((toolCall) => {
        const { toolNameInProgress, toolNameCompleted } = formatToolName(toolCall.name)
        const isCompleted = toolCall.isCompleted

        return (
          <motion.div
            key={toolCall.id}
            initial={{ opacity: 0, scale: 0.95 }}
            animate={{ opacity: 1, scale: 1 }}
            exit={{ opacity: 0, scale: 0.95 }}
            className="w-full"
          >
            <Badge
              variant="outline"
              className={cn(
                'flex items-center gap-1.5 w-full rounded-full border text-sm'
                // isCompleted ? '' : ''
              )}
            >
              {isCompleted ? (
                <CheckCircle className="mr-1 text-green-600" />
              ) : (
                <LoaderIcon className="h-3 w-3 mr-1 animate-spin" />
              )}
              <span>{isCompleted ? toolNameCompleted : `${toolNameInProgress}...`}</span>
            </Badge>
          </motion.div>
        )
      })}
    </div>
  )
}
