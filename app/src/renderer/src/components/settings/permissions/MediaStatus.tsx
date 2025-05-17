'use client'
import { useEffect, useState } from 'react'
import {
  Camera,
  Mic,
  Monitor,
  CheckCircle2,
  XCircle,
  AlertCircle,
  HelpCircle,
  LucideIcon
} from 'lucide-react'
import { DetailCard } from './DetailCard'

type MediaType = 'camera' | 'microphone' | 'screen'
type MediaStatusType =
  | 'granted'
  | 'not-determined'
  | 'denied'
  | 'restricted'
  | 'unavailable'
  | 'loading'

const types: MediaType[] = ['camera', 'microphone', 'screen']

const typeConfig: Record<MediaType, { icon: LucideIcon; title: string }> = {
  camera: { icon: Camera, title: 'Camera' },
  microphone: { icon: Mic, title: 'Microphone' },
  screen: { icon: Monitor, title: 'Screen Recording' }
}

const explanations: Record<MediaType, string> = {
  camera: 'Capture your facial expressions. (coming soon)',
  microphone: 'Speak with your Twin naturally. (coming soon)',
  screen: 'Twin understands and remembers your activity.'
}

const getStatusConfig = (status: MediaStatusType) => {
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
    case 'not-determined':
      return {
        icon: HelpCircle,
        color: 'text-muted-foreground',
        label: 'Not determined'
      }
    case 'denied':
      return {
        icon: XCircle,
        color: 'text-red-500',
        label: 'Denied'
      }
    case 'restricted':
      return {
        icon: AlertCircle,
        color: 'text-yellow-500',
        label: 'Restricted'
      }
    case 'unavailable':
      return {
        icon: XCircle,
        color: 'text-red-500',
        label: 'Unavailable'
      }
    default:
      // Handle unexpected status values gracefully
      console.warn(`Unexpected media status: ${status}`)
      return {
        icon: HelpCircle,
        color: 'text-muted-foreground',
        label: String(status) // Display the raw status if unknown
      }
  }
}

export default function MediaStatus() {
  const [status, setStatus] = useState<Record<MediaType, MediaStatusType>>({
    camera: 'loading',
    microphone: 'loading',
    screen: 'loading'
  })
  const [isLoading, setIsLoading] = useState<Record<MediaType, boolean>>({
    camera: true,
    microphone: true,
    screen: true
  })

  const queryMediaStatus = async (type: MediaType) => {
    try {
      setIsLoading((prev) => ({ ...prev, [type]: true }))
      const currentStatus = await window.api.queryMediaStatus(type)
      setStatus((prev) => ({ ...prev, [type]: currentStatus as MediaStatusType }))
    } catch (error) {
      console.error(`Error querying ${type} status:`, error)
      setStatus((prev) => ({ ...prev, [type]: 'denied' })) // Default to denied on error
    } finally {
      setIsLoading((prev) => ({ ...prev, [type]: false }))
    }
  }

  const queryAllMediaStatus = async () => {
    await Promise.all(types.map((type) => queryMediaStatus(type)))
  }

  useEffect(() => {
    queryAllMediaStatus()
    const interval = setInterval(() => {
      queryAllMediaStatus()
    }, 5000)

    return () => clearInterval(interval)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []) // Run only on mount

  const requestAccess = async (type: MediaType) => {
    try {
      setIsLoading((prev) => ({ ...prev, [type]: true }))
      await window.api.requestMediaAccess(type)
      // Re-query status after requesting access
      await queryMediaStatus(type)
    } catch (error) {
      console.error(`Error requesting ${type} access:`, error)
      // Optionally update status to reflect the error
      setIsLoading((prev) => ({ ...prev, [type]: false }))
    }
  }

  return (
    <>
      {types.map((type) => {
        const currentStatus = status[type]
        const config = typeConfig[type]
        const statusInfo = getStatusConfig(currentStatus)
        const buttonLabel =
          currentStatus === 'not-determined' || currentStatus === 'denied' ? 'Request' : 'Settings'
        const handleButtonClick = () => {
          requestAccess(type)
        }

        return (
          <DetailCard
            key={type}
            title={config.title}
            IconComponent={config.icon}
            statusInfo={statusInfo}
            buttonLabel={buttonLabel}
            onButtonClick={handleButtonClick}
            isLoading={isLoading[type]}
            explanation={explanations[type]}
          />
        )
      })}
    </>
  )
}
