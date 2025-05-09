'use client'
import { useEffect, useState } from 'react'
import { Button } from '../../ui/button'

type MediaType = 'camera' | 'microphone' | 'screen'
type MediaStatus =
  | 'granted'
  | 'not-determined'
  | 'denied'
  | 'restricted'
  | 'unavailable'
  | 'loading'

const types: MediaType[] = ['camera', 'microphone', 'screen']

interface RowProps {
  type: MediaType
  status: string
  queryAllMediaStatus(): void
}

function StatusRow({ type, status, queryAllMediaStatus }: RowProps) {
  const formattedStatus =
    {
      loading: 'Loading',
      granted: 'Granted',
      'not-determined': 'No permission',
      denied: 'Denied',
      restricted: 'Restricted',
      unavailable: 'Unavailable'
    }[status] || status

  const requestAccess = async () => {
    await window.api.requestMediaAccess(type)
    await queryAllMediaStatus()
  }

  return (
    <div className="grid grid-cols-3 items-center">
      <span className="capitalize">{type}</span>
      <span className="text-center">{formattedStatus}</span>

      <div className="flex justify-end">
        {status === 'not-determined' && (
          <Button className="w-fit" size="sm" onClick={requestAccess}>
            Request Permission
          </Button>
        )}

        {(status === 'denied' || status === 'restricted' || status === 'granted') && (
          <Button className="w-fit" size="sm" onClick={requestAccess}>
            Open {type} settings
          </Button>
        )}
      </div>
    </div>
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
    <div className="flex flex-col gap-6">
      {types.map((type) => (
        <StatusRow
          key={type}
          type={type}
          status={status[type]}
          queryAllMediaStatus={queryAllMediaStatus}
        />
      ))}
    </div>
  )
}
