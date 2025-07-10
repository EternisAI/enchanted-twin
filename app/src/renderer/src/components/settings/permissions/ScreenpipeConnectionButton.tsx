import { Button } from '@renderer/components/ui/button'
import { PlugIcon } from 'lucide-react'
import ScreenpipeConnectionModal from './ScreenpipeConnectionModal'
import { useScreenpipeConnection } from '@renderer/hooks/useScreenpipeConnection'
import { getSafeScreenRecordingPermission } from '@renderer/lib/utils/permissionUtils'

interface ScreenpipeConnectionButtonProps {
  onConnectionSuccess?: () => void
  buttonText?: string
  variant?: 'default' | 'outline' | 'secondary' | 'destructive' | 'ghost' | 'link'
  size?: 'default' | 'sm' | 'lg' | 'icon'
  className?: string
}

export default function ScreenpipeConnectionButton({
  onConnectionSuccess,
  buttonText = 'Connect',
  variant = 'outline',
  size = 'sm',
  className
}: ScreenpipeConnectionButtonProps) {
  const {
    status,
    permissions,
    showConnectionModal,
    setShowConnectionModal,
    needsConnection,
    handleConnect,
    handleRequestPermission,
    handleStartScreenpipe
  } = useScreenpipeConnection()

  const handleConnectionComplete = async () => {
    try {
      if (needsConnection) {
        handleConnect()
      } else {
        onConnectionSuccess?.()
      }
    } catch (error) {
      console.error('Connection failed:', error)
    }
  }

  const handleStartScreenpipeWithCallback = async () => {
    try {
      await handleStartScreenpipe()
      // Check if connection is complete after starting
      if (!needsConnection) {
        setShowConnectionModal(false)
        onConnectionSuccess?.()
      }
    } catch (error) {
      throw error
    }
  }


  return (
    <>
      <Button
        variant={variant}
        size={size}
        onClick={handleConnectionComplete}
        className={className}
        disabled={!status.isInstalled && needsConnection}
      >
        <PlugIcon className="w-4 h-4 mr-1" />
        {buttonText}
      </Button>

      <ScreenpipeConnectionModal
        isOpen={showConnectionModal}
        onClose={() => setShowConnectionModal(false)}
        screenRecordingPermission={getSafeScreenRecordingPermission(permissions.screen)}
        isScreenpipeRunning={status.isRunning}
        onRequestPermission={handleRequestPermission}
        onStartScreenpipe={handleStartScreenpipeWithCallback}
      />
    </>
  )
}