import { useSubscription } from '@apollo/client'
import { NotificationAddedDocument } from '@renderer/graphql/generated/graphql'
import { useEffect } from 'react'

export function useOsNotifications() {
  useSubscription(NotificationAddedDocument, {
    onData: ({ data }) => {
      const notification = data?.data?.notificationAdded
      if (notification) {
        console.log('notification', notification)
        window.api.notify(notification)
      }
    },
    onError: (error) => {
      console.error('Error subscribing to notifications:', error)
    }
  })

  useEffect(() => {
    window.api.onDeepLink((url) => {
      // handle deep link navigation
      window.location.hash = url // or `router.push(url)`
    })
  }, [])
}
