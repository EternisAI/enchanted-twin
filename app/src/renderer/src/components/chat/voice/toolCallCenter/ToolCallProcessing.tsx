import { motion } from 'framer-motion'
import { ToolCall } from '@renderer/graphql/generated/graphql'
import { Badge } from '../../../ui/badge'
import { CheckCircle, LoaderIcon } from 'lucide-react'
import { cn } from '@renderer/lib/utils'
import { getToolConfig } from '../../config'

interface ToolCallProcessingProps {
  toolCalls: ToolCall[]
}

export default function ToolCallProcessing({ toolCalls }: ToolCallProcessingProps) {
  return (
    <div className="flex flex-col gap-2 w-full items-end">
      {toolCalls.map((toolCall) => {
        const { toolNameInProgress, toolNameCompleted } = getToolConfig(toolCall.name)
        const isCompleted = toolCall.isCompleted

        return (
          <motion.div
            key={toolCall.id}
            initial={{ opacity: 0, scale: 0.95 }}
            animate={{ opacity: 1, scale: 1 }}
            exit={{ opacity: 0, scale: 0.95 }}
            className="w-fit"
          >
            <Badge
              variant="outline"
              className={cn('flex items-center gap-1.5 w-full rounded-full border text-sm px-1')}
            >
              {isCompleted ? (
                <CheckCircle className="mr-1 text-green-600" />
              ) : (
                <LoaderIcon className="mr-1 animate-spin" />
              )}
              <span>{isCompleted ? toolNameCompleted : `${toolNameInProgress}`}</span>
            </Badge>
          </motion.div>
        )
      })}
    </div>
  )
}
