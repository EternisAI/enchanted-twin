import { Card, CardContent, CardHeader } from '@renderer/components/ui/card'
import { useEffect, useState } from 'react'
import { Button } from '@renderer/components/ui/button'
import { Play, StopCircle } from 'lucide-react'

export default function ScreenpipePanel() {
  const [isRunning, setIsRunning] = useState(false)

  useEffect(() => {
    const fetchStatus = async () => {
      const status = await window.api.screenpipe.getStatus()
      setIsRunning(status)
    }
    fetchStatus()
  }, [])

  const handleStart = async () => {
    await window.api.screenpipe.start()
    setIsRunning(true)
  }

  const handleStop = async () => {
    await window.api.screenpipe.stop()
    setIsRunning(false)
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
        <p className="text-sm text-muted-foreground">
          {isRunning
            ? 'Screenpipe is currently active and streaming.'
            : 'Screenpipe is not running. Start it to enable screen streaming.'}
        </p>
        <div className="flex gap-2">
          <Button
            onClick={handleStart}
            disabled={isRunning}
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
