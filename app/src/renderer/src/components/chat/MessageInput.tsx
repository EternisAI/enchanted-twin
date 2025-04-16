import { useState } from 'react'
import { Button } from '../ui/button'

type MessageInputProps = {
  onSend: (text: string) => void
  isWaitingTwinResponse: boolean
}

export default function MessageInput({ onSend, isWaitingTwinResponse }: MessageInputProps) {
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
    <div className="flex gap-3 items-center">
      <textarea
        value={text}
        onChange={(e) => setText(e.target.value)}
        onKeyDown={handleKeyDown}
        rows={3}
        placeholder="Type a message..."
        className="flex-1 resize-none border rounded-md p-2 text-sm"
      />
      <SendButton onClick={handleSend} isWaitingTwinResponse={isWaitingTwinResponse} />
    </div>
  )
}

function SendButton({
  onClick,
  isWaitingTwinResponse
}: {
  onClick: () => void
  isWaitingTwinResponse: boolean
}) {
  return (
    <Button
      onClick={onClick}
      className="cursor-pointer bg-green-600 text-white px-4 py-2 h-10 rounded-md hover:bg-green-700"
      disabled={isWaitingTwinResponse}
    >
      Send
    </Button>
  )
}
