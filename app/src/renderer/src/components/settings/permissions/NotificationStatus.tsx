'use client'

import { useEffect, useState } from 'react'
import { Button } from '@renderer/components/ui/button'
import { Bell, CheckCircle2, XCircle, HelpCircle } from 'lucide-react'
import { cn } from '@renderer/lib/utils'
import { Card } from '@renderer/components/ui/card'

export default function NotificationStatusCard() {
  const [osNotificationEnabled, setOsNotificationEnabled] = useState<boolean>(false)
  const [isMacOS, setIsMacOS] = useState<boolean>(false)
  const [isLoading, setIsLoading] = useState<boolean>(true)

  useEffect(() => {
    setIsMacOS(/Mac/i.test(navigator.userAgent))

    const fetchStatus = async () => {
      try {
        setIsLoading(true)
        const status: boolean = await window.api.getNotificationStatus()
        setOsNotificationEnabled(status)
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

    if (isMacOS) {
      return {
        icon: osNotificationEnabled ? CheckCircle2 : XCircle,
        color: osNotificationEnabled ? 'text-green-500' : 'text-red-500',
        label: osNotificationEnabled ? 'Enabled' : 'Disabled'
      }
    }

    return {
      icon: osNotificationEnabled ? CheckCircle2 : XCircle,
      color: osNotificationEnabled ? 'text-green-500' : 'text-red-500',
      label: osNotificationEnabled ? 'Enabled' : 'Disabled'
    }
  }

  const statusInfo = getStatusConfig()
  const StatusIcon = statusInfo.icon

  return (
    <Card className="p-4 min-w-[200px] flex flex-col items-center gap-3">
      <div className="flex flex-col items-center gap-2">
        <Bell className="h-8 w-8 text-muted-foreground" />
        <span className="font-medium">Notifications</span>
      </div>

      <div className="flex items-center gap-2">
        <StatusIcon className={cn('h-5 w-5', statusInfo.color)} />
        <span className={cn('text-sm', statusInfo.color)}>{statusInfo.label}</span>
      </div>

      <Button variant="outline" className="w-full" size="sm" onClick={openOsSettings}>
        Settings
      </Button>
    </Card>
  )
}
