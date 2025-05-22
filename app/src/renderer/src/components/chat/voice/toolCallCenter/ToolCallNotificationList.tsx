import { motion, AnimatePresence } from 'framer-motion'
import { AppNotification } from '@renderer/graphql/generated/graphql'
import ToolCallNotificationItem from './ToolCallNotificationItem'

interface ToolCallNotificationListProps {
  notifications: AppNotification[]
}

export default function ToolCallNotificationList({ notifications }: ToolCallNotificationListProps) {
  const isOverlapping = notifications.length > 2
  const overlapSize = '-105px' // @TODO: make this dynamic based on the item size

  return (
    <div className="relative flex flex-col gap-4 w-full min-h-[140px]">
      <AnimatePresence>
        {notifications.map((notification, index) => {
          const shouldOverlap = isOverlapping && index > 0
          const marginTop = shouldOverlap ? overlapSize : undefined
          const zIndex = 10 + index // ensures top to bottom layering

          const isTopItem = index === notifications.length - 1
          let rotation = 0
          if (!isTopItem && isOverlapping) {
            rotation = index % 2 === 0 ? -5 : 5
          }

          return (
            <motion.div
              key={notification.id}
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0, rotate: rotation }}
              exit={{ opacity: 0, y: -20 }}
              transition={{ duration: 0.2, delay: index * 0.1 }}
              className="w-full"
              style={{
                marginTop,
                zIndex,
                position: 'relative'
              }}
            >
              <ToolCallNotificationItem notification={notification} />
            </motion.div>
          )
        })}
      </AnimatePresence>
    </div>
  )
}
