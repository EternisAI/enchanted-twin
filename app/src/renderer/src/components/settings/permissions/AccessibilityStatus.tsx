'use client'
import { useEffect, useState } from 'react'
import { DetailCard } from './DetailCard'
import { Accessibility, CheckCircle2, XCircle, AlertCircle, HelpCircle } from 'lucide-react'

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
  }, [])

  const requestPermission = async () => {
    await window.api.accessibility.request()
    // Add a small delay before re-checking as the OS might take time to update
    setTimeout(() => checkPermission(), 500)
  }

  const openSettings = async () => {
    // Assuming there's a generic way to open OS settings, similar to notifications
    // If not, this might need adjustment based on how accessibility settings are opened
    window.api.openSettings() // Reuse the same API for now
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
          color: 'text-green-500',
          label: 'Granted'
        }
      case 'denied':
        return {
          icon: XCircle,
          color: 'text-red-500',
          label: 'Denied'
        }
      case 'unavailable':
        return {
          icon: AlertCircle,
          color: 'text-yellow-500',
          label: 'Unavailable'
        }
      case 'error':
      default:
        return {
          icon: XCircle,
          color: 'text-red-500',
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
      IconComponent={Accessibility}
      statusInfo={statusInfo}
      buttonLabel={buttonLabel}
      onButtonClick={handleButtonClick}
      isLoading={isLoading}
    />
  )
}
