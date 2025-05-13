import { motion } from 'framer-motion'
import { Chat } from '@renderer/graphql/generated/graphql'
import { useNavigate } from '@tanstack/react-router'
import { History, X } from 'lucide-react'
import { Button } from '../ui/button'
import { ScrollArea } from '../ui/scroll-area'

interface HistoryChatsProps {
  chats: Chat[]
  isActive: (path: string) => boolean
  onClose: () => void
  onShowSuggestions: (show: boolean) => void
}

export function HistoryChats({ chats, isActive, onClose, onShowSuggestions }: HistoryChatsProps) {
  const navigate = useNavigate()

  const handleClose = () => {
    onShowSuggestions(true)
    onClose()
  }

  return (
    <motion.div
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, y: 10 }}
      className="w-full"
    >
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <History className="w-4 h-4" />
          <h2 className="text-lg font-semibold">Chat History</h2>
        </div>
        <Button variant="ghost" size="icon" onClick={handleClose}>
          <X className="w-4 h-4" />
        </Button>
      </div>
      <ScrollArea className="h-[400px]">
        <div className="space-y-2">
          {chats.map((chat) => (
            <motion.button
              key={chat.id}
              initial={{ opacity: 0, y: 4 }}
              animate={{ opacity: 1, y: 0 }}
              onClick={() => {
                navigate({ to: `/chat/${chat.id}` })
                handleClose()
              }}
              className={`w-full text-left p-3 rounded-lg transition-colors ${
                isActive(`/chat/${chat.id}`) ? 'bg-primary/10 text-primary' : 'hover:bg-muted/80'
              }`}
            >
              {chat.name}
            </motion.button>
          ))}
        </div>
      </ScrollArea>
    </motion.div>
  )
}
