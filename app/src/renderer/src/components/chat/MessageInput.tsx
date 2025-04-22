import { useState } from 'react'
import { Button } from '../ui/button'
import { StopCircleIcon } from 'lucide-react'

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
    <div className="flex gap-3 items-center flex-1">
      <textarea
        value={text}
        onChange={(e) => setText(e.target.value)}
        onKeyDown={handleKeyDown}
        rows={3}
        placeholder="Type a message..."
        className="flex-1 resize-none border rounded-md p-2 text-sm"
      />
      <SendButton
        onSend={handleSend}
        isWaitingTwinResponse={isWaitingTwinResponse}
        onStop={onStop}
      />
    </div>
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
