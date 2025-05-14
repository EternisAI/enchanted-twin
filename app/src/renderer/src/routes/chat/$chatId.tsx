// routes/chat/$chatId.tsx
import ChatView from '@renderer/components/chat/ChatView'
import { client } from '@renderer/graphql/lib'
import { createFileRoute } from '@tanstack/react-router'
import { GetChatDocument } from '@renderer/graphql/generated/graphql'

interface ChatSearchParams {
  initialMessage?: string
}

export const Route = createFileRoute('/chat/$chatId')({
  component: ChatRouteComponent,
  validateSearch: (search: Record<string, unknown>): ChatSearchParams => {
    return {
      initialMessage: typeof search.initialMessage === 'string' ? search.initialMessage : undefined
    }
  },
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
  const { initialMessage } = Route.useSearch()

  if (!data) return <div className="p-4">Invalid chat ID.</div>
  if (error) return <div className="p-4 text-red-500">Error loading chat.</div>

  return (
    <div className="flex flex-col h-full flex-1 w-full">
      <div className="flex-1 overflow-hidden w-full">
        <div className="flex flex-col items-center h-full w-full">
          <div className="w-full max-w-4xl mx-auto h-full">
            <ChatView key={data.id} chat={data} initialMessage={initialMessage} />
          </div>
        </div>
      </div>
    </div>
  )
}
