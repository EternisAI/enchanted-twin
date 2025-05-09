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
  AlertDialogCancel
} from '../ui/alert-dialog'
import { Button } from '../ui/button'
import { Plus, Trash2, ChevronLeft, ChevronRight } from 'lucide-react'
import { useMutation } from '@apollo/client'
import { client } from '@renderer/graphql/lib'
import { Omnibar } from '../Omnibar'
import { useOmnibarStore } from '@renderer/lib/stores/omnibar'
import { useState } from 'react'

export function Sidebar({ chats }: { chats: Chat[] }) {
  const { location } = useRouterState()
  const { openOmnibar } = useOmnibarStore()
  const [isCollapsed, setIsCollapsed] = useState(false)

  return (
    <>
      <aside
        className={cn(
          'flex flex-col bg-muted/50 p-4 rounded-lg h-full gap-4 transition-all duration-300',
          isCollapsed ? 'w-16' : 'w-64'
        )}
      >
        <div className="flex items-center justify-between mb-4">
          {!isCollapsed && <h2 className="text-4xl text-foreground">Chats</h2>}
          <Button
            variant="ghost"
            size="icon"
            onClick={() => setIsCollapsed(!isCollapsed)}
            className="ml-auto"
          >
            {isCollapsed ? (
              <ChevronRight className="h-4 w-4" />
            ) : (
              <ChevronLeft className="h-4 w-4" />
            )}
          </Button>
        </div>
        {!isCollapsed && (
          <Button variant="outline" className="w-full justify-between px-2" onClick={openOmnibar}>
            <div className="flex items-center gap-2">
              <Plus className="w-3 h-3" />
              <span>New chat</span>
            </div>
            <div className="flex items-center gap-2 text-[10px] text-muted-foreground">
              <kbd className="rounded bg-muted px-2 py-1">⌘ K</kbd>
            </div>
          </Button>
        )}
        <div className="flex-1 overflow-y-auto scrollbar-thin scrollbar-thumb-gray-300 scrollbar-track-transparent">
          {chats.map((chat: Chat) => {
            const isActive = location.pathname === `/chat/${chat.id}`
            return (
              <SidebarItem
                key={chat.id}
                chat={chat}
                isActive={isActive}
                isCollapsed={isCollapsed}
              />
            )
          })}
        </div>
      </aside>
      <Omnibar />
    </>
  )
}

function SidebarItem({
  chat,
  isActive,
  isCollapsed
}: {
  chat: Chat
  isActive: boolean
  isCollapsed: boolean
}) {
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
        filter: (match) => match.routeId === '/chat/$chatId'
      })

      if (isActive) {
        navigate({ to: '/' })
      }
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
        className={cn('block px-3 py-2 text-sm font-medium flex-1 truncate', {
          'text-primary': isActive,
          'text-muted-foreground': !isActive
        })}
      >
        {isCollapsed ? chat.name?.[0] || 'U' : chat.name || 'Untitled Chat'}
      </Link>
      {!isCollapsed && (
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
              <AlertDialogTitle>Delete chat</AlertDialogTitle>
              <AlertDialogDescription>
                This action cannot be undone. It will permanently delete the chat.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>Do not delete</AlertDialogCancel>
              <Button
                variant="destructive"
                onClick={() => {
                  deleteChat({ variables: { chatId: chat.id } })
                }}
              >
                Delete
              </Button>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      )}
    </div>
  )
}
