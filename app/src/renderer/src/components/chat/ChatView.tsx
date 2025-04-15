import { useRef, useState } from 'react'
import { useEffect } from 'react'
import MessageList from './MessageList'
import MessageInput from './MessageInput'
import { Chat, Message } from '@renderer/graphql/generated/graphql'
import { useSendMessage } from '@renderer/hooks/useChat'
import { useMessageSubscription } from '@renderer/hooks/useMessageSubscription'

export default function ChatView({ chat }: { chat: Chat }) {
  const bottomRef = useRef<HTMLDivElement | null>(null)
  const [messages, setMessages] = useState<Message[]>(chat.messages)
  const [isWaitingTwinResponse, setIsWaitingTwinResponse] = useState(false)
  const { sendMessage } = useSendMessage(chat.id, (msg: Message) => {
    setMessages([...messages, msg])
    setIsWaitingTwinResponse(true)
  })

  useMessageSubscription(chat.id, (msg) => {
    setMessages((prev) => [...prev, msg])
    setIsWaitingTwinResponse(false)
  })

  console.log('chat id and data', chat.id, chat)

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [])

  return (
    <div className="flex flex-col items-center py-10 w-full h-full">
      <style>
        {`
          :root {
            --direction: -1;
          }
        `}
      </style>
      <div className="flex flex-col flex-1 max-w-2xl  w-full">
        <div className="flex-1 overflow-y-auto p-4">
          <MessageList messages={messages} isWaitingTwinResponse={isWaitingTwinResponse} />
          <div ref={bottomRef} />
        </div>
        <div className="p-4">
          <MessageInput onSend={sendMessage} isWaitingTwinResponse={isWaitingTwinResponse} />
        </div>
      </div>
    </div>
  )
}
