import { useNavigate } from '@tanstack/react-router'
import { Button } from '../ui/button'
import { ArrowLeft, Send } from 'lucide-react'
import { useState } from 'react'
import { formatDistanceToNow } from 'date-fns'
import { motion } from 'framer-motion'

interface HolonChatComponentProps {
  threadId: string
  action: string
}

export default function HolonChatComponent({ threadId, action }: HolonChatComponentProps) {
  const navigate = useNavigate()
  const [message, setMessage] = useState('')
  const [chatMessages, setChatMessages] = useState<
    Array<{ id: string; content: string; sender: 'user' | 'twin'; timestamp: string }>
  >([])

  // Mock thread data - this would come from your data source
  const threadData = {
    id: threadId,
    title: 'Hey Bay-Area poker twins!',
    content: "My twin and I are putting together a friendly $1/$2 No-Limit Hold'em cash game...",
    author: {
      alias: 'You',
      identity: 'user123'
    },
    createdAt: '2024-01-15T10:37:00Z'
  }

  const handleBack = () => {
    navigate({ to: '/holon/$threadId', params: { threadId } })
  }

  const handleSendMessage = () => {
    if (!message.trim()) return

    const newMessage = {
      id: Date.now().toString(),
      content: message,
      sender: 'user' as const,
      timestamp: new Date().toISOString()
    }

    setChatMessages([...chatMessages, newMessage])
    setMessage('')

    // Simulate twin response
    setTimeout(() => {
      const twinResponse = {
        id: (Date.now() + 1).toString(),
        content: `I'll help you with "${action}". Let me process that for you...`,
        sender: 'twin' as const,
        timestamp: new Date().toISOString()
      }
      setChatMessages((prev) => [...prev, twinResponse])
    }, 1000)
  }

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSendMessage()
    }
  }

  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{ duration: 0.3 }}
      className="flex flex-col h-full w-full max-w-4xl mx-auto"
    >
      {/* Header */}
      <div className="flex items-center gap-4 p-4 border-b border-border bg-background/80 backdrop-blur-sm sticky top-0 z-10">
        <Button variant="ghost" size="icon" onClick={handleBack}>
          <ArrowLeft className="w-4 h-4" />
        </Button>
        <div className="flex-1">
          <h1 className="text-lg font-semibold">Chat about: {action}</h1>
          <div className="text-sm text-muted-foreground">
            From &ldquo;{threadData.title}&rdquo; • {threadData.author.alias}
          </div>
        </div>
      </div>

      {/* Thread Context */}
      <div className="p-4 bg-muted/30 border-b border-border">
        <div className="text-sm text-muted-foreground mb-2">Thread Context:</div>
        <div className="bg-background rounded-lg p-3 border">
          <h3 className="font-medium text-sm mb-1">{threadData.title}</h3>
          <p className="text-xs text-muted-foreground line-clamp-2">{threadData.content}</p>
        </div>
      </div>

      {/* Chat Messages */}
      <div className="flex-1 overflow-y-auto p-4 space-y-4">
        {chatMessages.length === 0 ? (
          <div className="text-center text-muted-foreground py-8">
            <div className="text-lg font-medium mb-2">
              Ready to help with &ldquo;{action}&rdquo;
            </div>
            <div className="text-sm">
              Start a conversation with your digital twin about this action.
            </div>
          </div>
        ) : (
          chatMessages.map((msg) => (
            <div
              key={msg.id}
              className={`flex ${msg.sender === 'user' ? 'justify-end' : 'justify-start'}`}
            >
              <div
                className={`max-w-xs lg:max-w-md px-4 py-2 rounded-lg ${
                  msg.sender === 'user'
                    ? 'bg-primary text-primary-foreground'
                    : 'bg-muted border border-border'
                }`}
              >
                <div className="text-sm">{msg.content}</div>
                <div
                  className={`text-xs mt-1 ${
                    msg.sender === 'user' ? 'text-primary-foreground/70' : 'text-muted-foreground'
                  }`}
                >
                  {formatDistanceToNow(new Date(msg.timestamp), { addSuffix: true })}
                </div>
              </div>
            </div>
          ))
        )}
      </div>

      {/* Message Input */}
      <div className="sticky bottom-0 bg-background/80 backdrop-blur-sm border-t border-border p-4">
        <div className="flex gap-2">
          <textarea
            value={message}
            onChange={(e) => setMessage(e.target.value)}
            onKeyPress={handleKeyPress}
            placeholder={`Ask your twin about "${action}"...`}
            className="flex-1 resize-none rounded-lg border border-border px-3 py-2 text-sm bg-background focus:outline-none focus:ring-2 focus:ring-ring focus:border-transparent"
            rows={1}
            style={{
              minHeight: '40px',
              maxHeight: '120px',
              height: 'auto'
            }}
            onInput={(e) => {
              const target = e.target as HTMLTextAreaElement
              target.style.height = 'auto'
              target.style.height = `${target.scrollHeight}px`
            }}
          />
          <Button onClick={handleSendMessage} disabled={!message.trim()} size="icon">
            <Send className="w-4 h-4" />
          </Button>
        </div>
      </div>
    </motion.div>
  )
}
