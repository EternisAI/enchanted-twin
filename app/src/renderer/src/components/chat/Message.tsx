import React from 'react'
import { Message, ToolCall as ToolCallType } from '@renderer/graphql/generated/graphql'
import { motion } from 'framer-motion'
import { cn } from '@renderer/lib/utils'
import {
  CheckCircle,
  ChevronRight,
  Eye,
  EyeClosed,
  Lightbulb,
  LoaderIcon,
  Volume2,
  VolumeOff
} from 'lucide-react'
import { extractReasoningAndReply, getToolConfig } from './config'
import { Badge } from '../ui/badge'
import ImagePreview from './ImagePreview'
import Markdown from './Markdown'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '../ui/collapsible'
import { useTTS } from '@renderer/hooks/useTTS'
import { useMemo, useState } from 'react'

const messageAnimation = {
  initial: { opacity: 0, y: 20 },
  animate: { opacity: 1, y: 0 },
  transition: { duration: 0.3, ease: 'easeOut' }
}

export function UserMessageBubble({ message }: { message: Message }) {
  const [isAnonymized, setIsAnonymized] = useState(false)

  const privacyDictJson = {
    Arthur: 'Max',
    google: 'BigCorp',
    'san francisco': 'US CITY'
  }

  return (
    <motion.div
      className="flex justify-end"
      initial="initial"
      animate="animate"
      variants={messageAnimation}
    >
      <div className="flex flex-col gap-1 max-w-md">
        <div className="bg-accent dark:bg-black dark:border dark:border-border text-foreground rounded-lg px-4 py-2 max-w-md relative group">
          {message.text && <p>{anonymizeText(message.text, privacyDictJson, isAnonymized)}</p>}
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
        </div>
        <div className="flex items-center gap-2 w-full">
          <button
            onClick={() => setIsAnonymized(!isAnonymized)}
            className="p-1 rounded-md bg-accent cursor-pointer hover:bg-accent/50"
            style={{ pointerEvents: 'auto' }}
            tabIndex={-1}
            aria-label={isAnonymized ? 'Show original message' : 'Anonymize message'}
          >
            {isAnonymized ? (
              <EyeClosed className="h-4 w-4 text-primary" />
            ) : (
              <Eye className="h-4 w-4 text-primary" />
            )}
          </button>
          <div className="text-xs text-muted-foreground">
            {new Date(message.createdAt).toLocaleTimeString()}
          </div>
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
        <div className="flex flex-row items-center gap-4 justify-between w-full">
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

const anonymizeText = (
  text: string,
  privacyDictJson: Record<string, string>,
  isAnonymized: boolean
) => {
  if (!isAnonymized) return text

  let parts: (string | React.ReactElement)[] = [text]

  Object.entries(privacyDictJson).forEach(([original, replacement]) => {
    const regex = new RegExp(`(${original})`, 'gi')
    parts = parts.flatMap((part) => {
      if (typeof part === 'string') {
        return part
          .split(regex)
          .map((segment, index) => {
            if (regex.test(segment)) {
              return (
                <span
                  key={`${original}-${index}`}
                  className="bg-muted-foreground text-secondary px-1.25 py-0.25 rounded text-foreground font-medium"
                >
                  {replacement}
                </span>
              )
            }
            return segment
          })
          .filter((segment) => segment !== '')
      }
      return part
    })
  })

  return <span>{parts}</span>
}
