import { Card, CardContent, CardHeader } from '@renderer/components/ui/card'
import { useEffect, useState } from 'react'
import { Button } from '@renderer/components/ui/button'
import { Play, StopCircle, AlertCircle } from 'lucide-react'
import { Alert, AlertDescription } from '@renderer/components/ui/alert'

type MediaStatusType =
  | 'granted'
  | 'not-determined'
  | 'denied'
  | 'restricted'
  | 'unavailable'
  | 'loading'

export default function ScreenpipePanel() {
  const [isRunning, setIsRunning] = useState(false)
  const [permissions, setPermissions] = useState<Record<string, MediaStatusType>>({
    screen: 'loading',
    microphone: 'loading',
    accessibility: 'loading'
  })

  useEffect(() => {
    const fetchStatus = async () => {
      const status = await window.api.screenpipe.getStatus()
      setIsRunning(status)
    }
    fetchStatus()

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
    return () => clearInterval(interval)
  }, [])

  const handleStart = async () => {
    await window.api.screenpipe.start()
    setIsRunning(true)
  }

  const handleStop = async () => {
    await window.api.screenpipe.stop()
    setIsRunning(false)
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
            isRunning ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'
          }`}
        >
          {isRunning ? 'Running' : 'Stopped'}
        </span>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
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
          {isRunning
            ? 'Screenpipe is currently active and streaming.'
            : 'Screenpipe is not running. Start it to enable screen streaming.'}
        </p>
        <div className="flex gap-2">
          <Button
            onClick={handleStart}
            disabled={isRunning || !hasAllPermissions()}
            variant="default"
            className="flex items-center gap-1"
          >
            <Play className="w-4 h-4" />
            Start
          </Button>
          <Button
            onClick={handleStop}
            disabled={!isRunning}
            variant="destructive"
            className="flex items-center gap-1"
          >
            <StopCircle className="w-4 h-4" />
            Stop
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}
