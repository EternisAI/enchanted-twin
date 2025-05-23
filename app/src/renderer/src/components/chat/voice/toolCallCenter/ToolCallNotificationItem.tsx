import { motion } from 'framer-motion'
import { cn } from '@renderer/lib/utils'
import { Bell } from 'lucide-react'
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
      className={cn(
        'flex flex-col gap-8 p-4 rounded-lg border border-gray-200 bg-background shadow-sm'
      )}
    >
      <div className="flex-shrink-0">
        <Bell className="h-6 w-6 text-gray-400" />
      </div>
      <div className="flex flex-col gap-6">
        <div className="flex flex-col items-start justify-between">
          <span className="text-xs text-muted-foreground whitespace-nowrap">
            {formatDistanceToNow(new Date(notification.createdAt), { addSuffix: true })}
          </span>
          <h4 className="text-md font-medium">{notification.title}</h4>
        </div>
        {/* <p className="text-sm text-muted-foreground mt-1 max-h-[60px] overflow-y-auto">
          {notification.message}
        </p> */}
        {/* {notification.image && (
          <img
            src={notification.image}
            alt={notification.title}
            className="mt-2 rounded-md w-full h-32 object-cover"
          />
        )} */}
      </div>
    </motion.div>
  )
}
