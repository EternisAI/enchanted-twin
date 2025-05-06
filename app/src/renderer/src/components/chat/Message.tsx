import { Message } from '@renderer/graphql/generated/graphql'
import { motion } from 'framer-motion'
import { cn } from '@renderer/lib/utils'
import { CheckCircle, ChevronRight, LoaderIcon } from 'lucide-react'
import { TOOL_NAMES } from './config'
import { Badge } from '../ui/badge'
import Markdown from './Markdown'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '../ui/collapsible'
import { useMemo } from 'react'

const messageAnimation = {
  initial: { opacity: 0, y: 20 },
  animate: { opacity: 1, y: 0 },
  transition: { duration: 0.3, ease: 'easeOut' }
}

function extractReasoningAndReply(raw: string): {
  thinkingText: string | null
  replyText: string
} {
  const thinkingTag = '<think>'
  const thinkingEndTag = '</think>'

  if (!raw.startsWith(thinkingTag)) return { thinkingText: null, replyText: raw }

  const closingIndex = raw.indexOf(thinkingEndTag)
  if (closingIndex !== -1) {
    const thinking = raw.slice(thinkingTag.length, closingIndex).trim()
    const rest = raw.slice(closingIndex + thinkingEndTag.length).trim()
    return { thinkingText: thinking, replyText: rest }
  } else {
    const thinking = raw.slice(thinkingTag.length).trim()
    return { thinkingText: thinking, replyText: '' }
  }
}

export function UserMessageBubble({ message }: { message: Message }) {
  return (
    <motion.div
      className="flex justify-end"
      initial="initial"
      animate="animate"
      variants={messageAnimation}
    >
      <div className="bg-accent dark:bg-black dark:border dark:border-border text-foreground rounded-lg px-4 py-2 max-w-md">
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
        <div className="text-xs text-muted-foreground pt-1">
          {new Date(message.createdAt).toLocaleTimeString()}
        </div>
      </div>
    </motion.div>
  )
}

export function AssistantMessageBubble({ message }: { message: Message }) {
  const { thinkingText, replyText } = useMemo(
    () => extractReasoningAndReply(message.text || ''),
    [message.text]
  )

  return (
    <motion.div
      className="flex justify-start"
      initial="initial"
      animate="animate"
      variants={messageAnimation}
    >
      <div className="flex flex-col text-foreground py-2 max-w-[90%]">
        {thinkingText && (
          <Collapsible className="flex flex-col gap-2 pb-2">
            <CollapsibleTrigger className="flex items-center gap-1 text-sm text-muted-foreground cursor-pointer hover:underline group">
              <ChevronRight className="h-4 w-4 transition-transform group-data-[state=open]:rotate-90" />
              <span className="font-medium">💭 Reasoning</span>
            </CollapsibleTrigger>
            <CollapsibleContent
              className={cn(
                'overflow-hidden transition-all data-[state=closed]:animate-collapsible-up data-[state=open]:animate-collapsible-down',
                'mt-2 text-muted-foreground text-sm italic bg-muted p-3 rounded-lg border border-border'
              )}
            >
              {thinkingText}
            </CollapsibleContent>
          </Collapsible>
        )}

        {replyText && <Markdown>{replyText}</Markdown>}
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
        <div className="text-xs text-muted-foreground ">
          {new Date(message.createdAt).toLocaleTimeString()}
        </div>
      </div>
    </motion.div>
  )
}
