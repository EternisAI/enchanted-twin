import { createContext, useContext, ReactNode, useState, useEffect } from 'react'
import { useSubscription } from '@apollo/client'
import { AppNotification, NotificationAddedDocument } from '@renderer/graphql/generated/graphql'
import { useNavigate } from '@tanstack/react-router'

interface NotificationsContextType {
  notifications: AppNotification[]
}

const NotificationsContext = createContext<NotificationsContextType | undefined>(undefined)

export function NotificationsProvider({ children }: { children: ReactNode }) {
  const [notifications, setNotifications] = useState<AppNotification[]>([])
  const navigate = useNavigate()

  useSubscription(NotificationAddedDocument, {
    onData: ({ data }) => {
      const notification = data?.data?.notificationAdded
      if (notification) {
        window.api.notify(notification)
        setNotifications((prev) => [...prev, notification])
      }
    },
    onError: (error) => {
      console.error('Error subscribing to notifications:', error)
    }
  })

  useEffect(() => {
    console.log('setting up deep link listener')
    window.api.onDeepLink((url) => {
      // Backend Notification format: twin://chat/{chatId}
      const match = url.match(/twin:\/\/chat\/(.+)/)
      if (match && match[1]) {
        const chatId = match[1]
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
