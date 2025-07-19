import { useState, useCallback } from 'react'
import { Flag, Send, X, Loader2, Check } from 'lucide-react'
import { motion, AnimatePresence } from 'framer-motion'
import { Button } from '@renderer/components/ui/button'
import { Textarea } from '@renderer/components/ui/textarea'
import { Popover, PopoverContent, PopoverTrigger } from '@renderer/components/ui/popover'
import { ActionButton } from '@renderer/components/chat/messages/actions/ActionButton'
import { Message, Role } from '@renderer/graphql/generated/graphql'
import { toast } from 'sonner'

type FeedbackType = 'not-enough-anonymization' | 'too-much-anonymized' | 'other' | null

interface FeedbackPopoverProps {
  currentMessage: Message
  messages: Message[]
  chatPrivacyDict: string | null
}

export function FeedbackPopover({
  currentMessage,
  messages,
  chatPrivacyDict
}: FeedbackPopoverProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [selectedType, setSelectedType] = useState<FeedbackType>(null)
  const [feedback, setFeedback] = useState('')
  const [hasInitialized, setHasInitialized] = useState(false)
  const [isLoading, setIsLoading] = useState(false)
  const [feedbackSubmitted, setFeedbackSubmitted] = useState(false)

  const hasUnsavedChanges = feedback.trim() !== '' || selectedType !== null

  // Find the user message that triggered this AI response
  const getUserMessage = useCallback(() => {
    const currentIndex = messages.findIndex((m) => m.id === currentMessage.id)
    // Look backwards for the most recent user message
    for (let i = currentIndex - 1; i >= 0; i--) {
      if (messages[i].role === Role.User) {
        return messages[i]
      }
    }
    return null
  }, [messages, currentMessage.id])

  const handleFeedbackChange = useCallback((value: string) => {
    setFeedback(value)
  }, [])

  const handleTypeSelect = useCallback((type: FeedbackType) => {
    setSelectedType(type)
    setHasInitialized(true)
  }, [])

  const resetState = useCallback(() => {
    setIsOpen(false)
    setSelectedType(null)
    setFeedback('')
    setHasInitialized(false)
    setIsLoading(false)
  }, [])

  const submitFeedback = useCallback(async () => {
    if (!selectedType) throw new Error('No feedback type selected')

    const userMessage = getUserMessage()
    const feedbackData = {
      feedbackType: selectedType,
      feedbackText: feedback,
      aiMessage: currentMessage.text,
      userMessage: userMessage?.text || null,
      privacyDict: chatPrivacyDict,
      messageId: currentMessage.id,
      userMessageId: userMessage?.id || null,
      timestamp: new Date().toISOString()
    }

    await (
      window.api.analytics as unknown as {
        captureFeedback: (event: string, properties: Record<string, unknown>) => Promise<void>
      }
    ).captureFeedback('message_feedback_submitted', feedbackData)
  }, [selectedType, feedback, currentMessage, getUserMessage, chatPrivacyDict])

  const handleSubmit = useCallback(() => {
    if (!selectedType || isLoading) return

    const attemptSubmission = async () => {
      setIsLoading(true)

      try {
        await submitFeedback()
        setFeedbackSubmitted(true)
        toast.success('Feedback sent successfully!')
        resetState()
      } catch (error) {
        console.error('Failed to submit feedback:', error)
        setIsLoading(false) // Re-enable form on error

        // Show error toast with manual retry option
        toast.error(`Failed to send feedback: ${(error as Error).message}`, {
          action: {
            label: 'Retry',
            onClick: attemptSubmission
          }
        })
      }
    }

    attemptSubmission()
  }, [submitFeedback, selectedType, resetState, isLoading])

  const handleOpenChange = useCallback(
    (open: boolean) => {
      // Prevent closing while loading
      if (isLoading) return

      if (open) {
        setIsOpen(true)
      } else if (!hasUnsavedChanges) {
        resetState()
      } else {
        // User tried to close with unsaved changes, show confirmation
        const confirmed = confirm(
          'You have unsaved feedback. Are you sure you want to close without sending?'
        )
        if (confirmed) {
          resetState()
        } else {
          // Keep popover open by not changing state
          setIsOpen(true)
        }
      }
    },
    [hasUnsavedChanges, resetState, isLoading]
  )

  return (
    <Popover open={isOpen} onOpenChange={handleOpenChange}>
      <PopoverTrigger asChild>
        <ActionButton
          tooltipLabel={feedbackSubmitted ? 'Feedback already submitted' : 'Give feedback'}
          onClick={() => !feedbackSubmitted && setIsOpen(true)}
          aria-label={feedbackSubmitted ? 'Feedback already submitted' : 'Give feedback'}
          disabled={feedbackSubmitted}
        >
          {feedbackSubmitted ? (
            <Check className="h-4 w-4 text-green-600" />
          ) : (
            <Flag className="h-4 w-4" />
          )}
        </ActionButton>
      </PopoverTrigger>
      <PopoverContent
        className="w-100 min-h-[250px] overflow-hidden bg-background/90 backdrop-blur-md"
        align="start"
        side="bottom"
      >
        <motion.div
          layout
          className="space-y-4"
          initial={false}
          transition={{
            duration: 0.2,
            ease: 'easeInOut',
            layout: { duration: 0.2, ease: 'easeInOut' }
          }}
        >
          <div className="flex items-center justify-between">
            <h4 className="font-semibold text-base">What&apos;s wrong with this message?</h4>
            <Button
              variant="ghost"
              size="icon"
              disabled={isLoading}
              onClick={() => {
                if (hasUnsavedChanges) {
                  const confirmed = confirm(
                    'You have unsaved feedback. Are you sure you want to close without sending?'
                  )
                  if (confirmed) resetState()
                } else {
                  resetState()
                }
              }}
              className="h-6 w-6 rounded-full"
              aria-label="Close feedback"
            >
              <X className="h-4 w-4" />
            </Button>
          </div>

          <AnimatePresence mode="wait">
            {!selectedType ? (
              <motion.div
                key="selection"
                initial={hasInitialized ? { opacity: 0, x: -20 } : { opacity: 1, x: 0 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: -20 }}
                transition={{ duration: 0.2, ease: 'easeInOut' }}
                className="space-y-3"
              >
                <div className="space-y-2">
                  <Button
                    variant="outline"
                    disabled={isLoading}
                    className="w-full justify-start text-left h-auto p-3"
                    onClick={() => handleTypeSelect('not-enough-anonymization')}
                  >
                    <div>
                      <div className="font-medium text-sm">Not enough anonymization</div>
                      <div className="text-xs text-muted-foreground">
                        Personal information is still visible
                      </div>
                    </div>
                  </Button>

                  <Button
                    variant="outline"
                    disabled={isLoading}
                    className="w-full justify-start text-left h-auto p-3"
                    onClick={() => handleTypeSelect('too-much-anonymized')}
                  >
                    <div>
                      <div className="font-medium text-sm">Too much anonymized</div>
                      <div className="text-xs text-muted-foreground">
                        Important context was removed
                      </div>
                    </div>
                  </Button>

                  <Button
                    variant="outline"
                    disabled={isLoading}
                    className="w-full justify-start text-left h-auto p-3"
                    onClick={() => handleTypeSelect('other')}
                  >
                    <div>
                      <div className="font-medium text-sm">Other</div>
                      <div className="text-xs text-muted-foreground">
                        Something else about this message
                      </div>
                    </div>
                  </Button>
                </div>
              </motion.div>
            ) : (
              <motion.div
                key="feedback-form"
                initial={{ opacity: 0, x: 20 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: 20 }}
                transition={{ duration: 0.2, ease: 'easeInOut' }}
                className="space-y-3"
              >
                <div className="flex items-center justify-between">
                  <p className="text-sm font-medium">
                    {selectedType === 'not-enough-anonymization' && 'Not enough anonymization'}
                    {selectedType === 'too-much-anonymized' && 'Too much anonymized'}
                    {selectedType === 'other' && 'Other feedback'}
                  </p>
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={isLoading}
                    onClick={() => {
                      setSelectedType(null)
                      setFeedback('')
                    }}
                    className="text-xs"
                  >
                    Change
                  </Button>
                </div>

                <div className="space-y-2">
                  <Textarea
                    id="feedback-text"
                    placeholder="Describe the issue or provide additional context..."
                    value={feedback}
                    autoFocus
                    disabled={isLoading}
                    onChange={(e) => handleFeedbackChange(e.target.value)}
                    className="min-h-20 resize-none text-sm"
                  />
                </div>

                <div className="flex gap-2 pt-2">
                  <Button size="sm" onClick={handleSubmit} disabled={isLoading} className="flex-1">
                    {isLoading ? (
                      <>
                        <Loader2 className="h-4 w-4 animate-spin" />
                        Sending...
                      </>
                    ) : (
                      <>
                        Send Feedback <Send className="h-4 w-4" />
                      </>
                    )}
                  </Button>
                </div>
                <div className="text-xs text-muted-foreground">
                  You will send the <strong>full message</strong> to the developers.
                </div>
              </motion.div>
            )}
          </AnimatePresence>
        </motion.div>
      </PopoverContent>
    </Popover>
  )
}
