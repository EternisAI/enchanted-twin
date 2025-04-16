// routes/chat/$chatId.tsx
import ChatView from '@renderer/components/chat/ChatView'
import { mockChats } from '@renderer/components/chat/mock'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/chat/$chatId')({
  component: ChatRouteComponent,
  loader: async ({ params }) => {
    await new Promise((resolve) => setTimeout(resolve, 1500))
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
  },
  pendingComponent: () => {
    return (
      <div className="flex flex-col items-center justify-center h-full">
        <div className="flex items-center justify-center gap-2 h-20">
          {[...Array(3)].map((_, i) => (
            <div
              key={i}
              className="h-3 w-3 bg-green-500 rounded-full animate-bounce"
              style={{ animationDelay: `${i * 0.15}s` }}
            />
          ))}
        </div>
      </div>
    )
  },
  pendingMs: 100,
  pendingMinMs: 300
})

function ChatRouteComponent() {
  const { data } = Route.useLoaderData()

  //   if (loading) return <div className="p-4">Loading chat...</div>
  //   if (error) return <div className="p-4 text-red-500">Error loading chat.</div>
  if (!data) return <div className="p-4">Invalid chat ID.</div>

  return <ChatView key={data.id} chat={data} />
}
