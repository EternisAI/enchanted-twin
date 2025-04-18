import { Link, useNavigate, useRouter, useRouterState } from '@tanstack/react-router'
import { Chat, DeleteChatDocument, GetChatsDocument } from '@renderer/graphql/generated/graphql'
import { cn } from '@renderer/lib/utils'
import {
  AlertDialog,
  AlertDialogTrigger,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogCancel,
  AlertDialogAction
} from '../ui/alert-dialog'
import { Button } from '../ui/button'
import { Trash2 } from 'lucide-react'
import { useMutation } from '@apollo/client'
import { client } from '@renderer/graphql/lib'

export function Sidebar({ chats }: { chats: Chat[] }) {
  const { location } = useRouterState()

  const isHome = location.pathname === '/chat'

  return (
    <aside className="flex flex-col justify-between gap-3 w-64 bg-gray-50 border-r p-4 overflow-y-auto">
      <div className="flex flex-col gap-3">
        <h2 className="text-lg font-semibold mb-4">Chats</h2>
        <div className="space-y-2">
          {chats.map((chat: Chat) => {
            const isActive = location.pathname === `/chat/${chat.id}`
            return <SidebarItem key={chat.id} chat={chat} isActive={isActive} />
          })}
        </div>
      </div>

      <div>
        <Link to="/chat" disabled={isHome}>
          <button className="w-full bg-green-500 text-white px-4 py-2 rounded-md">New Chat</button>
        </Link>
      </div>
    </aside>
  )
}

function SidebarItem({ chat, isActive }: { chat: Chat; isActive: boolean }) {
  const navigate = useNavigate()
  const router = useRouter()
  const [deleteChat] = useMutation(DeleteChatDocument, {
    refetchQueries: [GetChatsDocument],
    onError: (error) => {
      console.error(error)
    },
    onCompleted: async () => {
      await client.cache.evict({ fieldName: 'getChats' })
      await router.invalidate({
        filter: (match) => match.routeId === '/chat'
      })

      if (isActive) {
        navigate({ to: '/chat' })
      }
    }
  })

  return (
    <div
      className={cn('flex items-center gap-2 justify-between rounded-md group', {
        'bg-green-100': isActive,
        'hover:bg-gray-100': !isActive
      })}
    >
      <Link
        key={chat.id}
        disabled={isActive}
        to="/chat/$chatId"
        params={{ chatId: chat.id }}
        className={cn('block px-3 py-2 text-sm font-medium flex-1', {
          'text-green-700': isActive,
          'text-gray-800': !isActive
        })}
      >
        {chat.name.slice(0, 25) || 'Untitled Chat'}
      </Link>
      <AlertDialog>
        <AlertDialogTrigger asChild>
          <Button
            variant="ghost"
            size="icon"
            className="ml-2 opacity-0 group-hover:opacity-100 transition-opacity hover:bg-red-100"
          >
            <Trash2 className="w-4 h-4 text-red-500" />
          </Button>
        </AlertDialogTrigger>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete chat</AlertDialogTitle>
            <AlertDialogDescription>
              This action cannot be undone. It will permanently delete the chat.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                deleteChat({ variables: { chatId: chat.id } })
              }}
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
