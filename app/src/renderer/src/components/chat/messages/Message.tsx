import React from 'react'
import { Message, ToolCall as ToolCallType } from '@renderer/graphql/generated/graphql'
import { motion } from 'framer-motion'
import { cn } from '@renderer/lib/utils'
import { CheckCircle, ChevronRight, Lightbulb, LoaderIcon } from 'lucide-react'
import { extractReasoningAndReply, getToolConfig } from '@renderer/components/chat/config'
import { Badge } from '@renderer/components/ui/badge'
import ImagePreview from './ImagePreview'
import Markdown from '@renderer/components/chat/messages/Markdown'
import { FeedbackPopover } from './actions/FeedbackPopover'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger
} from '@renderer/components/ui/collapsible'
import { useMemo } from 'react'
import { ReadAloudButton } from './actions/ReadAloudButton'
import { MessageActionsBar } from './actions/MessageActionsBar'
import {
  sortKeysByLengthDesc,
  replaceWithCasePreservation,
  anonymizeTextForMarkdownString
} from '@renderer/lib/anonymization'

const messageAnimation = {
  initial: { opacity: 0, y: 20 },
  animate: { opacity: 1, y: 0 },
  transition: { duration: 0.3, ease: 'easeOut' }
}

export function UserMessageBubble({
  message,
  isAnonymized = false,
  chatPrivacyDict,
  showTimestamp = true,
  className
}: {
  message: Message
  chatPrivacyDict: string | null
  isAnonymized?: boolean
  showTimestamp?: boolean
  className?: string
}) {
  return (
    <motion.div
      className={cn('flex justify-end', className)}
      initial="initial"
      animate="animate"
      variants={messageAnimation}
    >
      <div className="flex flex-col gap-1 max-w-md">
        <div className="bg-accent text-foreground rounded-lg px-4 py-2 max-w-md relative group break-words">
          {message.text && (
            <p>
              <AnonymizedContent
                text={message.text}
                chatPrivacyDict={chatPrivacyDict}
                isAnonymized={isAnonymized}
              />
            </p>
          )}
          {message.imageUrls.length > 0 && (
            <div className="grid grid-cols-4 gap-2 my-2 w-fit max-w-full">
              {message.imageUrls.map((url, i) => (
                <ImagePreview
                  key={i}
                  src={url}
                  alt={`attachment-${i}`}
                  thumbClassName="h-32 w-32 rounded-sm"
                />
              ))}
            </div>
          )}
        </div>
        {showTimestamp && (
          <div className="flex justify-end items-center gap-2 w-full">
            <div className="text-[9px] text-muted-foreground font-mono">
              {new Date(message.createdAt).toLocaleTimeString()}
            </div>
          </div>
        )}
      </div>
    </motion.div>
  )
}

export function AssistantMessageBubble({
  message,
  messages,
  isAnonymized = false,
  chatPrivacyDict
}: {
  message: Message
  messages?: Message[]
  isAnonymized?: boolean
  chatPrivacyDict: string | null
  showTimestamp?: boolean
}) {
  const { thinkingText, replyText } = useMemo(
    () => extractReasoningAndReply(message.text || ''),
    [message.text]
  )

  const isStillThinking = thinkingText?.trim() !== '' && !replyText

  const shouldShowOriginalContent = useMemo(() => {
    const hideContentForTools = ['preview_thread', 'send_to_holon']

    return !message.toolCalls.some((toolCall) => {
      return toolCall.isCompleted && hideContentForTools.includes(toolCall.name)
    })
  }, [message.toolCalls])

  return (
    <motion.div
      className="flex justify-start"
      initial="initial"
      animate="animate"
      variants={messageAnimation}
    >
      <div className="flex flex-col text-foreground py-1 max-w-[90%] relative group gap-1">
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
              <AnonymizedContent
                text={thinkingText}
                chatPrivacyDict={chatPrivacyDict}
                isAnonymized={isAnonymized}
              />
            </CollapsibleContent>
          </Collapsible>
        )}

        {replyText && shouldShowOriginalContent && (
          <AnonymizedContent
            text={replyText}
            chatPrivacyDict={chatPrivacyDict}
            isAnonymized={isAnonymized}
            asMarkdown={true}
          />
        )}
        {message.imageUrls.length > 0 && (
          <div className="grid grid-cols-4 gap-2 my-2 w-fit max-w-full">
            {message.imageUrls.map((url, i) => (
              <ImagePreview
                key={i}
                src={url}
                alt={`attachment-${i}`}
                thumbClassName="inline-block h-32 w-32 object-cover rounded-sm"
              />
            ))}
          </div>
        )}
        <div className="flex flex-row items-start gap-4 justify-start w-full">
          <div className="flex flex-col gap-2">
            <div className="flex flex-wrap gap-4 items-center">
              {message.toolCalls.map((toolCall) => (
                <ToolCall key={toolCall.id} toolCall={toolCall} />
              ))}
            </div>
          </div>
        </div>
        {replyText && replyText.trim() && (
          <MessageActionsBar>
            <FeedbackPopover
              currentMessage={message}
              messages={messages || []}
              chatPrivacyDict={chatPrivacyDict}
            />
            <ReadAloudButton text={replyText} />
          </MessageActionsBar>
        )}
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
        <Badge variant="outline" className="border-border">
          <LoaderIcon className="h-4 w-4 animate-spin" />
          <span>{toolNameInProgress}...</span>
        </Badge>
      )}
    </div>
  )
}

