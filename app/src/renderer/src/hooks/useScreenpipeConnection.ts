import { useState, useEffect } from 'react'
import { toast } from 'sonner'

type MediaStatusType =
  | 'granted'
  | 'not-determined'
  | 'denied'
  | 'restricted'
  | 'unavailable'
  | 'loading'

interface ScreenpipeStatus {
  isRunning: boolean
  isInstalled: boolean
}

interface UseScreenpipeConnectionOptions {
  shouldShowModalFromSearch?: boolean
}

export function useScreenpipeConnection(options: UseScreenpipeConnectionOptions = {}) {
  const [status, setStatus] = useState<ScreenpipeStatus>({ isRunning: false, isInstalled: true })
  const [permissions, setPermissions] = useState<Record<string, MediaStatusType>>({
    screen: 'loading',
    microphone: 'loading',
    accessibility: 'loading'
  })
  const [showConnectionModal, setShowConnectionModal] = useState(false)

  const fetchStatus = async () => {
    try {
      const status = await window.api.screenpipe.getStatus()
      setStatus(status)
    } catch (err: unknown) {
      console.error(`Failed to fetch screenpipe status: ${err}`)
    }
  }

  const checkPermissions = async () => {
    const screenStatus = await window.api.queryMediaStatus('screen')
    const micStatus = await window.api.queryMediaStatus('microphone')
    const accessibilityStatus = await window.api.accessibility.getStatus()

    setPermissions({
      screen: screenStatus as MediaStatusType,
      microphone: micStatus as MediaStatusType,
      accessibility: accessibilityStatus as MediaStatusType
    })
  }

  const hasAllPermissions = () => {
    return Object.values(permissions).every((status) => status === 'granted')
  }

  const needsConnection = !hasAllPermissions() || !status.isRunning

  const handleConnect = () => {
    setShowConnectionModal(true)
  }

  const handleRequestPermission = async () => {
    try {
      await window.api.screenpipe.storeRestartIntent('/settings/permissions', true)
      await window.api.requestMediaAccess('screen')
    } catch (error) {
      console.error('Error requesting screen permission:', error)
      throw error
    }
  }

  const handleStartScreenpipe = async () => {
    try {
      const result = await window.api.screenpipe.start()
      if (!result.success) {
        throw new Error(result.error || 'Failed to start Screenpipe')
      }
      await fetchStatus()
      toast.success('Screenpipe started successfully')
    } catch (error) {
      console.error('Error starting Screenpipe:', error)
      toast.error('Failed to start Screenpipe')
      throw error
    }
  }

  useEffect(() => {
    fetchStatus()
    const fetchStatusInterval = setInterval(fetchStatus, 5000)

    checkPermissions()
    const interval = setInterval(checkPermissions, 5000)
    
    return () => {
      clearInterval(interval)
      clearInterval(fetchStatusInterval)
    }
  }, [])

  useEffect(() => {
    if (options.shouldShowModalFromSearch) {
      setShowConnectionModal(true)
    }
  }, [options.shouldShowModalFromSearch])

  return {
    status,
    permissions,
    showConnectionModal,
    setShowConnectionModal,
    hasAllPermissions,
    needsConnection,
    handleConnect,
    handleRequestPermission,
    handleStartScreenpipe,
    fetchStatus
  }
}