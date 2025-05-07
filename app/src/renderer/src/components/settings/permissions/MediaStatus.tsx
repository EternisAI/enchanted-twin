'use client'
import { useEffect, useState } from 'react'
import { Button } from '../../ui/button'
import { Camera, Mic, Monitor, CheckCircle2, XCircle, AlertCircle, HelpCircle } from 'lucide-react'
import { cn } from '@renderer/lib/utils'
import { Card } from '@renderer/components/ui/card'

type MediaType = 'camera' | 'microphone' | 'screen'
type MediaStatus =
  | 'granted'
  | 'not-determined'
  | 'denied'
  | 'restricted'
  | 'unavailable'
  | 'loading'

const types: MediaType[] = ['camera', 'microphone', 'screen']

const typeIcons = {
  camera: Camera,
  microphone: Mic,
  screen: Monitor
}

const statusConfig = {
  loading: {
    icon: HelpCircle,
    color: 'text-muted-foreground',
    label: 'Loading'
  },
  granted: {
    icon: CheckCircle2,
    color: 'text-green-500',
    label: 'Granted'
  },
  'not-determined': {
    icon: HelpCircle,
    color: 'text-muted-foreground',
    label: 'Not determined'
  },
  denied: {
    icon: XCircle,
    color: 'text-red-500',
    label: 'Denied'
  },
  restricted: {
    icon: AlertCircle,
    color: 'text-yellow-500',
    label: 'Restricted'
  },
  unavailable: {
    icon: XCircle,
    color: 'text-red-500',
    label: 'Unavailable'
  }
}

interface StatusCardProps {
  type: MediaType
  status: string
  queryAllMediaStatus(): void
}

function StatusCard({ type, status, queryAllMediaStatus }: StatusCardProps) {
  const TypeIcon = typeIcons[type]
  const statusInfo = statusConfig[status as keyof typeof statusConfig] || {
    icon: HelpCircle,
    color: 'text-muted-foreground',
    label: status
  }
  const StatusIcon = statusInfo.icon

  const requestAccess = async () => {
    await window.api.requestMediaAccess(type)
    await queryAllMediaStatus()
  }

  return (
    <Card className="p-4 min-w-[200px] flex flex-col items-center gap-3">
      <div className="flex flex-col items-center gap-2">
        <TypeIcon className="h-8 w-8 text-muted-foreground" />
        <span className="capitalize font-medium">{type}</span>
      </div>

      <div className="flex items-center gap-2">
        <StatusIcon className={cn('h-5 w-5', statusInfo.color)} />
        <span className={cn('text-sm', statusInfo.color)}>{statusInfo.label}</span>
      </div>

      <Button variant="outline" className="w-full" size="sm" onClick={requestAccess}>
        {status === 'not-determined' ? 'Request' : 'Settings'}
      </Button>
    </Card>
  )
}

export default function MediaStatus() {
  const [status, setStatus] = useState<Record<MediaType, MediaStatus>>({
    camera: 'loading',
    microphone: 'loading',
    screen: 'loading'
  })

  const queryAllMediaStatus = async () => {
    const pairs = await Promise.all(
      types.map(async (type) => [type, await window.api.queryMediaStatus(type)])
    )
    setStatus(Object.fromEntries(pairs) as Record<MediaType, MediaStatus>)
  }

  useEffect(() => {
    queryAllMediaStatus()
  }, [])

  return (
    <div className="flex flex-wrap gap-4">
      {types.map((type) => (
        <StatusCard
          key={type}
          type={type}
          status={status[type]}
          queryAllMediaStatus={queryAllMediaStatus}
        />
      ))}
    </div>
  )
}
