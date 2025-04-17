import { useRef, useEffect, useState } from 'react'
import MessageList from './MessageList'
import MessageInput from './MessageInput'
import { Chat, Message, Role } from '@renderer/graphql/generated/graphql'
import { useSendMessage } from '@renderer/hooks/useChat'
import { useMessageSubscription } from '@renderer/hooks/useMessageSubscription'

const INPUT_HEIGHT = '130px'

export default function ChatView({ chat }: { chat: Chat }) {
  const bottomRef = useRef<HTMLDivElement | null>(null)
  const [messages, setMessages] = useState<Message[]>(chat.messages)
  const [isWaitingTwinResponse, setIsWaitingTwinResponse] = useState(false)

  //@TODO: This will be used to append OR replace a message if same id
  const appendMessage = (msg: Message) => {
    setMessages((prev) => {
      return [...prev, msg]
    })
  }

  const { sendMessage } = useSendMessage(chat.id, (msg) => {
    appendMessage(msg)
    setIsWaitingTwinResponse(true)
  })

  useMessageSubscription(chat.id, (msg) => {
    console.log('message subscription', msg)
    if (msg.role === Role.User) {
      return
    }
    appendMessage(msg)
    setIsWaitingTwinResponse(false)
  })

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  return (
    <div className="flex flex-col flex-1 min-h-full w-full justify-between">
      <div
        className="p-6 flex flex-col items-center overflow-y-auto scrollbar-thin scrollbar-thumb-gray-300 scrollbar-track-transparent"
        style={{ maxHeight: `calc(100vh - ${INPUT_HEIGHT})` }}
      >
        <div
          className="flex flex-col max-w-3xl w-full"
          style={{
            viewTransitionName: 'page-content'
          }}
        >
          <MessageList messages={messages} isWaitingTwinResponse={isWaitingTwinResponse} />
          <div ref={bottomRef} />
        </div>
      </div>
      <div
        className="px-6 py-6 border-t border-gray-200"
        style={{ height: INPUT_HEIGHT } as React.CSSProperties}
      >
        <MessageInput
          isWaitingTwinResponse={isWaitingTwinResponse}
          onSend={sendMessage}
          onStop={() => {
            setIsWaitingTwinResponse(false)
          }}
        />
      </div>
    </div>
  )
}
