import { useState, useEffect } from 'react'
import { Button } from '../ui/button'
import { ArrowBigUp, X } from 'lucide-react'
import { Textarea } from '../ui/textarea'
import { motion, AnimatePresence } from 'framer-motion'
import { cn } from '../../lib/utils'

type MessageInputProps = {
  onSend: (text: string) => void
  isWaitingTwinResponse: boolean
  onStop?: () => void
}

export default function MessageInput({ onSend, isWaitingTwinResponse, onStop }: MessageInputProps) {
  const [text, setText] = useState('')

  const handleSend = () => {
    if (!text.trim() || isWaitingTwinResponse) return
    onSend(text)
    setText('')
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  return (
    <motion.div
      layoutId="message-input-container"
      className={cn(
        'flex flex-col gap-3 rounded-t-lg border border-border bg-background/90 p-4 shadow-xl w-full'
      )}
      transition={{
        layout: { type: 'spring', damping: 25, stiffness: 280 }
      }}
    >
      <div className="flex items-center gap-3 w-full">
        <Textarea
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={handleKeyDown}
          rows={3}
          autoFocus
          placeholder="Type a message..."
          className="flex-1 resize-none bg-transparent text-foreground placeholder-muted-foreground outline-none"
        />
        <SendButton
          onSend={handleSend}
          isWaitingTwinResponse={isWaitingTwinResponse}
          onStop={onStop}
          text={text}
        />
      </div>
    </motion.div>
  )
}

function SendButton({
  onSend,
  onStop,
  isWaitingTwinResponse,
  text
}: {
  isWaitingTwinResponse: boolean
  onSend: () => void
  onStop?: () => void
  text: string
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
      className="rounded-full transition-all duration-200 ease-in-out relative"
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
            <ArrowBigUp className="w-4 h-4" />
          </motion.div>
        )}
      </AnimatePresence>
    </Button>
  )
}
