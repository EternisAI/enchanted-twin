// ChatHome.tsx
import { useNavigate, useRouter } from '@tanstack/react-router'
import MessageInput from './MessageInput'
import { useMutation } from '@apollo/client'
import { CreateChatDocument, SendMessageDocument } from '@renderer/graphql/generated/graphql'
import { client } from '@renderer/graphql/lib'

export default function ChatHome() {
  const navigate = useNavigate()
  const router = useRouter()
  const [createChat] = useMutation(CreateChatDocument)
  const [sendMessage] = useMutation(SendMessageDocument)

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

  return (
    <div
      className="flex flex-col items-center  w-full h-full"
      style={{ viewTransitionName: 'page-content' }}
    >
      <style>
        {`
          :root {
            --direction: -1;
          }
        `}
      </style>
      <div className="flex flex-col flex-1 min-h-full w-full justify-between">
        <div
          className="p-6 flex flex-col items-center overflow-y-auto scrollbar-thin scrollbar-thumb-gray-300 scrollbar-track-transparent"
          style={{ maxHeight: `calc(100vh - 130px)` }}
        >
          <h1 className="text-2xl font-bold text-black">Home</h1>
          <h5 className="text-gray-500 text-md">Send a message to start a conversation</h5>
        </div>
        <div
          className="px-6 py-6 border-t border-gray-200"
          style={{ height: '130px' } as React.CSSProperties}
        >
          <MessageInput isWaitingTwinResponse={false} onSend={handleStartChat} />
        </div>
      </div>
    </div>
  )
}
