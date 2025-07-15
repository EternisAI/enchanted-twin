import { Message, Role } from '@renderer/graphql/generated/graphql'
import { AssistantMessageBubble, UserMessageBubble } from './messages/Message'
import { motion } from 'framer-motion'

type MessageListProps = {
  messages: Message[]
  isWaitingTwinResponse: boolean
  chatPrivacyDict: string | null
  isAnonymized?: boolean
}

export default function MessageList({
  messages,
  isWaitingTwinResponse,
  chatPrivacyDict,
  isAnonymized = false
}: MessageListProps) {
  return (
    <div className="flex flex-col gap-10 w-full">
      {messages.map((msg) =>
        msg.role === Role.User ? (
          <UserMessageBubble
            key={msg.id}
            message={msg}
            isAnonymized={isAnonymized}
            chatPrivacyDict={chatPrivacyDict}
          />
        ) : (
          <AssistantMessageBubble key={msg.id} message={msg} />
        )
      )}
      {isWaitingTwinResponse && (
        <motion.div
          className="text-sm text-muted-foreground italic px-3 py-1 bg-transparent rounded-md w-fit"
          initial="initial"
          animate="animate"
          variants={{
            initial: { opacity: 0, y: 20 },
            animate: { opacity: 1, y: 0 }
          }}
        >
          <div className="flex items-center justify-center gap-1 h-4">
            {[...Array(3)].map((_, i) => (
              <motion.div
                initial={{ opacity: 0, y: 20, scale: 0.9 }}
                animate={{
                  opacity: [0.5, 0.8, 0.5],
                  y: [2, -2, 2],
                  scale: [0.9, 1, 0.9]
                }}
                transition={{
                  y: {
                    duration: 1,
                    repeat: Infinity,
                    ease: 'easeInOut',
                    delay: i * 0.15
                  },
                  opacity: {
                    duration: 1,
                    repeat: Infinity,
                    ease: 'easeInOut',
                    delay: i * 0.15
                  },
                  scale: {
                    duration: 1,
                    repeat: Infinity,
                    ease: 'easeInOut',
                    delay: i * 0.15
                  }
                }}
                key={i}
                className="h-2 w-2 bg-accent-foreground rounded-full"
              />
            ))}
          </div>
        </motion.div>
      )}
    </div>
  )
}
