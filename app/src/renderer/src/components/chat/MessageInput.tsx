import { useState } from 'react'

export default function MessageInput() {
  const [text, setText] = useState('')

  const handleSend = () => {
    if (!text.trim()) return
    // TODO: send message mutation
    console.log('Sending:', text)
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
      <button
        onClick={handleSend}
        className="bg-green-600 text-white px-4 py-2 h-10 rounded-md hover:bg-green-700"
      >
        Send
      </button>
    </div>
  )
}
