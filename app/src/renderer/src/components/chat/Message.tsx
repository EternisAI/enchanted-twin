import { Message, ToolCall as ToolCallType } from '@renderer/graphql/generated/graphql'
import { motion } from 'framer-motion'
import { cn } from '@renderer/lib/utils'
import { CheckCircle, ChevronRight, Lightbulb, LoaderIcon, Volume2, VolumeOff } from 'lucide-react'
import { extractReasoningAndReply, getToolConfig } from './config'
import { Badge } from '../ui/badge'
import ImagePreview from './ImagePreview'
import Markdown from './Markdown'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '../ui/collapsible'
import { useTTS } from '@renderer/hooks/useTTS'
import { useMemo } from 'react'

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
      <div className="bg-accent dark:bg-black dark:border dark:border-border text-foreground rounded-lg px-4 py-2 max-w-md">
        {message.text && <p>{message.text}</p>}
        {message.imageUrls.length > 0 && (
          <div className="grid grid-cols-4 gap-y-4 my-2">
            {message.imageUrls.map((url, i) => (
              <ImagePreview
                key={i}
                src={url}
                alt={`attachment-${i}`}
                thumbClassName="inline-block h-32 w-32 object-cover rounded"
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
  const { speak, stop, isSpeaking } = useTTS()
  const { thinkingText, replyText } = useMemo(
    () => extractReasoningAndReply(message.text || ''),
    [message.text]
  )

  const isStillThinking = thinkingText?.trim() !== '' && !replyText

  return (
    <motion.div
      className="flex justify-start"
      initial="initial"
      animate="animate"
      variants={messageAnimation}
    >
      <div className="flex flex-col text-foreground py-2 max-w-[90%] relative group">
        {thinkingText && (
          <Collapsible className="flex flex-col gap-2 pb-2">
            <CollapsibleTrigger className="flex items-center gap-1 text-sm text-muted-foreground cursor-pointer hover:underline group">
              <ChevronRight className="h-4 w-4 transition-transform group-data-[state=open]:rotate-90" />

              {isStillThinking ? (
                <motion.div
                  variants={{
                    animate: {
                      scale: [1, 1.02, 1],
                      opacity: [0.7, 1, 0.7],
                      transition: {
                        duration: 2,
                        repeat: Infinity,
                        ease: 'easeInOut'
                      }
                    }
                  }}
                  animate="animate"
                  className="font-medium flex items-center gap-1"
                >
                  <Lightbulb className="h-4 w-4" />
                  Reasoning...
                </motion.div>
              ) : (
                <span className="font-medium flex items-center gap-1">
                  <Lightbulb className="w-4 h-5" /> Reasoning
                </span>
              )}
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
          <div className="grid grid-cols-4 gap-y-4 my-2">
            {message.imageUrls.map((url, i) => (
              <ImagePreview
                key={i}
                src={url}
                alt={`attachment-${i}`}
                thumbClassName="inline-block h-40 w-40 object-cover rounded"
              />
            ))}
          </div>
        )}
        <div className="flex flex-row items-center  gap-4 justify-between w-full">
          <div className="flex flex-col gap-2">
            <div className="flex flex-wrap gap-4 items-center">
              {message.toolCalls.map((toolCall) => (
                <ToolCall key={toolCall.id} toolCall={toolCall} />
              ))}
            </div>
            <div className="text-xs text-left text-muted-foreground ">
              {new Date(message.createdAt).toLocaleTimeString()}
            </div>
          </div>
          {replyText && replyText.trim() && (
            <span className="flex items-center opacity-0 group-hover:opacity-100 transition-opacity">
              {isSpeaking ? (
                <button
                  onClick={stop}
                  className="transition-opacity p-1 rounded-full bg-background/80 hover:bg-muted z-10"
                  style={{ pointerEvents: 'auto' }}
                  tabIndex={-1}
                  aria-label="Stop message audio"
                >
                  <VolumeOff className="h-5 w-5 text-primary" />
                </button>
              ) : (
                <button
                  onClick={() => speak(replyText || '')}
                  className="transition-opacity p-1 rounded-full bg-background/80 hover:bg-muted z-10"
                  style={{ pointerEvents: 'auto' }}
                  tabIndex={-1}
                  aria-label="Play message audio"
                >
                  <Volume2 className="h-5 w-5 text-primary" />
                </button>
              )}
            </span>
          )}
        </div>
      </div>
    </motion.div>
  )
}

function ToolCall({ toolCall }: { toolCall: ToolCallType }) {
  const {
    toolNameInProgress,
    toolNameCompleted,
    customComponent: CustomComponent
  } = getToolConfig(toolCall.name)

  if (toolCall.isCompleted && CustomComponent) {
    return <CustomComponent toolCall={toolCall} />
  }

  return (
    <div
      className={cn(
        'flex items-center gap-2 pt-2',
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
}
