import { motion } from 'framer-motion'
import { cn } from '@renderer/lib/utils'
import { ExternalLink, Bell } from 'lucide-react'
import { AppNotification } from '@renderer/graphql/generated/graphql'
import { formatDistanceToNow } from 'date-fns'

interface ToolCallNotificationItemProps {
  notification: AppNotification
}

export default function ToolCallNotificationItem({ notification }: ToolCallNotificationItemProps) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      className={cn('flex gap-3 p-3 rounded-lg border border-gray-200 bg-gray-50/50')}
    >
      <div className="flex-shrink-0">
        <Bell className="h-5 w-5 text-gray-500" />
      </div>
      <div className="flex flex-col gap-2">
        <div className="flex flex-col items-start justify-between">
          <h4 className="text-sm font-medium">{notification.title}</h4>
          <span className="text-xs text-muted-foreground whitespace-nowrap">
            {formatDistanceToNow(new Date(notification.createdAt), { addSuffix: true })}
          </span>
        </div>
        <p className="text-sm text-muted-foreground mt-1">{notification.message}</p>
        {notification.image && (
          <img
            src={notification.image}
            alt={notification.title}
            className="mt-2 rounded-md w-full h-32 object-cover"
          />
        )}
        {notification.link && (
          <a
            href={notification.link}
            target="_blank"
            rel="noopener noreferrer"
            className="mt-2 inline-flex items-center gap-1 text-xs text-gray-500 hover:text-gray-600"
          >
            <ExternalLink className="h-3 w-3" />
            Open Link
          </a>
        )}
      </div>
    </motion.div>
  )
}
