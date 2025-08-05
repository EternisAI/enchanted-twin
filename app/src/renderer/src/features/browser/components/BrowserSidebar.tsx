import { useState, useEffect } from 'react'
import { useBrowserStore, selectActiveSession } from '../stores/browserStore'
import { useQuery, useMutation } from '@apollo/client'
import {
  CreateChatDocument,
  SendMessageDocument,
  ChatCategory
} from '@renderer/graphql/generated/graphql'
import { Button } from '@renderer/components/ui/button'
import { Textarea } from '@renderer/components/ui/textarea'
import { Card } from '@renderer/components/ui/card'
import { Globe, Send, AlertCircle, FileText, Image } from 'lucide-react'
import { cn } from '@renderer/lib/utils'
import { ScrollArea } from '@renderer/components/ui/scroll-area'

interface BrowserSidebarProps {
  sessionId: string
  className?: string
}

export function BrowserSidebar({ sessionId, className }: BrowserSidebarProps) {
  const activeSession = useBrowserStore(selectActiveSession)
  const [chatId, setChatId] = useState<string | null>(null)
  const [message, setMessage] = useState('')
  const [messages, setMessages] = useState<Array<{ role: string; text: string }>>([])

  const [createChat] = useMutation(CreateChatDocument)
  const [sendMessage, { loading: sendingMessage }] = useMutation(SendMessageDocument)

  // Create a chat session when component mounts
  useEffect(() => {
    const initChat = async () => {
      try {
        const result = await createChat({
          variables: {
            name: `Browser: ${activeSession?.title || 'New Session'}`,
            category: ChatCategory.Text
          }
        })
        setChatId(result.data?.createChat.id || null)
      } catch (error) {
        console.error('Failed to create chat:', error)
      }
    }

    if (!chatId && activeSession) {
      initChat()
    }
  }, [chatId, activeSession, createChat])

  const handleSendMessage = async () => {
    if (!message.trim() || !chatId || sendingMessage) return

    const userMessage = message.trim()
    setMessage('')

    // Add user message to local state
    setMessages((prev) => [...prev, { role: 'user', text: userMessage }])

    try {
      // Include browser context in the message
      const contextMessage = `[Browser Context]
URL: ${activeSession?.url}
Title: ${activeSession?.title}
Content Preview: ${activeSession?.content.text.slice(0, 500)}...

User: ${userMessage}`

      const result = await sendMessage({
        variables: {
          chatId,
          text: contextMessage,
          reasoning: false,
          voice: false
        }
      })

      if (result.data?.sendMessage) {
        const responseText = result.data.sendMessage.text || ''
        setMessages((prev) => [
          ...prev,
          {
            role: 'assistant',
            text: responseText
          }
        ])
      }
    } catch (error) {
      console.error('Failed to send message:', error)
      setMessages((prev) => [
        ...prev,
        {
          role: 'error',
          text: 'Failed to send message. Please try again.'
        }
      ])
    }
  }

  if (!activeSession) {
    return null
  }

  return (
    <div className={cn('flex flex-col h-full bg-background', className)}>
      {/* Header */}
      <div className="p-4 border-b">
        <div className="flex items-center gap-2">
          <Globe className="w-5 h-5 text-muted-foreground" />
          <div className="flex-1 min-w-0">
            <h3 className="font-medium truncate">{activeSession.title || 'Browser Session'}</h3>
            <p className="text-xs text-muted-foreground truncate">{activeSession.url}</p>
          </div>
        </div>
      </div>

      {/* Content Info */}
      <Card className="mx-4 mt-4 p-3">
        <div className="space-y-2 text-sm">
          <div className="flex items-center gap-2">
            <FileText className="w-4 h-4 text-muted-foreground" />
            <span className="text-muted-foreground">Content extracted</span>
          </div>
          {activeSession.content.screenshot && (
            <div className="flex items-center gap-2">
              <Image className="w-4 h-4 text-muted-foreground" />
              <span className="text-muted-foreground">Screenshot available</span>
            </div>
          )}
          <div className="text-xs text-muted-foreground">
            {activeSession.content.text.length} characters
          </div>
        </div>
      </Card>

      {/* Chat Messages */}
      <ScrollArea className="flex-1 p-4">
        <div className="space-y-4">
          {messages.map((msg, idx) => (
            <div
              key={idx}
              className={cn(
                'p-3 rounded-lg',
                msg.role === 'user' && 'bg-primary/10 ml-8',
                msg.role === 'assistant' && 'bg-muted mr-8',
                msg.role === 'error' && 'bg-red-50 dark:bg-red-950 text-red-600 dark:text-red-400'
              )}
            >
              <div className="text-xs font-medium mb-1 opacity-60">
                {msg.role === 'user' ? 'You' : msg.role === 'assistant' ? 'Assistant' : 'Error'}
              </div>
              <div className="text-sm whitespace-pre-wrap">{msg.text}</div>
            </div>
          ))}
        </div>
      </ScrollArea>

      {/* Input */}
      <div className="p-4 border-t">
        <div className="flex gap-2">
          <Textarea
            value={message}
            onChange={(e) => setMessage(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault()
                handleSendMessage()
              }
            }}
            placeholder="Ask about the current page..."
            className="min-h-[60px] resize-none"
            disabled={!chatId || sendingMessage}
          />
          <Button
            onClick={handleSendMessage}
            disabled={!message.trim() || !chatId || sendingMessage}
            size="icon"
            className="self-end"
          >
            <Send className="w-4 h-4" />
          </Button>
        </div>
        {!chatId && (
          <div className="mt-2 flex items-center gap-2 text-xs text-orange-600 dark:text-orange-400">
            <AlertCircle className="w-3 h-3" />
            Initializing chat session...
          </div>
        )}
      </div>
    </div>
  )
}
