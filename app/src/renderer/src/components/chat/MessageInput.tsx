import { useState, useEffect, useRef, useLayoutEffect } from 'react'
import { Button } from '../ui/button'
import { ArrowUp, Lightbulb, X } from 'lucide-react'
import { motion, AnimatePresence } from 'framer-motion'
import { cn } from '../../lib/utils'
import { Tooltip, TooltipContent } from '../ui/tooltip'
import { TooltipTrigger } from '@radix-ui/react-tooltip'

type MessageInputProps = {
  onSend: (text: string, reasoning: boolean, voice: boolean) => void
  isWaitingTwinResponse: boolean
  onStop?: () => void
  isReasonSelected: boolean
  onReasonToggle?: (reasoningSelected: boolean) => void
  voiceMode?: boolean
}

export default function MessageInput({
  onSend,
  isWaitingTwinResponse,
  onStop,
  isReasonSelected,
  onReasonToggle,
  voiceMode = false
}: MessageInputProps) {
  const [text, setText] = useState('')

  const textareaRef = useRef<HTMLTextAreaElement>(null)

  const handleSend = () => {
    if (!text.trim() || isWaitingTwinResponse) return
    onSend(text, isReasonSelected, voiceMode)
    setText('')
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      if (isWaitingTwinResponse) {
        onStop?.()
      } else {
        handleSend()
      }
    }
  }

  useLayoutEffect(() => {
    const textarea = textareaRef.current
    if (textarea) {
      textarea.style.height = 'auto'
      textarea.style.height = `${textarea.scrollHeight}px`
    }
  }, [text])

  const handleClickContainer = (e: React.MouseEvent<HTMLDivElement>) => {
    if (textareaRef.current) {
      const target = e.target as Element
      // Focus if the click is not on the textarea itself or within a button element
      if (target !== textareaRef.current && !target.closest('button')) {
        textareaRef.current.focus()
      }
    }
  }

  const toggleReason = () => {
    onReasonToggle?.(!isReasonSelected)
  }

  return (
    <motion.div
      layoutId="message-input-container"
      className={cn(
        'flex flex-col gap-3 rounded-xl border border-border bg-card px-4 py-2.25 shadow-xl w-full'
      )}
      transition={{ type: 'spring', stiffness: 300, damping: 30 }}
      onClick={handleClickContainer}
    >
      <div className="flex items-center gap-3 w-full">
        <motion.textarea
          ref={textareaRef}
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={handleKeyDown}
          rows={1}
          autoFocus
          placeholder="What are you thinking?"
          className="flex-1 text-base placeholder:text-muted-foreground resize-none bg-transparent text-foreground outline-none overflow-y-auto max-h-[12rem]"
        />
        <div className="flex justify-end items-center gap-3">
          {!voiceMode && (
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  onClick={toggleReason}
                  className={cn(
                    'rounded-full transition-all shadow-none hover:shadow-lg !px-2.25 active:shadow-sm',
                    isReasonSelected
                      ? 'text-orange-500 !bg-orange-100/50 dark:!bg-orange-300/20 ring-orange-200 border-orange-200'
                      : ''
                  )}
                  variant="outline"
                >
                  <Lightbulb className="w-4 h-5" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                <p>Reasoning</p>
              </TooltipContent>
            </Tooltip>
          )}
          <SendButton
            onSend={handleSend}
            isWaitingTwinResponse={isWaitingTwinResponse}
            onStop={onStop}
            text={text}
          />
        </div>
      </div>
    </motion.div>
  )
}

export function SendButton({
  onSend,
  onStop,
  isWaitingTwinResponse,
  text,
  className
}: {
  isWaitingTwinResponse: boolean
  onSend: () => void
  onStop?: () => void
  text: string
  className?: string
}) {
  const [prevWaitingState, setPrevWaitingState] = useState(false)

  useEffect(() => {
    if (!isWaitingTwinResponse && prevWaitingState) {
      setPrevWaitingState(isWaitingTwinResponse)
    }
  }, [isWaitingTwinResponse, prevWaitingState])

  const handleStop = () => {
    onStop?.()
  }

  return (
    <Button
      size="icon"
      variant={isWaitingTwinResponse ? 'destructive' : 'default'}
      className={cn('rounded-full transition-all duration-200 ease-in-out relative', className)}
      onClick={isWaitingTwinResponse ? handleStop : onSend}
      disabled={!isWaitingTwinResponse && !text.trim()}
    >
      <AnimatePresence mode="wait">
        {isWaitingTwinResponse ? (
          <motion.div
            key="stop"
            initial={{ scale: 0, opacity: 0 }}
            animate={{ scale: 1, opacity: 1 }}
            exit={{ scale: 0, opacity: 0 }}
            transition={{ duration: 0.2, ease: 'easeOut' }}
            className="absolute inset-0 flex items-center justify-center"
          >
            <X className="w-4 h-4" />
          </motion.div>
        ) : (
          <motion.div
            key="send"
            initial={{ y: 20, opacity: 0 }}
            animate={{ y: 0, opacity: 1 }}
            exit={{ y: -20, opacity: 0 }}
            transition={{ duration: 0.4, ease: 'easeOut' }}
            className="absolute inset-0 flex items-center justify-center"
          >
            <ArrowUp
              className="w-4 h-4"
              fill={!isWaitingTwinResponse && !!text.trim() ? 'currentColor' : 'none'}
            />
          </motion.div>
        )}
      </AnimatePresence>
    </Button>
  )
}
