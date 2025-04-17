// ChatHome.tsx
import { useNavigate, useRouter } from '@tanstack/react-router'
import MessageInput from './MessageInput'
import { useMutation } from '@apollo/client'
import {
  CreateChatDocument,
  // GetProfileDocument,
  SendMessageDocument
} from '@renderer/graphql/generated/graphql'
import { client } from '@renderer/graphql/lib'

export default function ChatHome() {
  const navigate = useNavigate()
  const router = useRouter()
  const [createChat] = useMutation(CreateChatDocument)
  const [sendMessage] = useMutation(SendMessageDocument)
  // const { data } = useQuery(GetProfileDocument)

  const handleStartChat = async (text: string) => {
    try {
      const { data: createData } = await createChat({
        variables: { name: text }
      })
      const newChatId = createData?.createChat?.id

      if (newChatId) {
        await sendMessage({ variables: { chatId: newChatId, text } })
        navigate({ to: `/chat/${newChatId}` })
        // Refetch all chats
        await client.cache.evict({ fieldName: 'getChats' })
        await router.invalidate({
          filter: (match) => match.routeId === '/chat'
        })
      }
    } catch (error) {
      console.error('Failed to start chat:', error)
    }
  }

  // const twinName = data?.profile.name || 'Your Twin'

  return (
    <div
      className="flex flex-col items-center w-full "
      style={{ viewTransitionName: 'page-content' }}
    >
      <style>
        {`
          :root {
            --direction: -1;
          }
        `}
      </style>
      <div className="flex flex-col flex-1 justify-between">
        <div
          className="p-6 flex flex-col items-center overflow-y-auto scrollbar-thin scrollbar-thumb-gray-300 scrollbar-track-transparent gap-3"
          style={{ maxHeight: `calc(100vh - 130px)` }}
        >
          <div className="py-8">
            <div className="w-48 h-48 rounded-full bg-gray-200 flex items-center justify-center">
              <span className="text-gray-400 text-6xl">👤</span>
            </div>
          </div>

          <div className="flex gap-10 p-4 border border-border rounded-lg">
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
        <div className="px-6 py-6 border-t border-border h-[130px]">
          <MessageInput isWaitingTwinResponse={false} onSend={handleStartChat} />
        </div>
      </div>
    </div>
  )
}
