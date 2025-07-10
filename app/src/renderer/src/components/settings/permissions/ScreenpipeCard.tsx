import {
  StopCircle,
  AlertCircle,
  Download,
  Monitor,
  CheckCircle2,
  XCircle,
  Settings
} from 'lucide-react'
import { useEffect, useState } from 'react'
import { Button } from '@renderer/components/ui/button'
import { Alert, AlertDescription } from '@renderer/components/ui/alert'
import { toast } from 'sonner'
import ScreenpipeConnectionModal from './ScreenpipeConnectionModal'
import { DetailCard } from './DetailCard'
import { useScreenpipeConnection } from '@renderer/hooks/useScreenpipeConnection'
import { getSafeScreenRecordingPermission } from '@renderer/lib/utils/permissionUtils'

export default function ScreenpipePanel() {
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [autoStart, setAutoStart] = useState<boolean>(false)

  const {
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
  } = useScreenpipeConnection()

  useEffect(() => {
    const fetchAutoStart = async () => {
      try {
        const autoStartSetting = await window.api.screenpipe.getAutoStart()
        setAutoStart(autoStartSetting)
      } catch (err: unknown) {
        setError(`Failed to fetch auto-start setting: ${err}`)
      }
    }

    fetchAutoStart()
    const interval = setInterval(fetchAutoStart, 5000)
    return () => clearInterval(interval)
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
      toast.success('Screenpipe stopped')
    } catch (err: unknown) {
      setError(`Failed to stop Screenpipe: ${err}`)
      toast.error('Failed to stop Screenpipe')
    } finally {
      setIsLoading(false)
    }
  }

  const getPermissionMessages = (): string[] => {
    const messages: string[] = []
    if (permissions.screen !== 'granted') messages.push('Screen Recording')
    if (permissions.microphone !== 'granted') messages.push('Microphone')
    if (permissions.accessibility !== 'granted') messages.push('Accessibility')
    return messages
  }

  const getStatusInfo = () => {
    if (!status.isInstalled) {
      return {
        icon: Download,
        color: 'text-orange-500 dark:text-orange-400',
        label: 'Not Installed'
      }
    }

    if (!hasAllPermissions()) {
      return {
        icon: XCircle,
        color: 'text-neutral-500 dark:text-neutral-400',
        label: 'Permissions Required'
      }
    }

    if (status.isRunning) {
      return {
        icon: CheckCircle2,
        color: 'text-green-500 dark:text-green-400',
        label: 'Running'
      }
    }

    return {
      icon: XCircle,
      color: 'text-neutral-500 dark:text-neutral-400',
      label: 'Stopped'
    }
  }

  const getButtonLabel = () => {
    if (!status.isInstalled) return 'Install'
    if (needsConnection) return 'Connect'
    if (status.isRunning) return 'Stop'
    return 'Start'
  }

  const handleButtonClick = () => {
    if (!status.isInstalled) {
      handleInstall()
    } else if (needsConnection) {
      handleConnect()
    } else if (status.isRunning) {
      handleStop()
    } else {
      handleStart()
    }
  }

  const getExplanation = () => {
    if (!status.isInstalled) {
      return 'Install Screenpipe to enable AI screen context awareness.'
    }
    if (!hasAllPermissions()) {
      return `Missing permissions: ${getPermissionMessages().join(', ')}.`
    }
    if (status.isRunning) {
      return autoStart
        ? 'Currently capturing screen activity and will auto-start on launch.'
        : 'Currently capturing screen activity for AI context.'
    }
    return autoStart
      ? 'Ready to capture screen activity. Will auto-start on next launch.'
      : 'Ready to capture screen activity for AI context.'
  }

  const getGrantedIcon = () => {
    if (status.isRunning) {
      return (
        <div className="flex gap-1">
          <Button
            onClick={handleStop}
            disabled={isLoading}
            variant="destructive"
            size="sm"
            className="h-fit py-1 px-2"
          >
            <StopCircle className="w-3 h-3 mr-1" />
            Stop
          </Button>
        </div>
      )
    }
    return <Settings className="h-4 w-4" />
  }

  const statusInfo = getStatusInfo()


  return (
    <>
      {error && (
        <Alert variant="destructive" className="mb-3">
          <AlertCircle className="h-4 w-4" />
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      <DetailCard
        title="Screenpipe"
        IconComponent={Monitor}
        statusInfo={statusInfo}
        buttonLabel={getButtonLabel()}
        onButtonClick={handleButtonClick}
        isLoading={isLoading}
        explanation={getExplanation()}
        grantedIcon={getGrantedIcon()}
      />

      <ScreenpipeConnectionModal
        isOpen={showConnectionModal}
        onClose={() => setShowConnectionModal(false)}
        screenRecordingPermission={getSafeScreenRecordingPermission(permissions.screen)}
        isScreenpipeRunning={status.isRunning}
        onRequestPermission={handleRequestPermission}
        onStartScreenpipe={handleStartScreenpipe}
      />
    </>
  )
}
