// routes/chat/$chatId.tsx
import ChatView from '@renderer/components/chat/ChatView'
import { mockChats } from '@renderer/components/chat/mock'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/chat/$chatId')({
  component: ChatRouteComponent,
  loader: async ({ params }) => {
    const chat = mockChats.find((chat) => chat.id == params.chatId)
    console.log('params!!', params, chat)

    if (!chat) {
      return {
        data: null,
        loading: false,
        error: 'Chat not found'
      }
    }

    return {
      data: mockChats.find((chat) => chat.id == params.chatId),
      loading: false,
      error: null
    }
  }
})

function ChatRouteComponent() {
  const { data } = Route.useLoaderData()

  //   if (loading) return <div className="p-4">Loading chat...</div>
  //   if (error) return <div className="p-4 text-red-500">Error loading chat.</div>
  if (!data) return <div className="p-4">Invalid chat ID.</div>

  return <ChatView chat={data} />
}
