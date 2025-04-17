import { Link, useRouterState } from '@tanstack/react-router'
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

export function Sidebar({ chats }: { chats: Chat[] }) {
  const { location } = useRouterState()

  const isHome = location.pathname === '/chat' // TODO: refactor, this is a hack to check if we're on the home page

  return (
    <aside className="flex flex-col justify-between gap-3 w-64 bg-muted/50 p-4 overflow-y-auto rounded-lg">
      <div className="flex flex-col gap-3">
        <h2 className="text-lg font-semibold mb-4 text-foreground">Chats</h2>
        <div className="space-y-2">
          {chats.map((chat: Chat) => {
            const isActive = location.pathname === `/chat/${chat.id}`
            return <SidebarItem key={chat.id} chat={chat} isActive={isActive} />
          })}
        </div>
      </div>

      <div>
        <Link to="/chat" disabled={isHome}>
          <Button className="w-full">New Chat</Button>
        </Link>
      </div>
    </aside>
  )
}

function SidebarItem({ chat, isActive }: { chat: Chat; isActive: boolean }) {
  const [deleteChat] = useMutation(DeleteChatDocument, {
    refetchQueries: [GetChatsDocument],
    onError: (error) => {
      console.error(error)
    },
    onCompleted: () => {
      console.log('Chat deleted')
    }
  })

  return (
    <div
      className={cn('flex items-center gap-2 justify-between rounded-md group', {
        'bg-primary/10': isActive,
        'hover:bg-accent': !isActive
      })}
    >
      <Link
        key={chat.id}
        disabled={isActive}
        to="/chat/$chatId"
        params={{ chatId: chat.id }}
        className={cn('block px-3 py-2 text-sm font-medium flex-1', {
          'text-primary': isActive,
          'text-muted-foreground': !isActive
        })}
      >
        {chat.name.slice(0, 25) || 'Untitled Chat'}
      </Link>
      <AlertDialog>
        <AlertDialogTrigger asChild>
          <Button
            variant="ghost"
            size="icon"
            className="ml-2 opacity-0 group-hover:opacity-100 transition-opacity hover:bg-destructive/10"
          >
            <Trash2 className="w-4 h-4 text-destructive" />
          </Button>
        </AlertDialogTrigger>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete chat?</AlertDialogTitle>
            <AlertDialogDescription>
              This action cannot be undone. It will permanently delete the chat.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Do not delete</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                deleteChat({ variables: { chatId: chat.id } })
              }}
              asChild
            >
              <Button variant="destructive">Delete (Not implemented yet)</Button>
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
