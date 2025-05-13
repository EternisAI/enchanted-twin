import { Card, CardContent, CardHeader } from '@renderer/components/ui/card'
import { Play, StopCircle, AlertCircle, Download } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Button } from '@renderer/components/ui/button'
import { Alert, AlertDescription } from '@renderer/components/ui/alert'

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
  const [status, setStatus] = useState<ScreenpipeStatus>({ isRunning: false, isInstalled: false })
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [permissions, setPermissions] = useState<Record<string, MediaStatusType>>({
    screen: 'loading',
    microphone: 'loading',
    accessibility: 'loading'
  })

  const fetchStatus = async () => {
    try {
      const status = await window.api.screenpipe.getStatus()
      setStatus(status)
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
    } catch (err: unknown) {
      setError(`Failed to stop Screenpipe: ${err}`)
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

  return (
    <Card className="w-full">
      <CardHeader className="text-lg font-semibold flex items-center justify-between">
        <span>Screenpipe</span>
        <span
          className={`text-sm font-medium px-2 py-1 rounded-full ${
            status.isRunning ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'
          }`}
        >
          {status.isRunning ? 'Running' : 'Stopped'}
        </span>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        {error && (
          <Alert variant="destructive">
            <AlertCircle className="h-4 w-4" />
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}
        {!hasAllPermissions() && (
          <Alert variant="destructive">
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
              ? 'Screenpipe is currently active and streaming.'
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
      </CardContent>
    </Card>
  )
}
