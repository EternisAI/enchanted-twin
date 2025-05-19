import { Mic } from 'lucide-react'
import { useNavigate, useRouter } from '@tanstack/react-router'
import { useMutation } from '@apollo/client'
import { CreateChatDocument } from '@renderer/graphql/generated/graphql'
import { client } from '@renderer/graphql/lib'
import MessageInput from '../chat/MessageInput'

export default function VoiceChatHome() {
  const [createChat] = useMutation(CreateChatDocument)
  const router = useRouter()
  const navigate = useNavigate()

  const handleStartVoiceChat = async (initialMessage: string) => {
    try {
      const { data: createData } = await createChat({
        variables: { name: initialMessage }
      })
      const newChatId = createData?.createChat?.id
      if (newChatId) {
        navigate({ to: `/voice/${newChatId}`, search: { initialMessage } })
        await client.cache.evict({ fieldName: 'getChats' })
        await router.invalidate()
      }
    } catch (error) {
      console.error('Failed to create chat:', error)
    }
  }

  return (
    <div className="flex flex-col items-center justify-center h-full gap-6 p-8">
      <div className="flex flex-col items-center gap-4 text-center">
        <div className="p-4 rounded-full bg-primary/10">
          <Mic className="w-12 h-12 text-primary" />
        </div>
        <h1 className="text-3xl font-bold">Voice Chat</h1>
        <p className="text-muted-foreground max-w-md">
          Start a new voice chat session to interact with your AI assistant via voice commands.
        </p>
      </div>
      <MessageInput
        isWaitingTwinResponse={false}
        onSend={handleStartVoiceChat}
        onStop={() => {}}
        hasReasoning={false}
      />
    </div>
  )
}