const anonymizeText = (text: string, privacyDictJson: string | null, isAnonymized: boolean) => {
  if (!privacyDictJson || !isAnonymized) return text

  let privacyDict: Record<string, string>
  try {
    privacyDict = JSON.parse(privacyDictJson) as Record<string, string>
  } catch {
    // If JSON is malformed, return original text
    return text
  }

  let parts: (string | React.ReactElement)[] = [text]

  // Sort rules by length (longest first) to avoid partial matches
  const sortedOriginals = sortKeysByLengthDesc(privacyDict)

  sortedOriginals.forEach((original) => {
    const replacement = privacyDict[original]

    // Skip if replacement is not a string
    if (typeof replacement !== 'string') {
      return
    }

    parts = parts.flatMap((part) => {
      if (typeof part === 'string') {
        // Use the case-preserving replacement logic
        const processedText = replaceWithCasePreservation(part, original, replacement)

        // If no replacement occurred, return the original part
        if (processedText === part) {
          return [part]
        }

        // Now split by the replacement to create React elements
        const segments: (string | React.ReactElement)[] = []
        let searchStart = 0

        while (true) {
          const lowerText = processedText.toLowerCase()
          const idx = lowerText.indexOf(replacement.toLowerCase(), searchStart)

          if (idx === -1) {
            // No more replacements, add the rest of the text
            if (searchStart < processedText.length) {
              segments.push(processedText.substring(searchStart))
            }
            break
          }

          // Add text before the replacement
          if (idx > searchStart) {
            segments.push(processedText.substring(searchStart, idx))
          }

          // Add the replacement as a React element
          segments.push(
            <span
              key={`${original}-${idx}`}
              className="bg-muted-foreground px-1.25 py-0.25 rounded text-primary-foreground font-medium"
            >
              {processedText.substring(idx, idx + replacement.length)}
            </span>
          )

          searchStart = idx + replacement.length
        }

        return segments.filter((segment) => segment !== '')
      }
      return part
    })
  })

  return <span>{parts}</span>
}

function anonymizeTextForMarkdown(
  text: string,
  privacyDictJson: string | null,
  isAnonymized: boolean
): string {
  if (!privacyDictJson || !isAnonymized) return text

  let privacyDict: Record<string, string>
  try {
    privacyDict = JSON.parse(privacyDictJson) as Record<string, string>
  } catch {
    // If JSON is malformed, return original text
    return text
  }

  return anonymizeTextForMarkdownString(text, privacyDict)
}

function AnonymizedContent({
  text,
  chatPrivacyDict,
  isAnonymized,
  asMarkdown = false
}: {
  text: string
  chatPrivacyDict: string | null
  isAnonymized: boolean
  asMarkdown?: boolean
}) {
  if (asMarkdown) {
    const mdText = anonymizeTextForMarkdown(text, chatPrivacyDict, isAnonymized)
    return <Markdown>{mdText}</Markdown>
  } else {
    return anonymizeText(text, chatPrivacyDict, isAnonymized)
  }
}
