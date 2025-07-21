import React, { useEffect } from 'react'
import { TooltipContent } from '../ui/tooltip'
import { TooltipTrigger } from '../ui/tooltip'
import { Tooltip } from '../ui/tooltip'
import { AnimatePresence, motion } from 'framer-motion'
import { Button } from '../ui/button'
import { AudioLinesIcon, Lightbulb, X } from 'lucide-react'
import { cn } from '@renderer/lib/utils'
import { SendButton } from './MessageInput'
import { toast } from 'sonner'

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
          layout
          ref={textareaRef}
          value={query}
          onChange={(e) => onInputChange(e.target.value)}
          onKeyDown={(e: React.KeyboardEvent<HTMLTextAreaElement>) => {
            if (e.key === 'Enter' && !e.shiftKey) {
              e.preventDefault()
              handleSubmit(e)
            }
          }}
          placeholder="Ask a question privately…"
          className="outline-none !bg-transparent flex-1 border-0 shadow-none focus-visible:ring-0 focus-visible:ring-offset-0 py-4 pl-2 pr-1 resize-none overflow-y-auto min-h-[50px] max-h-[240px] auto-sizing-textarea"
          rows={1}
        />

        <motion.div
          layout
          className="flex items-center justify-end gap-2"
          transition={{ type: 'spring' as const, stiffness: 300, damping: 30, origin: 'end' as const }}
        >
          <AnimatePresence mode="popLayout" initial={false}>
            {!isVoiceMode && (
              <motion.div
                key="reasoning"
                layout
                transition={{ type: 'spring' as const, stiffness: 300, damping: 30, origin: 'end' as const }}
              >
                <ReasoningButton
                  isSelected={isReasonSelected}
                  onClick={() => setIsReasonSelected(!isReasonSelected)}
                  disabled={isVoiceMode}
                />
              </motion.div>
            )}
            {!isVoiceMode && query.length === 0 ? (
              <motion.div
                key="talk"
                layoutId="action-button"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={{ type: 'spring' as const, stiffness: 300, damping: 30, origin: 'end' as const }}
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
                layoutId="action-button"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={{ type: 'spring' as const, stiffness: 300, damping: 30, origin: 'end' as const }}
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

export function ReasoningButton({ isSelected, onClick, disabled }: ReasoningButtonProps) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          size="icon"
          onClick={onClick}
          className={cn(isSelected && '!text-orange-500')}
          variant="outline"
          disabled={disabled}
        >
          <Lightbulb className="w-4 h-4" />
        </Button>
      </TooltipTrigger>
      <TooltipContent>
        <p>Reasoning</p>
      </TooltipContent>
    </Tooltip>
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
          variant="ghost"
          onClick={() => {
            if (isVoiceReady) {
              onClick()
            } else {
              toast.error('Voice dependencies installation in progress')
            }
          }}
          size="icon"
        >
          <AnimatePresence mode="wait" initial={false}>
            <motion.span
              key="off"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              transition={{ duration: 0.2 }}
            >
              <AudioLinesIcon className="w-4 h-4" />
            </motion.span>
          </AnimatePresence>
        </Button>
      </TooltipTrigger>
      <TooltipContent>
        <p>{isVoiceReady ? 'Start voice conversation' : 'Preparing voice...'}</p>
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
