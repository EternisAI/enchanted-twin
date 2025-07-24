import { useState, useEffect, useRef, useLayoutEffect } from 'react'
import { Button } from '../ui/button'
import { ArrowUp, X } from 'lucide-react'
import { motion, AnimatePresence } from 'framer-motion'
import { cn } from '../../lib/utils'

import { EnableVoiceModeButton, ReasoningButton } from './ChatInputBox'
import useDependencyStatus from '@renderer/hooks/useDependencyStatus'
import { PrivacyButton } from './privacy/PrivacyButton'

type MessageInputProps = {
  onSend: (text: string, reasoning: boolean, voice: boolean) => void
  isWaitingTwinResponse: boolean
  onStop?: () => void
  isReasonSelected: boolean
  onReasonToggle?: (reasoningSelected: boolean) => void
  voiceMode?: boolean
  placeholder?: string
  onVoiceModeChange?: () => void
  isStreamingResponse?: boolean
}

export default function MessageInput({
  onSend,
  isWaitingTwinResponse,
  onStop,
  isReasonSelected,
  onReasonToggle,
  voiceMode = false,
  placeholder = 'Send a message privately…',
  isStreamingResponse,
  onVoiceModeChange
}: MessageInputProps) {
  const [text, setText] = useState('')

  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const { isVoiceReady } = useDependencyStatus()

  const handleSend = () => {
    if (!text.trim() || isWaitingTwinResponse || isStreamingResponse) return
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
        'relative z-50 flex flex-col gap-3 rounded-xl border border-border bg-card/90 backdrop-blur-md px-4 py-2.25 shadow-xl w-full'
      )}
      transition={{ type: 'spring', stiffness: 350, damping: 55 }}
      layout
      onClick={handleClickContainer}
    >
      <motion.div layout className="flex items-center gap-3 w-full">
        <motion.textarea
          layout="position"
          ref={textareaRef}
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={handleKeyDown}
          rows={1}
          autoFocus
          placeholder={placeholder}
          className="flex-1 placeholder:text-muted-foreground resize-none bg-transparent text-foreground outline-none !overflow-y-auto max-h-[12rem] "
        />
        <motion.div layout="position" className="flex justify-end items-center gap-3 h-fit">
          <PrivacyButton className="text-primary/50" label={false} />
          {!voiceMode && <ReasoningButton isSelected={isReasonSelected} onClick={toggleReason} />}
          <motion.div
            key="send-button"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="h-9"
          >
            <SendButton
              onSend={handleSend}
              isWaitingTwinResponse={isWaitingTwinResponse}
              isStreamingResponse={isStreamingResponse}
              onStop={onStop}
              text={text}
              onVoiceModeChange={onVoiceModeChange}
              isVoiceReady={isVoiceReady}
            />
          </motion.div>
        </motion.div>
      </motion.div>
    </motion.div>
  )
}

export function SendButton({
  onSend,
  onStop,
  isWaitingTwinResponse,
  isStreamingResponse = false,
  text,
  className,
  onVoiceModeChange,
  isVoiceReady
}: {
  isWaitingTwinResponse: boolean
  onSend: () => void
  onStop?: () => void
  isStreamingResponse?: boolean
  text: string
  className?: string
  onVoiceModeChange?: () => void
  isVoiceReady: boolean
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

  const isWaitingForAgent = isWaitingTwinResponse || isStreamingResponse

  return (
    <>
      {!isWaitingForAgent && !text.trim() ? (
        <EnableVoiceModeButton onClick={() => onVoiceModeChange?.()} isVoiceReady={isVoiceReady} />
      ) : (
        <Button
          size="icon"
          variant={isWaitingForAgent ? 'destructive' : 'default'}
          className={cn('rounded-full transition-all duration-200 ease-in-out relative', className)}
          onClick={isWaitingForAgent ? handleStop : onSend}
          disabled={isStreamingResponse}
        >
          <AnimatePresence mode="wait">
            {isWaitingForAgent ? (
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
                <ArrowUp className="w-4 h-4" />
              </motion.div>
            )}
          </AnimatePresence>
        </Button>
      )}
    </>
  )
}
