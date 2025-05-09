import { Chat } from '@renderer/graphql/generated/graphql'
import { ChatCard } from './ChatCard'
import { motion } from 'framer-motion'
import { Button } from '@renderer/components/ui/button'
import { X } from 'lucide-react'
import { isToday, isThisWeek, isThisMonth, isThisYear } from 'date-fns'

interface HistoryChatsProps {
  chats: Chat[]
  isActive: (path: string) => boolean
  onClose: () => void
}

const container = {
  hidden: { opacity: 0 },
  show: {
    opacity: 1,
    transition: {
      staggerChildren: 0.025
    }
  }
}

const item = {
  hidden: { opacity: 0 },
  show: { opacity: 1 }
}

const groupChatsByTime = (chats: Chat[]) => {
  const groups: { [key: string]: Chat[] } = {
    today: [],
    thisWeek: [],
    thisMonth: [],
    thisYear: [],
    older: []
  }

  chats.forEach((chat) => {
    const date = new Date(chat.createdAt)
    if (isToday(date)) {
      groups.today.push(chat)
    } else if (isThisWeek(date)) {
      groups.thisWeek.push(chat)
    } else if (isThisMonth(date)) {
      groups.thisMonth.push(chat)
    } else if (isThisYear(date)) {
      groups.thisYear.push(chat)
    } else {
      groups.older.push(chat)
    }
  })

  return groups
}

export function HistoryChats({ chats, isActive, onClose }: HistoryChatsProps) {
  const groupedChats = groupChatsByTime(chats)

  const renderGroup = (title: string, chats: Chat[]) => {
    if (chats.length === 0) return null

    return (
      <div className="w-full">
        <h3 className="text-sm font-medium text-muted-foreground mb-2">{title}</h3>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {chats.map((chat) => (
            <motion.div key={chat.id} variants={item}>
              <ChatCard chat={chat} isActive={isActive(`/chat/${chat.id}`)} hideTimestamp />
            </motion.div>
          ))}
        </div>
      </div>
    )
  }

  return (
    <div className="w-full flex flex-col items-center gap-4">
      <motion.div
        variants={container}
        initial="hidden"
        animate="show"
        exit="hidden"
        className="w-full flex flex-col gap-6"
      >
        {renderGroup('Today', groupedChats.today)}
        {renderGroup('This Week', groupedChats.thisWeek)}
        {renderGroup('This Month', groupedChats.thisMonth)}
        {renderGroup('This Year', groupedChats.thisYear)}
        {renderGroup('Older', groupedChats.older)}
      </motion.div>
      <Button
        onClick={onClose}
        variant="ghost"
        size="lg"
        className="text-muted-foreground hover:text-foreground"
      >
        <X className="w-4 h-4" />
        <span>Hide conversations</span>
      </Button>
    </div>
  )
}
