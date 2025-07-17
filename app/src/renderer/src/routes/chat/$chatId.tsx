// routes/chat/$chatId.tsx
import ChatView from '@renderer/components/chat/ChatView'
import { ChatProvider } from '@renderer/contexts/ChatContext'
import { client } from '@renderer/graphql/lib'
import { createFileRoute } from '@tanstack/react-router'
import { GetChatDocument } from '@renderer/graphql/generated/graphql'
import { TypingIndicator } from '@renderer/components/chat/TypingIndicator'

interface ChatSearchParams {
  initialMessage?: string
  threadId?: string
  action?: string
  reasoning?: boolean
}

export const Route = createFileRoute('/chat/$chatId')({
  component: ChatRouteComponent,
  validateSearch: (search: Record<string, unknown>): ChatSearchParams => {
    return {
      initialMessage: typeof search.initialMessage === 'string' ? search.initialMessage : undefined,
      reasoning: search.reasoning === 'true' || search.reasoning === true
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
      <div className="flex flex-col items-center justify-center h-full w-full">
        <TypingIndicator />
      </div>
    )
  },
  pendingMs: 100,
  pendingMinMs: 300
})

function ChatRouteComponent() {
  const { data, error } = Route.useLoaderData()
  const { initialMessage, reasoning } = Route.useSearch()

  if (!data) return <div className="p-4">Invalid chat ID.</div>
  if (error) return <div className="p-4 text-red-500">Error loading chat.</div>

  return (
    <div className="flex flex-col h-full flex-1 w-full">
      <div className="flex-1 overflow-hidden w-full">
        <div className="flex flex-col items-center h-full w-full">
          <div className="w-full h-full">
            <ChatProvider
              key={data.id}
              chat={data}
              initialMessage={initialMessage}
              initialReasoningState={reasoning}
            >
              <ChatView key={data.id} chat={data} />
            </ChatProvider>
          </div>
        </div>
      </div>
    </div>
  )
}
