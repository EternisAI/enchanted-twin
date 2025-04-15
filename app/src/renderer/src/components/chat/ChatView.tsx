import { useRef } from 'react'
import { useEffect } from 'react'
import MessageList from './MessageList'
import MessageInput from './MessageInput'
import { Chat } from '@renderer/graphql/generated/graphql'

export default function ChatView({ chat }: { chat: Chat }) {
  const bottomRef = useRef<HTMLDivElement | null>(null)

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
          <MessageList messages={chat.messages} />
          <div ref={bottomRef} />
        </div>
        <div className="p-4">
          <MessageInput />
        </div>
      </div>
    </div>
  )
}
