import { useSubscription } from '@apollo/client'
import { NotificationAddedDocument } from '@renderer/graphql/generated/graphql'
import { useEffect } from 'react'
import { useNavigate } from '@tanstack/react-router'

export function useOsNotifications() {
  const navigate = useNavigate()

  useSubscription(NotificationAddedDocument, {
    onData: ({ data }) => {
      const notification = data?.data?.notificationAdded
      if (notification) {
        window.api.notify(notification)
      }
    },
    onError: (error) => {
      console.error('Error subscribing to notifications:', error)
    }
  })

  useEffect(() => {
    window.api.onDeepLink((url) => {
      // Backend Notification format: twin://chat/{chatId}
      const match = url.match(/twin:\/\/chat\/(.+)/)
      if (match && match[1]) {
        const chatId = match[1]
        navigate({ to: '/chat/$chatId', params: { chatId } })
      }
    })
  }, [navigate])
}
