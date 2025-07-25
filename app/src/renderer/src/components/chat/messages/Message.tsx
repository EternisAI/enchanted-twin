import { Message, ToolCall as ToolCallType } from '@renderer/graphql/generated/graphql'
import { motion } from 'framer-motion'
import { cn } from '@renderer/lib/utils'
import { CheckCircle, ChevronRight, Lightbulb, LoaderIcon, XCircle } from 'lucide-react'
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
import { AnonymizedContent, type MarkdownComponent } from '@renderer/lib/anonymization'

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
            MarkdownComponent={MarkdownWrapper}
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

  if (toolCall.isCompleted && CustomComponent && !toolCall.error) {
    return <CustomComponent toolCall={toolCall} />
  }

  return (
    <div
      className={cn(
        'flex flex-col gap-2 pt-2',
        toolCall.isCompleted
          ? toolCall.error
            ? 'text-red-600'
            : 'text-green-600'
          : 'text-muted-foreground'
      )}
    >
      {toolCall.isCompleted ? (
        toolCall.error ? (
          <Badge className="text-red-600 border-red-500" variant="outline">
            <XCircle className="h-4 w-4" />
            <span>Failed: {toolNameCompleted}</span>
          </Badge>
        ) : (
          <Badge className="text-green-600 border-green-500" variant="outline">
            <CheckCircle className="h-4 w-4" />
            <span>{toolNameCompleted}</span>
          </Badge>
        )
      ) : (
        <Badge variant="outline" className="border-border">
          <LoaderIcon className="h-4 w-4 animate-spin" />
          <span>{toolNameInProgress}...</span>
        </Badge>
      )}

      {/* Show error message if present */}
      {toolCall.error && (
        <div className="text-sm text-red-600 max-w-md">
          <span className="font-medium">Error:</span> {toolCall.error}
        </div>
      )}
    </div>
  )
}

// Create a typed wrapper for Markdown component
const MarkdownWrapper: MarkdownComponent = ({ children }: { children: string }) => (
  <Markdown>{children}</Markdown>
)
