import { useState, useCallback } from 'react'
import { Flag, X } from 'lucide-react'
import { motion, AnimatePresence } from 'framer-motion'
import { Button } from '@renderer/components/ui/button'
import { Textarea } from '@renderer/components/ui/textarea'
import { Popover, PopoverContent, PopoverTrigger } from '@renderer/components/ui/popover'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger
} from '@renderer/components/ui/tooltip'

type FeedbackType = 'not-enough-anonymization' | 'too-much-anonymized' | 'other' | null

export function FeedbackPopover() {
  const [isOpen, setIsOpen] = useState(false)
  const [selectedType, setSelectedType] = useState<FeedbackType>(null)
  const [feedback, setFeedback] = useState('')
  const [hasUnsavedChanges, setHasUnsavedChanges] = useState(false)

  const handleFeedbackChange = useCallback(
    (value: string) => {
      setFeedback(value)
      setHasUnsavedChanges(value.trim() !== '' || selectedType !== null)
    },
    [selectedType]
  )

  const handleTypeSelect = useCallback((type: FeedbackType) => {
    setSelectedType(type)
    setHasUnsavedChanges(true)
    setHasInitialized(true)
  }, [])

  const handleClose = useCallback(() => {
    if (hasUnsavedChanges) {
      const confirmed = window.confirm(
        'You have unsaved feedback. Are you sure you want to close without sending?'
      )
      if (!confirmed) return
    }

    setIsOpen(false)
    setSelectedType(null)
    setFeedback('')
    setHasUnsavedChanges(false)
    setHasInitialized(false)
  }, [hasUnsavedChanges])

  const handleSubmit = useCallback(() => {
    if (!selectedType) return

    // TODO: Implement feedback submission logic
    console.log('Feedback submitted:', { type: selectedType, feedback })

    // Reset state after submission
    setIsOpen(false)
    setSelectedType(null)
    setFeedback('')
    setHasUnsavedChanges(false)
    setHasInitialized(false)
  }, [selectedType, feedback])

  const handleOpenChange = useCallback(
    (open: boolean) => {
      if (!open) {
        handleClose()
      } else {
        setIsOpen(open)
      }
    },
    [handleClose]
  )

  const [hasInitialized, setHasInitialized] = useState(false)

  return (
    <Popover open={isOpen} onOpenChange={handleOpenChange}>
      <PopoverTrigger asChild>
        <button
          className="transition-opacity p-1 rounded-full bg-background/80 hover:bg-muted z-10"
          aria-label="Provide feedback on this message"
        >
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger asChild>
                <Flag className="h-4 w-4 text-muted-foreground" />
              </TooltipTrigger>
              <TooltipContent>Provide feedback on this message</TooltipContent>
            </Tooltip>
          </TooltipProvider>
        </button>
      </PopoverTrigger>
      <PopoverContent
        className="w-80 min-h-[250px] overflow-hidden"
        align="end"
        side="top"
        onInteractOutside={(e) => {
          if (hasUnsavedChanges) {
            e.preventDefault()
            handleClose()
          }
        }}
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
            <h4 className="font-semibold text-sm">What&apos;s wrong with this message?</h4>
            <Button
              variant="ghost"
              size="icon"
              onClick={handleClose}
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
                    onClick={() => {
                      setSelectedType(null)
                      setFeedback('')
                      setHasUnsavedChanges(false)
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
                    onChange={(e) => handleFeedbackChange(e.target.value)}
                    className="min-h-20 resize-none"
                  />
                </div>

                <div className="flex gap-2 pt-2">
                  <Button size="sm" onClick={handleSubmit} className="flex-1">
                    Send Feedback
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => {
                      setSelectedType(null)
                      setFeedback('')
                      setHasUnsavedChanges(false)
                    }}
                    className="flex-1"
                  >
                    Cancel
                  </Button>
                </div>
              </motion.div>
            )}
          </AnimatePresence>
        </motion.div>
      </PopoverContent>
    </Popover>
  )
}
