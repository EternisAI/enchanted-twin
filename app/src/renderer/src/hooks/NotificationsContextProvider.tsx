import { createContext, useContext, ReactNode, useState, useEffect } from 'react'
import { useSubscription } from '@apollo/client'
import { AppNotification, NotificationAddedDocument } from '@renderer/graphql/generated/graphql'
import { useNavigate, useRouter } from '@tanstack/react-router'
import { client } from '@renderer/graphql/lib'

interface NotificationsContextType {
  notifications: AppNotification[]
}

const NotificationsContext = createContext<NotificationsContextType | undefined>(undefined)

function extractChatIdFromNotificationLink(link: string): string | null {
  // Backend Notification format: twin://chat/{chatId}
  const match = link.match(/twin:\/\/chat\/(.+)/)
  return match ? match[1] : null
}

export function NotificationsProvider({ children }: { children: ReactNode }) {
  const [notifications, setNotifications] = useState<AppNotification[]>([])
  const navigate = useNavigate()
  const router = useRouter()

  useSubscription(NotificationAddedDocument, {
    onData: ({ data }) => {
      const notification = data?.data?.notificationAdded
      if (notification) {
        window.api.notify(notification)
        setNotifications((prev) => [...prev, notification])

        if (notification.link) {
          const chatId = extractChatIdFromNotificationLink(notification.link)
          if (chatId) {
            client.cache.evict({
              fieldName: 'getChat',
              args: { id: chatId }
            })

            router.invalidate({
              filter: (match) => match.routeId === '/chat/$chatId' && match.params.chatId === chatId
            })

            console.log('Cache invalidated for chat:', chatId)
          }
        }
      }
    },
    onError: (error) => {
      console.error('Error subscribing to notifications:', error)
    }
  })

  useEffect(() => {
    window.api.onDeepLink((url) => {
      const chatId = extractChatIdFromNotificationLink(url)
      if (chatId) {
        navigate({ to: '/chat/$chatId', params: { chatId } })
      }
    })
  }, [navigate])

  return (
    <NotificationsContext.Provider value={{ notifications }}>
      {children}
    </NotificationsContext.Provider>
  )
}

export function useNotifications() {
  const context = useContext(NotificationsContext)
  if (context === undefined) {
    throw new Error('useNotifications must be used within a NotificationsProvider')
  }
  return context
}
