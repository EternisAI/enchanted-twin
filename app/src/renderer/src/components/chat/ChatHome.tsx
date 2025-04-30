// ChatHome.tsx
import { useNavigate, useRouter } from '@tanstack/react-router'
import MessageInput from './MessageInput'
import { useMutation, useQuery } from '@apollo/client'
import {
  CreateChatDocument,
  GetProfileDocument,
  SendMessageDocument
} from '@renderer/graphql/generated/graphql'
import { client } from '@renderer/graphql/lib'
import { ContextCard } from './ContextCard'

export default function ChatHome() {
  const navigate = useNavigate()
  const router = useRouter()
  const [createChat] = useMutation(CreateChatDocument)
  const [sendMessage] = useMutation(SendMessageDocument)
  const { data: profile } = useQuery(GetProfileDocument)

  const handleStartChat = async (text: string) => {
    try {
      const { data: createData } = await createChat({
        variables: { name: text }
      })
      const newChatId = createData?.createChat?.id

      if (newChatId) {
        navigate({
          to: `/chat/${newChatId}`,
          search: { initialMessage: text }
        })

        // Refetch all chats
        await client.cache.evict({ fieldName: 'getChats' })
        await router.invalidate({
          filter: (match) => match.routeId === '/chat'
        })

        await sendMessage({ variables: { chatId: newChatId, text } })
      }
    } catch (error) {
      console.error('Failed to start chat:', error)
    }
  }

  const twinName = profile?.profile?.name || 'Your Twin'

  return (
    <div className="flex flex-col items-center h-full w-full">
      <style>
        {`
          :root {
            --direction: -1;
          }
        `}
      </style>
      <div className="flex flex-col flex-1 w-full max-w-4xl justify-between">
        <div className="flex flex-col items-center overflow-y-auto scrollbar-thin scrollbar-thumb-gray-300 scrollbar-track-transparent gap-12">
          <div className="py-8 w-full flex flex-col items-center gap-4">
            <h1 className="text-3xl font-bold text-center">{twinName}</h1>
            <div className="w-full max-w-lg mx-auto">
              <ContextCard />
            </div>
          </div>

          <div className="flex gap-10 p-4 border border-border rounded-lg max-w-lg mx-auto w-full">
            <div className="flex flex-col gap-2">
              <span>Today&apos;s Highlight</span>
              <span className="text-muted-foreground text-sm">10 Messages</span>
            </div>
            <div className="flex flex-col gap-2">
              <span>Last 30 days</span>
              <span className="text-muted-foreground text-sm">10 Messages</span>
            </div>
            <div className="flex flex-col gap-2">
              <span>Twin Suggestions</span>
              <span className="text-muted-foreground text-sm">Go Walk</span>
            </div>
          </div>
        </div>
        <div className="px-6 h-[130px] w-full flex flex-col justify-end">
          <MessageInput isWaitingTwinResponse={false} onSend={handleStartChat} />
        </div>
      </div>
    </div>
  )
}
