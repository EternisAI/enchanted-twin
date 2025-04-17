import { createFileRoute } from '@tanstack/react-router'
import { Sidebar } from '@renderer/components/chat/Sidebar'
import { Outlet } from '@tanstack/react-router'
import { client } from '@renderer/graphql/lib'
import { Chat, GetChatsDocument } from '@renderer/graphql/generated/graphql'

export const Route = createFileRoute('/chat')({
  loader: async () => {
    const { data, loading, error } = await client.query({
      query: GetChatsDocument,
      variables: { first: 20, offset: 0 }
      // fetchPolicy: 'network-only'
    })
    console.log('loader called', data, loading, error)
    return { data, loading, error }
  },

  component: ChatLayout
})

function ChatLayout() {
  const { data } = Route.useLoaderData()

  const chats: Chat[] = data?.getChats || []

  return (
    <div className="flex flex-1 h-full">
      <Sidebar chats={chats} />
      <main className="flex-1 overflow-hidden">
        <Outlet />
      </main>
    </div>
  )
}
