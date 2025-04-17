import { Message } from '@renderer/graphql/generated/graphql'
import { motion } from 'framer-motion'
import { Markdown } from './Markdown'
import { cn } from '@renderer/lib/utils'
import { CheckCircle, LoaderIcon } from 'lucide-react'

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
      <div className="bg-background text-foreground rounded-lg px-4 py-2 shadow max-w-md dark:border">
        {message.text && <p>{message.text}</p>}
        {message.imageUrls.length > 0 && (
          <div className="mt-2 space-x-2">
            {message.imageUrls.map((url, i) => (
              <img
                key={i}
                src={url}
                alt={`attachment-${i}`}
                className="inline-block max-h-24 max-w-24 object-fit rounded"
              />
            ))}
          </div>
        )}
        <div className="text-xs text-muted-foreground pt-1">
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
      <div className="bg-accent text-foreground rounded-lg px-4 py-2 shadow max-w-md">
        {message.text && <Markdown>{message.text}</Markdown>}
        {message.imageUrls.length > 0 && (
          <div className="mt-2 space-x-2">
            {message.imageUrls.map((url, i) => (
              <img
                key={i}
                src={url}
                alt={`attachment-${i}`}
                className="inline-block h-24 w-24 object-cover rounded"
              />
            ))}
          </div>
        )}

        {message.toolCalls.map((toolCall) => {
          return (
            <div
              key={toolCall.id}
              className={cn(
                'flex items-center gap-2',
                toolCall.isCompleted ? 'text-green-600' : 'text-muted-foreground'
              )}
            >
              {toolCall.isCompleted ? (
                <>
                  <CheckCircle className="h-4 w-4" />
                  <span>Used {toolCall.name}</span>
                </>
              ) : (
                <>
                  <LoaderIcon className="h-4 w-4 animate-spin" />
                  <span>Using {toolCall.name}...</span>
                </>
              )}
            </div>
          )
        })}

        {message.toolResults.length > 0 && (
          <div className="mt-3 bg-green-50 p-2 rounded text-xs text-foreground whitespace-pre-wrap">
            <strong>Tool Result:</strong>
            <pre>{JSON.stringify(message.toolResults, null, 2)}</pre>
          </div>
        )}
        <div className="text-xs text-muted-foreground pt-1">
          {new Date(message.createdAt).toLocaleTimeString()}
        </div>
      </div>
    </motion.div>
  )
}
