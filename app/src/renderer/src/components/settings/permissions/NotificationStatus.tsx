'use client'

import { useEffect, useState } from 'react'
import { Button } from '@renderer/components/ui/button'

export default function NotificationStatusCard() {
  const [osNotificationEnabled, setOsNotificationEnabled] = useState<boolean>(false)
  const [isMacOS, setIsMacOS] = useState<boolean>(false)

  useEffect(() => {
    setIsMacOS(/Mac/i.test(navigator.userAgent))

    const fetchStatus = async () => {
      try {
        const status = await window.api.getNotificationStatus()
        console.log('OS Status:', status)
        setOsNotificationEnabled(status === 'granted')
      } catch (error) {
        console.error('Failed to get OS status:', error)
        setOsNotificationEnabled(false)
      }
    }

    fetchStatus()
  }, [])

  const openOsSettings = () => {
    window.api.openSettings()
  }

  const isNotificationsEnabled = osNotificationEnabled ? 'Enabled' : 'Disabled'

  return (
    <div className="flex flex-col gap-1">
      <div className="grid grid-cols-3 items-center py-1">
        <span>Notifications</span>

        <div className="text-center">Protected</div>

        <div className="text-right">
          {isMacOS ? (
            <Button className="w-fit" size="sm" onClick={openOsSettings}>
              Open notification center
            </Button>
          ) : (
            <span className="text-muted-foreground">{isNotificationsEnabled}</span>
          )}
        </div>
      </div>

      <div className="flex pt-2">
        {isMacOS ? (
          <div className="mb-3 text-sm text-muted-foreground">
            <p>
              Please check your system preferences to ensure notifications are enabled for this
              application.
            </p>
          </div>
        ) : (
          <Button className="w-[210px]" onClick={openOsSettings}>
            Open Settings
          </Button>
        )}
      </div>
    </div>
  )
}
