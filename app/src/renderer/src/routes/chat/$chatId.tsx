// routes/chat/$chatId.tsx
import ChatView from '@renderer/components/chat/ChatView'
import { client } from '@renderer/graphql/lib'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { GetChatDocument } from '@renderer/graphql/generated/graphql'
import { Button } from '@renderer/components/ui/button'
import { ArrowLeft } from 'lucide-react'
import { useEffect } from 'react'

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
  const navigate = useNavigate()

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        navigate({ to: '/' })
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [navigate])

  //   if (loading) return <div className="p-4">Loading chat...</div>
  if (!data) return <div className="p-4">Invalid chat ID.</div>
  if (error) return <div className="p-4 text-red-500">Error loading chat.</div>

  return (
    <div className="flex flex-col h-full w-full">
      <div className="p-2 fixed top-[28px] left-0 right-0 border-b border-border backdrop-blur-md bg-background/80 z-10">
        <div className="max-w-4xl mx-auto">
          <Button variant="ghost" onClick={() => navigate({ to: '/' })} className="gap-2">
            <ArrowLeft className="w-4 h-4" />
            Back to chats
          </Button>
        </div>
      </div>
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
