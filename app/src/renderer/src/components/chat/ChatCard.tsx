import { Chat } from '@renderer/graphql/generated/graphql'
import { Link } from '@tanstack/react-router'
import { cn } from '@renderer/lib/utils'
import { Trash2 } from 'lucide-react'
import { Button } from '../ui/button'
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
import { useMutation } from '@apollo/client'
import { DeleteChatDocument, GetChatsDocument } from '@renderer/graphql/generated/graphql'
import { client } from '@renderer/graphql/lib'
import { useRouter } from '@tanstack/react-router'

interface ChatCardProps {
  chat: Chat
  isActive?: boolean
}

export function ChatCard({ chat, isActive }: ChatCardProps) {
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
    }
  })

  const lastMessage = chat.messages[chat.messages.length - 1]
  const lastMessageText = lastMessage?.text || 'No messages yet'
  const truncatedText =
    lastMessageText.length > 100 ? lastMessageText.substring(0, 100) + '...' : lastMessageText

  return (
    <Link
      to="/chat/$chatId"
      params={{ chatId: chat.id }}
      className={cn(
        'flex flex-col gap-2 p-4 rounded-lg border border-border hover:border-primary/50 transition-colors',
        isActive && 'border-primary bg-primary/5'
      )}
    >
      <div className="flex items-center justify-between">
        <span className="text-lg font-medium hover:text-primary transition-colors">
          {chat.name || 'Untitled Chat'}
        </span>
        <AlertDialog>
          <AlertDialogTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="opacity-0 group-hover:opacity-100 transition-opacity hover:bg-destructive/10"
              onClick={(e) => e.stopPropagation()}
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
                onClick={(e) => {
                  e.stopPropagation()
                  deleteChat({ variables: { chatId: chat.id } })
                }}
              >
                Delete
              </Button>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>
      <p className="text-sm text-muted-foreground line-clamp-2">{truncatedText}</p>
      <div className="text-xs text-muted-foreground">
        {lastMessage ? new Date(lastMessage.createdAt).toLocaleString() : 'No messages'}
      </div>
    </Link>
  )
}
