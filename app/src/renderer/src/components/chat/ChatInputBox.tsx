import React, { useEffect, useState } from 'react'
import { TooltipContent } from '../ui/tooltip'
import { TooltipTrigger } from '../ui/tooltip'
import { Tooltip } from '../ui/tooltip'
import { AnimatePresence, motion } from 'framer-motion'
import { Button } from '../ui/button'
import { AudioLinesIcon, Brain, CheckIcon, X } from 'lucide-react'
import { checkVoiceDisabled, cn } from '@renderer/lib/utils'
import { SendButton } from './MessageInput'
import { toast } from 'sonner'
import { Popover, PopoverContent, PopoverTrigger } from '../ui/popover'

type ChatInputBoxProps = {
  query: string
  isVoiceReady: boolean
  textareaRef: React.RefObject<HTMLTextAreaElement | null>
  onInputChange: (query: string) => void
  handleSubmit: (e: React.KeyboardEvent<HTMLTextAreaElement>) => void
  isReasonSelected: boolean
  setIsReasonSelected: (isReasonSelected: boolean) => void
  isVoiceMode: boolean
  onVoiceModeChange: (toggleSidebar?: boolean) => void
  handleCreateChat: () => void
  onLayoutAnimationComplete?: () => void
}

export default function ChatInputBox({
  query,
  textareaRef,
  isReasonSelected,
  isVoiceReady,
  isVoiceMode,
  onInputChange,
  handleSubmit,
  setIsReasonSelected,
  handleCreateChat,
  onVoiceModeChange,
  onLayoutAnimationComplete
}: ChatInputBoxProps) {
  const isVoiceDisabled = checkVoiceDisabled()

  // Auto-resize textarea fallback for browsers without field-sizing support
  useEffect(() => {
    if (!textareaRef.current) return

    const textarea = textareaRef.current

    // Check if field-sizing is supported
    const supportsFieldSizing = CSS.supports('field-sizing', 'content')

    if (!supportsFieldSizing) {
      // Manual auto-resize for browsers without field-sizing
      const adjustHeight = () => {
        textarea.style.height = 'auto'
        textarea.style.height = `${textarea.scrollHeight}px`
      }

      // Initial adjustment
      adjustHeight()

      // Adjust on input
      textarea.addEventListener('input', adjustHeight)

      return () => {
        textarea.removeEventListener('input', adjustHeight)
      }
    }
  }, [textareaRef])

  return (
    <>
      <style>{`
        .auto-sizing-textarea {
          field-sizing: content;
          min-height: 50px;
          max-height: 240px;
        }

        @supports not (field-sizing: content) {
          /* Fallback for browsers that don't support field-sizing */
          .auto-sizing-textarea {
            min-height: 50px;
            max-height: 240px;
          }
        }

        .auto-sizing-textarea::-webkit-scrollbar {
          width: 4px;
        }
        .auto-sizing-textarea::-webkit-scrollbar-track {
          background: transparent;
        }
        .auto-sizing-textarea::-webkit-scrollbar-thumb {
          background-color: rgba(155, 155, 155, 0.5);
          border-radius: 2px;
        }
        .auto-sizing-textarea::-webkit-scrollbar-thumb:hover {
          background-color: rgba(155, 155, 155, 0.7);
        }
      `}</style>
      <motion.div
        layoutId="message-input-container"
        transition={{
          layout: { type: 'spring', stiffness: 200, damping: 30 }
        }}
        onLayoutAnimationComplete={onLayoutAnimationComplete}
        className="relative overflow-hidden px-3 z-50 w-full flex items-center gap-2 border border-border rounded-lg dark:bg-background bg-white focus-within:shadow-xl dark:focus-within:border-primary/25 transition-shadow duration-200"
      >
        <motion.textarea
          layout="position"
          ref={textareaRef}
          value={query}
          onChange={(e) => onInputChange(e.target.value)}
          onKeyDown={(e: React.KeyboardEvent<HTMLTextAreaElement>) => {
            if (e.key === 'Enter' && !e.shiftKey) {
              e.preventDefault()
              handleSubmit(e)
            }
          }}
          placeholder="Send a message privately…"
          className="outline-none !bg-transparent flex-1 border-0 shadow-none focus-visible:ring-0 focus-visible:ring-offset-0 py-4 pl-2 pr-1 resize-none overflow-y-auto min-h-[50px] max-h-[240px] auto-sizing-textarea"
          rows={1}
        />

        <motion.div
          layout="position"
          className="flex items-center justify-end gap-2"
          transition={{ type: 'spring', stiffness: 300, damping: 30 }}
        >
          <AnimatePresence mode="popLayout" initial={false}>
            {!isVoiceMode && (
              <motion.div
                key="reasoning"
                layout="position"
                transition={{ type: 'spring', stiffness: 300, damping: 30 }}
              >
                <ReasoningButton
                  isSelected={isReasonSelected}
                  onClick={() => setIsReasonSelected(!isReasonSelected)}
                  disabled={isVoiceMode}
                />
              </motion.div>
            )}
            {!isVoiceMode && query.length === 0 && !isVoiceDisabled ? (
              <motion.div
                key="talk"
                layout="position"
                layoutId="action-button"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={{ type: 'spring', stiffness: 300, damping: 30 }}
                className="flex items-center justify-end w-fit h-9"
              >
                <EnableVoiceModeButton
                  onClick={() => {
                    onVoiceModeChange(false)
                  }}
                  isVoiceReady={isVoiceReady}
                />
              </motion.div>
            ) : query.length > 0 || isVoiceMode ? (
              <motion.div
                key="send"
                layout="position"
                layoutId="action-button"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={{ type: 'spring', stiffness: 300, damping: 30 }}
                className="flex items-center justify-end w-9 h-9"
              >
                <SendButton
                  className="w-9 h-9"
                  text={query}
                  onSend={() => {
                    handleCreateChat()
                  }}
                  isWaitingTwinResponse={false}
                  onVoiceModeChange={() => onVoiceModeChange(false)}
                  isVoiceReady={isVoiceReady}
                />
              </motion.div>
            ) : null}
          </AnimatePresence>
        </motion.div>
      </motion.div>
    </>
  )
}

