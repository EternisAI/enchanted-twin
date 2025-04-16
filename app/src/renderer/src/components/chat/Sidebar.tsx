import { Link, useRouterState } from '@tanstack/react-router'
import { Chat } from '@renderer/graphql/generated/graphql'
import { cn } from '@renderer/lib/utils'

export function Sidebar({ chats }: { chats: Chat[] }) {
  const { location } = useRouterState()

  return (
    <aside className="flex flex-col justify-between gap-3 w-64 bg-gray-50 border-r p-4 overflow-y-auto">
      <div className="flex flex-col gap-3">
        <h2 className="text-lg font-semibold mb-4">Chats</h2>
        <div className="space-y-2">
          {chats.map((chat: { id: string; name: string }) => {
            const isActive = location.pathname === `/chat/${chat.id}`
            return (
              <Link
                key={chat.id}
                disabled={isActive}
                to="/chat/$chatId"
                params={{ chatId: chat.id }}
                className={cn('block px-3 py-2 rounded-md text-sm font-medium', {
                  'bg-green-100 text-green-700': isActive,
                  'hover:bg-gray-100 text-gray-800': !isActive
                })}
              >
                {chat.name || 'Untitled Chat'}
              </Link>
            )
          })}
        </div>
      </div>

      <div>
        <Link to="/chat">
          <button className="w-full bg-green-500 text-white px-4 py-2 rounded-md">New Chat</button>
        </Link>
      </div>
    </aside>
  )
}
