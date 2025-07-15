import { useState, useEffect, useCallback } from 'react'

type MicrophoneStatus = 'granted' | 'denied' | 'not-determined' | 'loading'

export interface UseMicrophonePermissionReturn {
  microphoneStatus: MicrophoneStatus
  isRequestingAccess: boolean
  queryMicrophoneStatus: () => Promise<void>
  requestMicrophoneAccess: () => Promise<void>
}

export default function useMicrophonePermission(): UseMicrophonePermissionReturn {
  const [microphoneStatus, setMicrophoneStatus] = useState<MicrophoneStatus>('loading')
  const [isRequestingAccess, setIsRequestingAccess] = useState(false)

  const queryMicrophoneStatus = useCallback(async () => {
    try {
      const status = await window.api.queryMediaStatus('microphone')
      setMicrophoneStatus(status as MicrophoneStatus)
    } catch (error) {
      console.error('Error querying microphone status:', error)
      setMicrophoneStatus('denied')
    }
  }, [])

  const requestMicrophoneAccess = useCallback(async () => {
    try {
      setIsRequestingAccess(true)
      await window.api.requestMediaAccess('microphone')
      await queryMicrophoneStatus()

      window.api.analytics.capture('permission_asked', {
        name: 'microphone'
      })
    } catch (error) {
      console.error('Error requesting microphone access:', error)
    } finally {
      setIsRequestingAccess(false)
    }
  }, [queryMicrophoneStatus])

  useEffect(() => {
    queryMicrophoneStatus()
    const interval = setInterval(queryMicrophoneStatus, 5000)
    return () => clearInterval(interval)
  }, [queryMicrophoneStatus])

  return {
    microphoneStatus,
    isRequestingAccess,
    queryMicrophoneStatus,
    requestMicrophoneAccess
  }
}
