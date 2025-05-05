import { Message, Role } from '@renderer/graphql/generated/graphql'
import { AssistantMessageBubble, UserMessageBubble } from './Message'
import { motion } from 'framer-motion'

type MessageListProps = {
  messages: Message[]
  isWaitingTwinResponse: boolean
}

export default function MessageList({ messages, isWaitingTwinResponse }: MessageListProps) {
  return (
    <div className="flex flex-col gap-10">
      {messages.map((msg) =>
        msg.role === Role.User ? (
          <UserMessageBubble key={msg.id} message={msg} />
        ) : (
          <AssistantMessageBubble key={msg.id} message={msg} />
        )
      )}
      {isWaitingTwinResponse && (
        <motion.div
          className="text-sm text-muted-foreground italic px-3 py-1 bg-accent rounded-md w-fit"
          initial="initial"
          animate="animate"
          variants={{
            initial: { opacity: 0, y: 20 },
            animate: { opacity: 1, y: 0 }
          }}
        >
          <div className="flex items-center justify-center gap-1 h-5">
            {[...Array(3)].map((_, i) => (
              <div
                key={i}
                className="h-2 w-2 bg-gray-500 rounded-full animate-bounce"
                style={{ animationDelay: `${i * 0.15}s` }}
              />
            ))}
          </div>
        </motion.div>
      )}
    </div>
  )
}
