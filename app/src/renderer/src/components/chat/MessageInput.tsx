import { useState } from 'react'
import { Button } from '../ui/button'
import { StopCircleIcon } from 'lucide-react'
import { Textarea } from '../ui/textarea'
import { motion } from 'framer-motion'

type MessageInputProps = {
  onSend: (text: string) => void
  isWaitingTwinResponse: boolean
  onStop?: () => void
}

export default function MessageInput({ onSend, isWaitingTwinResponse, onStop }: MessageInputProps) {
  const [text, setText] = useState('')

  const handleSend = () => {
    if (!text.trim()) return
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
      className="rounded-t-lg border border-border border-b-0 relative bottom-[1px] p-4 w-full"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      transition={{
        layout: { type: 'spring', damping: 25, stiffness: 200, mass: 0.8 },
        opacity: { duration: 0.2 }
      }}
    >
      <div className="flex gap-3 items-center flex-1">
        <Textarea
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={handleKeyDown}
          rows={3}
          autoFocus
          placeholder="Type a message..."
          className="flex-1 resize-none"
        />
        <SendButton
          onSend={handleSend}
          isWaitingTwinResponse={isWaitingTwinResponse}
          onStop={onStop}
        />
      </div>
    </motion.div>
  )
}

function SendButton({
  onSend,
  onStop,
  isWaitingTwinResponse
}: {
  isWaitingTwinResponse: boolean
  onSend: () => void
  onStop?: () => void
}) {
  return isWaitingTwinResponse ? (
    <Button onClick={onStop && onStop}>
      <StopCircleIcon className="w-4 h-4" />
    </Button>
  ) : (
    <Button onClick={onSend}>Send</Button>
  )
}
