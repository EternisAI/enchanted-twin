import { createFileRoute } from '@tanstack/react-router'
import { Sidebar } from '@renderer/components/chat/Sidebar'
import { Outlet } from '@tanstack/react-router'
import { mockChats } from '@renderer/components/chat/mock'

export const Route = createFileRoute('/chat')({
  loader: async () => {
    return {
      data: mockChats,
      loading: false,
      error: null
    }
    // const { data } = await client.query({
    //   query: GetChatsDocument,
    //   variables: { first: 20, offset: 0 }
    // })
    // return data
  },
  component: ChatLayout
})

function ChatLayout() {
  const { data } = Route.useLoaderData()
  console.log('getChats', data)

  return (
    <div className="flex h-screen">
      <Sidebar chats={data} />
      <main className="flex-1 overflow-hidden">
        <Outlet />
      </main>
    </div>
  )
}
