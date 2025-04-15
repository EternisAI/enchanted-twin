import { Message, Role } from '@renderer/graphql/generated/graphql'
import { AssistantMessageBubble, UserMessageBubble } from './Message'

export default function MessageList({ messages }: { messages: Message[] }) {
  return (
    <div className="flex flex-col gap-4">
      {messages.map((msg) =>
        msg.role === Role.User ? (
          <UserMessageBubble key={msg.id} message={msg} />
        ) : (
          <AssistantMessageBubble key={msg.id} message={msg} />
        )
      )}
    </div>
  )
}
