'use client'

import { useEffect, useState } from 'react'
import { Bell, CheckCircle2, XCircle, HelpCircle } from 'lucide-react'
import { DetailCard } from './DetailCard'

export default function NotificationStatusCard() {
  const [osNotificationEnabled, setOsNotificationEnabled] = useState<boolean>(false)
  const [isLoading, setIsLoading] = useState<boolean>(true)

  useEffect(() => {
    const fetchStatus = async () => {
      try {
        setIsLoading(true)
        const status = await window.api.getNotificationStatus()
        setOsNotificationEnabled(status === 'granted')
      } catch (error) {
        console.error('Failed to get OS status:', error)
        setOsNotificationEnabled(false)
      } finally {
        setIsLoading(false)
      }
    }

    fetchStatus()
  }, [])

  const openOsSettings = () => {
    window.api.openSettings()
  }

  const getStatusConfig = () => {
    if (isLoading) {
      return {
        icon: HelpCircle,
        color: 'text-muted-foreground',
        label: 'Loading'
      }
    }

    return {
      icon: osNotificationEnabled ? CheckCircle2 : XCircle,
      color: osNotificationEnabled ? 'text-green-500' : 'text-red-500',
      label: osNotificationEnabled ? 'Enabled' : 'Disabled'
    }
  }

  const statusInfo = getStatusConfig()

  return (
    <DetailCard
      title="Notifications"
      IconComponent={Bell}
      statusInfo={statusInfo}
      buttonLabel="Settings"
      onButtonClick={openOsSettings}
      isLoading={isLoading}
    />
  )
}
