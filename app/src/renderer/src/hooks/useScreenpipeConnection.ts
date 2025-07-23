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

// Unified connection states
export type ConnectionState =
  | 'not-installed'
  | 'permissions-required'
  | 'ready'
  | 'running'
  | 'loading'

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
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

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

  // Derived state helpers
  const hasAllPermissions = () => {
    return Object.values(permissions).every((status) => status === 'granted')
  }

  const isPermissionsLoading = () => {
    return Object.values(permissions).some((status) => status === 'loading')
  }

  const getMissingPermissions = (): string[] => {
    const missing: string[] = []
    if (permissions.screen !== 'granted') missing.push('Screen Recording')
    if (permissions.microphone !== 'granted') missing.push('Microphone')
    if (permissions.accessibility !== 'granted') missing.push('Accessibility')
    return missing
  }

  // Unified connection state
  const getConnectionState = (): ConnectionState => {
    if (isPermissionsLoading() || isLoading) return 'loading'
    if (!status.isInstalled) return 'not-installed'
    if (!hasAllPermissions()) return 'permissions-required'
    if (status.isRunning) return 'running'
    return 'ready'
  }

  const connectionState = getConnectionState()
  const needsConnection =
    connectionState === 'not-installed' ||
    connectionState === 'permissions-required' ||
    connectionState === 'ready'
  const canConnect = connectionState !== 'not-installed' && connectionState !== 'loading'

  // Get appropriate button label based on state
  const getButtonLabel = () => {
    switch (connectionState) {
      case 'not-installed':
        return 'Install'
      case 'permissions-required':
        return 'Connect'
      case 'ready':
        return 'Start'
      case 'running':
        return 'Stop'
      case 'loading':
        return 'Loading...'
    }
  }

  // Get status information for display
  const getStatusInfo = () => {
    switch (connectionState) {
      case 'not-installed':
        return {
          label: 'Not Installed',
          color: 'text-orange-500 dark:text-orange-400',
          description: 'Install Screenpipe to enable AI screen context awareness.'
        }
      case 'permissions-required':
        return {
          label: 'Permissions Required',
          color: 'text-neutral-500 dark:text-neutral-400',
          description: `Missing permissions: ${getMissingPermissions().join(', ')}.`
        }
      case 'ready':
        return {
          label: 'Stopped',
          color: 'text-neutral-500 dark:text-neutral-400',
          description: 'Ready to capture screen activity for AI context.'
        }
      case 'running':
        return {
          label: 'Running',
          color: 'text-green-500 dark:text-green-400',
          description: 'Currently capturing screen activity for AI context.'
        }
      case 'loading':
        return {
          label: 'Loading...',
          color: 'text-neutral-400',
          description: 'Checking status...'
        }
    }
  }

  const handleConnect = () => {
    setShowConnectionModal(true)
  }

  const handleRequestPermission = async () => {
    try {
      await window.api.screenpipe.storeRestartIntent('/settings/data-sources#screenpipe', true)
      await window.api.requestMediaAccess('screen')
    } catch (error) {
      console.error('Error requesting screen permission:', error)
      setError('Failed to request screen permission')
      throw error
    }
  }

  const handleInstall = async () => {
    setIsLoading(true)
    setError(null)
    try {
      const result = await window.api.screenpipe.install()
      if (!result.success) {
        throw new Error(result.error || 'Failed to install Screenpipe')
      }
      await fetchStatus()
      toast.success('Screenpipe installed successfully')
    } catch (error) {
      console.error('Error installing Screenpipe:', error)
      const message = error instanceof Error ? error.message : 'Failed to install Screenpipe'
      setError(message)
      toast.error(message)
      throw error
    } finally {
      setIsLoading(false)
    }
  }

  const handleStartScreenpipe = async () => {
    setIsLoading(true)
    setError(null)
    try {
      await checkPermissions()
      if (permissions.screen !== 'granted') {
        setShowConnectionModal(true)
        return
      }
      const result = await window.api.screenpipe.start()
      if (!result.success) {
        throw new Error(result.error || 'Failed to start Screenpipe')
      }
      await fetchStatus()
      toast.success('Screenpipe started successfully')
    } catch (error) {
      console.error('Error starting Screenpipe:', error)
      const message = error instanceof Error ? error.message : 'Failed to start Screenpipe'
      setError(message)
      toast.error(message)
      throw error
    } finally {
      setIsLoading(false)
    }
  }

  const handleStopScreenpipe = async () => {
    setIsLoading(true)
    setError(null)
    try {
      await window.api.screenpipe.stop()
      await fetchStatus()
      toast.success('Screenpipe stopped')
    } catch (error) {
      console.error('Error stopping Screenpipe:', error)
      const message = error instanceof Error ? error.message : 'Failed to stop Screenpipe'
      setError(message)
      toast.error(message)
      throw error
    } finally {
      setIsLoading(false)
    }
  }

  // Main action handler based on connection state
  const handlePrimaryAction = async () => {
    switch (connectionState) {
      case 'not-installed':
        await handleInstall()
        break
      case 'permissions-required':
        handleConnect()
        break
      case 'ready':
        await handleStartScreenpipe()
        break
      case 'running':
        await handleStopScreenpipe()
        break
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
    // Core state
    status,
    permissions,
    connectionState,

    // UI state
    showConnectionModal,
    setShowConnectionModal,
    isLoading,
    error,

    // Derived state
    hasAllPermissions,
    needsConnection,
    canConnect,
    getMissingPermissions,
    getButtonLabel,
    getStatusInfo,

    // Actions
    handleConnect,
    handleRequestPermission,
    handleInstall,
    handleStartScreenpipe,
    handleStopScreenpipe,
    handlePrimaryAction,
    fetchStatus
  }
}