interface ReasoningButtonProps {
  isSelected: boolean
  onClick: () => void
  disabled?: boolean
}

const REASONING_MODEL = 'gpt-5-think'
const NOT_REASONING_MODEL = 'gpt-5'

export function ReasoningButton({ isSelected, onClick, disabled }: ReasoningButtonProps) {
  const [open, setOpen] = useState(false)

  const models = [
    { value: false, label: 'Standard', model: NOT_REASONING_MODEL },
    { value: true, label: 'Advanced Reasoning', model: REASONING_MODEL }
  ]

  const handleModelSelect = (value: boolean) => {
    if (value !== isSelected) {
      onClick()
    }
    setOpen(false)
  }

  const brainVariants = {
    initial: { scale: 1 },
    animate: { scale: 1.1 },
    exit: { scale: 1 }
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          size="icon"
          className={cn('relative', isSelected && '!text-orange-500')}
          variant="outline"
          disabled={disabled}
        >
          <motion.div
            variants={brainVariants}
            animate={isSelected ? 'animate' : 'initial'}
            transition={{ type: 'spring', stiffness: 350, damping: isSelected ? 5 : 20 }}
          >
            <Brain className="w-4 h-4" />
          </motion.div>
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-56 p-1" align="end">
        <motion.div
          initial={{ opacity: 0, scale: 0.95 }}
          animate={{ opacity: 1, scale: 1 }}
          transition={{ duration: 0.15 }}
          className="flex flex-col"
        >
          <div className="px-2 py-1.5 text-sm font-semibold text-muted-foreground">
            Model Selection
          </div>
          <div className="h-px bg-border my-1" />
          {models.map((model, index) => (
            <motion.button
              key={model.model}
              initial={{ opacity: 0, x: -10 }}
              animate={{ opacity: 1, x: 0 }}
              transition={{ duration: 0.15, delay: index * 0.05 }}
              onClick={() => handleModelSelect(model.value)}
              className={cn(
                'flex items-center justify-between px-2 py-1.5 text-sm rounded-sm hover:bg-accent transition-colors outline-none',
                isSelected === model.value && 'bg-accent'
              )}
            >
              <div className="flex flex-col items-start">
                <span className="font-medium">{model.label}</span>
                <span className="text-xs text-muted-foreground">{model.model}</span>
              </div>
              <AnimatePresence mode="wait">
                {isSelected === model.value && (
                  <motion.div
                    initial={{ scale: 0 }}
                    animate={{ scale: 1 }}
                    exit={{ scale: 0 }}
                    transition={{ duration: 0.15 }}
                  >
                    <CheckIcon className="w-4 h-4 text-orange-500" />
                  </motion.div>
                )}
              </AnimatePresence>
            </motion.button>
          ))}
        </motion.div>
      </PopoverContent>
    </Popover>
  )
}

interface VoiceModeButtonProps {
  isVoiceReady: boolean
  onClick: () => void
}

export function EnableVoiceModeButton({ onClick, isVoiceReady }: VoiceModeButtonProps) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          variant="default"
          onClick={() => {
            if (isVoiceReady) {
              onClick()
            } else {
              toast.error('Voice dependencies installation in progress')
            }
          }}
          size="icon"
        >
          <motion.span
            key="off"
            layout="position"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.4, ease: 'easeOut' }}
          >
            <AudioLinesIcon className="w-4 h-4" />
          </motion.span>
        </Button>
      </TooltipTrigger>
      <TooltipContent>
        <p>{isVoiceReady ? 'Start voice chat' : 'Preparing voice...'}</p>
      </TooltipContent>
    </Tooltip>
  )
}

export function DisableVoiceModeButton({ onClick }: VoiceModeButtonProps) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          onClick={onClick}
          variant="outline"
          className={cn(
            '!px-4.5 relative rounded-full transition-all shadow-none hover:shadow-lg active:shadow-sm border-none'
          )}
          // variant={isVoiceMode ? 'outline' : 'default'}
        >
          <AnimatePresence mode="wait" initial={false}>
            <motion.div
              key="stop"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              transition={{ duration: 0.2, ease: 'easeOut' }}
              className="absolute inset-0 flex items-center justify-center"
            >
              <X className="w-5 h-5" />
            </motion.div>
          </AnimatePresence>
        </Button>
      </TooltipTrigger>
      <TooltipContent>
        <p>Stop voice mode</p>
      </TooltipContent>
    </Tooltip>
  )
}
