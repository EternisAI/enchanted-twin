// routes/chat/$chatId.tsx
import ChatView from '@renderer/components/chat/ChatView'
import { GetChatDocument } from '@renderer/graphql/generated/graphql'
import { client } from '@renderer/graphql/lib'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/chat/$chatId')({
  component: ChatRouteComponent,
  loader: async ({ params }) => {
    try {
      const { data } = await client.query({
        query: GetChatDocument,
        variables: { id: params.chatId },
        fetchPolicy: 'network-only'
      })
      return {
        data: data.getChat,
        loading: false,
        error: null
      }
    } catch (error: unknown) {
      return {
        data: null,
        loading: false,
        error: error instanceof Error ? error.message : 'An unknown error occurred'
      }
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
  const { data, error } = Route.useLoaderData()

  //   if (loading) return <div className="p-4">Loading chat...</div>
  if (!data) return <div className="p-4">Invalid chat ID.</div>
  if (error) return <div className="p-4 text-red-500">Error loading chat.</div>

  return <ChatView key={data.id} chat={data} />
}
