'use client'

import { useOsNotifications } from '@renderer/hooks/useNotifications'
import { useEffect, useState } from 'react'

export default function NotificationGate() {
  const [status, setStatus] = useState<'default' | 'granted' | 'denied'>('default')
  useOsNotifications()

  useEffect(() => {
    if ('Notification' in window) {
      setStatus(Notification.permission as 'default' | 'granted' | 'denied')
      if (Notification.permission === 'default') {
        Notification.requestPermission().then((p) =>
          setStatus(p as 'default' | 'granted' | 'denied')
        )
      }
    }
  }, [])

  if (status === 'denied') {
    return (
      <p className="p-4 text-sm text-red-500">
        You disabled notifications for this app. Enable them in your OS settings to receive
        background alerts.
      </p>
    )
  }

  return null
}
