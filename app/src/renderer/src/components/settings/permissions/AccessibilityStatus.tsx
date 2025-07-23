'use client'
import { useEffect, useState } from 'react'
import { DetailCard } from './DetailCard'
import { CheckCircle2, XCircle, AlertCircle, HelpCircle, PersonStanding } from 'lucide-react'

type AccessibilityStatusType = 'granted' | 'denied' | 'unavailable' | 'error' | 'loading'

export default function AccessibilityStatus() {
  const [status, setStatus] = useState<AccessibilityStatusType>('loading')
  const [isLoading, setIsLoading] = useState<boolean>(true)

  const checkPermission = async () => {
    try {
      setIsLoading(true)
      const accessibilityStatus = await window.api.accessibility.getStatus()
      setStatus(accessibilityStatus as AccessibilityStatusType)
    } catch (error) {
      console.error('Error checking accessibility permission:', error)
      setStatus('error')
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    checkPermission()
    const interval = setInterval(checkPermission, 5000)
    return () => clearInterval(interval)
  }, [])

  const requestPermission = async () => {
    await window.api.accessibility.request()
    // TODO: make this smarter, so we detect changes in the accessibility settings
    await new Promise((resolve) => setTimeout(resolve, 500))
    checkPermission()

    window.api.analytics.capture('permission_asked', {
      name: 'accessibility'
    })
  }

  const openSettings = async () => {
    // Open accessibility-specific settings pane
    window.api.accessibility.openSettings()
  }

  const getStatusConfig = () => {
    switch (status) {
      case 'loading':
        return {
          icon: HelpCircle,
          color: 'text-muted-foreground',
          label: 'Loading'
        }
      case 'granted':
        return {
          icon: CheckCircle2,
          color: 'text-green-500 dark:text-green-400',
          label: 'Granted'
        }
      case 'denied':
        return {
          icon: XCircle,
          color: 'text-neutral-500 dark:text-neutral-400',
          label: 'Denied'
        }
      case 'unavailable':
        return {
          icon: AlertCircle,
          color: 'text-yellow-500 dark:text-yellow-400',
          label: 'Unavailable'
        }
      case 'error':
      default:
        return {
          icon: XCircle,
          color: 'text-neutral-500 dark:text-neutral-400',
          label: 'Error'
        }
    }
  }

  const statusInfo = getStatusConfig()
  const buttonLabel = status === 'denied' || status === 'error' ? 'Request' : 'Settings'
  const handleButtonClick =
    status === 'denied' || status === 'error' ? requestPermission : openSettings

  return (
    <DetailCard
      title="Accessibility"
      IconComponent={PersonStanding}
      statusInfo={statusInfo}
      buttonLabel={buttonLabel}
      onButtonClick={handleButtonClick}
      isLoading={isLoading}
      explanation="Required for global keyboard shortcuts and system-wide features."
    />
  )
}
