'use client'
import { useEffect, useState } from 'react'
import { Button } from '@renderer/components/ui/button'

export default function AccessibilityStatus() {
  const [status, setStatus] = useState<string>('loading')

  const checkPermission = async () => {
    try {
      const accessibilityStatus = await window.api.accessibility.getStatus()
      setStatus(accessibilityStatus)
    } catch (error) {
      console.error('Error checking accessibility permission:', error)
      setStatus('error')
    }
  }

  useEffect(() => {
    checkPermission()
  }, [])

  const requestPermission = async () => {
    await window.api.accessibility.request()
    setTimeout(() => checkPermission(), 500)
  }

  const formattedStatus =
    {
      loading: 'Loading',
      granted: 'Granted',
      denied: 'Denied',
      unavailable: 'Unavailable',
      error: 'Error'
    }[status] || status

  return (
    <div className="flex flex-col gap-1">
      <div className="grid grid-cols-3 items-center py-1">
        <span>Accessibility</span>
        <span className="text-center">{formattedStatus}</span>
        <div className="flex justify-end">
          <Button className="w-fit" size="sm" onClick={requestPermission}>
            {status === 'denied' ? 'Request' : 'Open settings'}
          </Button>
        </div>
      </div>
    </div>
  )
}
