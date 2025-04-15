import { Message, Role } from '@renderer/graphql/generated/graphql'
import { AssistantMessageBubble, UserMessageBubble } from './Message'

type MessageListProps = {
  messages: Message[]
  isWaitingTwinResponse: boolean
}

export default function MessageList({ messages, isWaitingTwinResponse }: MessageListProps) {
  return (
    <div className="flex flex-col gap-4">
      {messages.map((msg) =>
        msg.role === Role.User ? (
          <UserMessageBubble key={msg.id} message={msg} />
        ) : (
          <AssistantMessageBubble key={msg.id} message={msg} />
        )
      )}
      {isWaitingTwinResponse && (
        <div className="text-sm text-gray-500 italic px-3 py-1 bg-gray-100 rounded-md w-fit">
          Your twin is thinking...
        </div>
      )}
    </div>
  )
}
