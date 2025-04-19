import { Message } from '@renderer/graphql/generated/graphql'
import { motion } from 'framer-motion'
import { Markdown } from './Markdown'
import { cn } from '@renderer/lib/utils'
import { CheckCircle, LoaderIcon } from 'lucide-react'
import { TOOL_NAMES } from './config'
import { Badge } from '../ui/badge'

const messageAnimation = {
  initial: { opacity: 0, y: 20 },
  animate: { opacity: 1, y: 0 },
  transition: { duration: 0.3, ease: 'easeOut' }
}

export function UserMessageBubble({ message }: { message: Message }) {
  return (
    <motion.div
      className="flex justify-end"
      initial="initial"
      animate="animate"
      variants={messageAnimation}
    >
      <div className="bg-white text-gray-800 rounded-lg px-4 py-2 shadow max-w-md">
        {message.text && <p>{message.text}</p>}
        {message.imageUrls.length > 0 && (
          <div className="flex gap-2 mt-2">
            {message.imageUrls.map((url, i) => (
              <img
                key={i}
                src={url}
                alt={`attachment-${i}`}
                className="inline-block h-48 w-48 object-cover rounded"
              />
            ))}
          </div>
        )}
        <div className="text-xs text-gray-500 pt-1">
          {new Date(message.createdAt).toLocaleTimeString()}
        </div>
      </div>
    </motion.div>
  )
}

export function AssistantMessageBubble({ message }: { message: Message }) {
  return (
    <motion.div
      className="flex justify-start"
      initial="initial"
      animate="animate"
      variants={messageAnimation}
    >
      <div className="bg-gray-100 text-gray-800 rounded-lg px-4 py-2 shadow max-w-md">
        {message.text && <Markdown>{message.text}</Markdown>}
        {message.imageUrls.length > 0 && (
          <div className="flex flex-col gap-2 my-2">
            {message.imageUrls.map((url, i) => (
              <img
                key={i}
                src={url}
                alt={`attachment-${i}`}
                className="inline-block h-80 w-80 object-cover rounded"
              />
            ))}
          </div>
        )}

        <div className="flex flex-wrap gap-4 pt-2">
          {message.toolCalls.map((toolCall) => {
            const toolNameInProgress = TOOL_NAMES[toolCall.name]?.inProgress || toolCall.name
            const toolNameCompleted = TOOL_NAMES[toolCall.name]?.completed || toolCall.name

            return (
              <div
                key={toolCall.id}
                className={cn(
                  'flex items-center gap-2',
                  toolCall.isCompleted ? 'text-green-600' : 'text-muted-foreground'
                )}
              >
                {toolCall.isCompleted ? (
                  <Badge className="text-green-600 border-green-500" variant="outline">
                    <CheckCircle className="h-4 w-4" />
                    <span>{toolNameCompleted}</span>
                  </Badge>
                ) : (
                  <Badge variant="outline" className="border-gray-300">
                    <LoaderIcon className="h-4 w-4 animate-spin" />
                    <span>{toolNameInProgress}...</span>
                  </Badge>
                )}
              </div>
            )
          })}
        </div>

        {/* {message.toolResults.length > 0 && (
          <div className="mt-3 bg-green-50 p-2 rounded text-xs text-gray-700 whitespace-pre-wrap">
            <strong>Tool Result:</strong>
            <pre>{JSON.stringify(message.toolResults, null, 2)}</pre>
          </div>
        )} */}
        <div className="text-xs text-gray-500 pt-1">
          {new Date(message.createdAt).toLocaleTimeString()}
        </div>
      </div>
    </motion.div>
  )
}
