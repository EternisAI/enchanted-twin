import { createFileRoute } from '@tanstack/react-router'
import { Sidebar } from '@renderer/components/chat/Sidebar'
import { Outlet } from '@tanstack/react-router'
import { client } from '@renderer/graphql/lib'
import { Chat, GetChatsDocument } from '@renderer/graphql/generated/graphql'

export const Route = createFileRoute('/chat')({
  loader: async () => {
    try {
      const { data, loading, error } = await client.query({
        query: GetChatsDocument,
        variables: { first: 20, offset: 0 }
      })
      console.log('loader called', data, loading, error)
      return { data, loading, error, success: true }
    } catch (error) {
      console.error('Error loading chats:', error)
      return {
        data: null,
        loading: false,
        error: error instanceof Error ? error : new Error('An unexpected error occurred'),
        success: false
      }
    }
  },

  component: ChatLayout
})

function ChatLayout() {
  const { data, error, success } = Route.useLoaderData()

  const chats: Chat[] = data?.getChats || []

  return (
    <div className="flex flex-1 h-full">
      <Sidebar chats={chats} />
      <main className="flex-1 overflow-hidden">
        {!success && (
          <div className="w-full flex justify-center items-center py-10">
            <div className="p-4 m-4 w-xl border border-red-300 bg-red-50 text-red-700 rounded-md">
              <h3 className="font-medium">Error loading chats</h3>
              <p className="text-sm">
                {error instanceof Error ? error.message : 'An unexpected error occurred'}
              </p>
            </div>
          </div>
        )}
        <Outlet />
      </main>
    </div>
  )
}
