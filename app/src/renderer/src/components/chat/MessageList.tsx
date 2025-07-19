import { Message, Role } from '@renderer/graphql/generated/graphql'
import { AssistantMessageBubble, UserMessageBubble } from './messages/Message'
import { TypingIndicator } from './TypingIndicator'

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
    <div className="flex flex-col gap-5 w-full pb-20">
      {messages.map((msg) =>
        msg.role === Role.User ? (
          <UserMessageBubble
            key={msg.id}
            message={msg}
            showTimestamp={false}
            isAnonymized={isAnonymized}
            chatPrivacyDict={chatPrivacyDict}
          />
        ) : (
          <AssistantMessageBubble
            key={msg.id}
            message={msg}
            showTimestamp={false}
            isAnonymized={isAnonymized}
            chatPrivacyDict={chatPrivacyDict}
          />
        )
      )}
      {isWaitingTwinResponse && <TypingIndicator />}
    </div>
  )
}
