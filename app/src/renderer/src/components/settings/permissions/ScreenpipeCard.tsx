import {
  StopCircle,
  AlertCircle,
  Download,
  Monitor,
  CheckCircle2,
  XCircle,
  Settings,
  PlugIcon,
  PlayCircle
} from 'lucide-react'
import { useEffect, useState } from 'react'
import { Button } from '@renderer/components/ui/button'
import { Alert, AlertDescription } from '@renderer/components/ui/alert'
import ScreenpipeConnectionModal from './ScreenpipeConnectionModal'
import { DetailCard } from './DetailCard'
import { useScreenpipeConnection } from '@renderer/hooks/useScreenpipeConnection'
import { getSafeScreenRecordingPermission } from '@renderer/lib/utils/permissionUtils'
import { useSearch } from '@tanstack/react-router'

export default function ScreenpipePanel() {
  const [autoStart, setAutoStart] = useState<boolean>(false)

  // Safely get search parameters
  let shouldShowModalFromSearch = false
  try {
    const searchParams = useSearch({ from: '/settings/permissions' })
    if (searchParams && 'screenpipe' in searchParams) {
      // Handle both string "true" and boolean true
      shouldShowModalFromSearch =
        searchParams.screenpipe === 'true' || searchParams.screenpipe === true
    }
  } catch {
    // Ignore errors accessing search params
  }

  const {
    connectionState,
    permissions,
    showConnectionModal,
    setShowConnectionModal,
    isLoading,
    error,
    getButtonLabel,
    getStatusInfo,
    handlePrimaryAction,
    handleRequestPermission,
    handleStartScreenpipe,
    handleStopScreenpipe
  } = useScreenpipeConnection({ shouldShowModalFromSearch })

  useEffect(() => {
    const fetchAutoStart = async () => {
      try {
        const autoStartSetting = await window.api.screenpipe.getAutoStart()
        setAutoStart(autoStartSetting)
      } catch (err: unknown) {
        console.error(`Failed to fetch auto-start setting: ${err}`)
      }
    }

    fetchAutoStart()
    const interval = setInterval(fetchAutoStart, 5000)
    return () => clearInterval(interval)
  }, [])

  const getStatusIcon = () => {
    switch (connectionState) {
      case 'not-installed':
        return Download
      case 'permissions-required':
        return XCircle
      case 'ready':
        return XCircle
      case 'running':
        return CheckCircle2
      case 'loading':
        return Settings
    }
  }

  const getExplanation = () => {
    const statusInfo = getStatusInfo()
    const baseDescription = statusInfo.description

    if (connectionState === 'running' && autoStart) {
      return 'Currently capturing screen activity and will auto-start on launch.'
    } else if (connectionState === 'ready' && autoStart) {
      return 'Ready to capture screen activity. Will auto-start on next launch.'
    }

    return baseDescription
  }

  const getGrantedIcon = () => {
    if (connectionState === 'running') {
      return (
        <div className="flex gap-1">
          <Button
            onClick={handleStopScreenpipe}
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
        statusInfo={{
          icon: getStatusIcon(),
          color: statusInfo.color,
          label: statusInfo.label
        }}
        buttonLabel={getButtonLabel()}
        onButtonClick={handlePrimaryAction}
        isLoading={isLoading}
        explanation={getExplanation()}
        grantedIcon={getGrantedIcon()}
      />

      <ScreenpipeConnectionModal
        isOpen={showConnectionModal}
        onClose={() => setShowConnectionModal(false)}
        screenRecordingPermission={getSafeScreenRecordingPermission(permissions.screen)}
        isScreenpipeRunning={connectionState === 'running'}
        onRequestPermission={handleRequestPermission}
        onStartScreenpipe={handleStartScreenpipe}
        onStopScreenpipe={handleStopScreenpipe}
      />
    </>
  )
}
