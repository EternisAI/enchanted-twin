import { Play, StopCircle, AlertCircle, Download, Monitor, Link } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Button } from '@renderer/components/ui/button'
import { Alert, AlertDescription } from '@renderer/components/ui/alert'
import { toast } from 'sonner'
import IconContainer from '@renderer/assets/icons/IconContainer'
import ScreenpipeConnectionModal from './ScreenpipeConnectionModal'
import { useSearch } from '@tanstack/react-router'

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

export default function ScreenpipePanel() {
  const [status, setStatus] = useState<ScreenpipeStatus>({ isRunning: false, isInstalled: true })
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [autoStart, setAutoStart] = useState<boolean>(false)
  const [permissions, setPermissions] = useState<Record<string, MediaStatusType>>({
    screen: 'loading',
    microphone: 'loading',
    accessibility: 'loading'
  })
  const [showConnectionModal, setShowConnectionModal] = useState(false)
  const searchParams = useSearch({ from: '/settings/permissions' })

  const fetchStatus = async () => {
    try {
      const status = await window.api.screenpipe.getStatus()
      const autoStartSetting = await window.api.screenpipe.getAutoStart()
      setStatus(status)
      setAutoStart(autoStartSetting)
    } catch (err: unknown) {
      setError(`Failed to fetch screenpipe status: ${err}`)
    }
  }

  useEffect(() => {
    fetchStatus()
    const fetchStatusInterval = setInterval(fetchStatus, 5000)

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
    checkPermissions()
    const interval = setInterval(checkPermissions, 5000)
    return () => {
      clearInterval(interval)
      clearInterval(fetchStatusInterval)
    }
  }, [])

  useEffect(() => {
    if (searchParams && 'screenpipe' in searchParams && searchParams.screenpipe === 'true') {
      setShowConnectionModal(true)
    }
  }, [searchParams])

  const handleInstall = async () => {
    setIsLoading(true)
    setError(null)
    try {
      const result = await window.api.screenpipe.install()
      if (!result.success) {
        setError(result.error || 'Failed to install Screenpipe')
      } else {
        await fetchStatus()
      }
    } catch (err: unknown) {
      setError(`Failed to install Screenpipe: ${err}`)
    } finally {
      setIsLoading(false)
    }
  }

  const handleStart = async () => {
    setIsLoading(true)
    setError(null)
    try {
      const result = await window.api.screenpipe.start()
      if (!result.success) {
        setError(result.error || 'Failed to start Screenpipe')
      } else {
        await fetchStatus()
      }
    } catch (err: unknown) {
      setError(`Failed to start Screenpipe: ${err}`)
    } finally {
      setIsLoading(false)
    }
  }

  const handleStop = async () => {
    setIsLoading(true)
    setError(null)
    try {
      await window.api.screenpipe.stop()
      await fetchStatus()
      toast.success('Screenpipe stopped')
    } catch (err: unknown) {
      setError(`Failed to stop Screenpipe: ${err}`)
      toast.error('Failed to stop Screenpipe')
    } finally {
      setIsLoading(false)
    }
  }

  const hasAllPermissions = () => {
    return Object.values(permissions).every((status) => status === 'granted')
  }

  const getPermissionMessages = (): string[] => {
    const messages: string[] = []
    if (permissions.screen !== 'granted') messages.push('Screen Recording')
    if (permissions.microphone !== 'granted') messages.push('Microphone')
    if (permissions.accessibility !== 'granted') messages.push('Accessibility')
    return messages
  }

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

  const needsConnection = !hasAllPermissions() || !status.isRunning

  return (
    <div className="w-full bg-transparent border-none">
      <div className="text-lg font-semibold flex items-center gap-2">
        <IconContainer className="bg-muted/50">
          <Monitor className="h-7 w-7" />
        </IconContainer>
        <span className="text-lg font-semibold">Screenpipe</span>
        <div className="flex gap-2">
          {autoStart && (
            <span className="text-xs font-medium px-2 py-1 rounded-full bg-blue-100 text-blue-800">
              Auto-start
            </span>
          )}
          {/* <span
            className={`text-sm font-medium px-2 py-1 rounded-full ${
              status.isRunning ? 'bg-green-100 text-green-800' : 'bg-neutral-100 text-neutral-800'
            }`}
          >
            {status.isRunning ? 'Running' : 'Stopped'}
          </span> */}
        </div>
      </div>
      <div className="flex flex-col gap-4 pt-4">
        {error && (
          <Alert variant="destructive">
            <AlertCircle className="h-4 w-4" />
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}
        {!hasAllPermissions() &&
          Object.values(permissions).every((status) => status !== 'loading') && (
            <Alert variant="default">
              <AlertCircle className="h-4 w-4" />
              <AlertDescription>
                Please enable the following permissions to use Screenpipe:{' '}
                {getPermissionMessages().join(', ')}
              </AlertDescription>
            </Alert>
          )}
        <p className="text-sm text-muted-foreground">
          {!status.isInstalled
            ? 'Screenpipe needs to be installed first.'
            : status.isRunning
              ? autoStart
                ? 'Screenpipe is currently active and will auto-start on app launch.'
                : 'Screenpipe is currently active.'
              : autoStart
                ? 'Screenpipe is not running but will auto-start on next app launch.'
                : 'Screenpipe is not running. Start it to enable screen streaming.'}
        </p>
        <div className="flex gap-2">
          {!status.isInstalled ? (
            <Button
              onClick={handleInstall}
              disabled={isLoading}
              variant="default"
              className="flex items-center gap-1"
            >
              <Download className="w-4 h-4" />
              Install Screenpipe
            </Button>
          ) : (
            <>
              {needsConnection && (
                <Button
                  onClick={handleConnect}
                  disabled={isLoading}
                  variant="default"
                  className="flex items-center gap-1"
                >
                  <Link className="w-4 h-4" />
                  Connect
                </Button>
              )}
              <Button
                onClick={handleStart}
                disabled={isLoading || status.isRunning || !hasAllPermissions()}
                variant="default"
                className="flex items-center gap-1"
              >
                <Play className="w-4 h-4" />
                Start
              </Button>
              <Button
                onClick={handleStop}
                disabled={isLoading || !status.isRunning}
                variant="destructive"
                className="flex items-center gap-1"
              >
                <StopCircle className="w-4 h-4" />
                Stop
              </Button>
            </>
          )}
        </div>
      </div>

      <ScreenpipeConnectionModal
        isOpen={showConnectionModal}
        onClose={() => setShowConnectionModal(false)}
        screenRecordingPermission={
          permissions.screen as
            | 'granted'
            | 'denied'
            | 'not-determined'
            | 'restricted'
            | 'unavailable'
        }
        isScreenpipeRunning={status.isRunning}
        onRequestPermission={handleRequestPermission}
        onStartScreenpipe={handleStartScreenpipe}
      />
    </div>
  )
}
