import { Button } from '@renderer/components/ui/button'
import { PlugIcon, StopCircle, Download, PlayCircle, Loader2 } from 'lucide-react'
import ScreenpipeConnectionModal from './ScreenpipeConnectionModal'
import { useScreenpipeConnection } from '@renderer/hooks/useScreenpipeConnection'
import { getSafeScreenRecordingPermission } from '@renderer/lib/utils/permissionUtils'
import { useEffect, useRef } from 'react'

interface ScreenpipeConnectionButtonProps {
  onConnectionSuccess?: () => void
  buttonText?: string
  variant?: 'default' | 'outline' | 'secondary' | 'destructive' | 'ghost' | 'link'
  size?: 'default' | 'sm' | 'lg' | 'icon'
  className?: string
  shouldAutoOpen?: boolean
}

export default function ScreenpipeConnectionButton({
  onConnectionSuccess,
  buttonText,
  variant = 'outline',
  size = 'default',
  className,
  shouldAutoOpen = false
}: ScreenpipeConnectionButtonProps) {
  const {
    permissions,
    connectionState,
    showConnectionModal,
    setShowConnectionModal,
    isLoading,
    canConnect,
    getButtonLabel,
    handlePrimaryAction,
    handleRequestPermission,
    handleStartScreenpipe,
    handleStopScreenpipe,
    fetchStatus
  } = useScreenpipeConnection({ shouldShowModalFromSearch: shouldAutoOpen })

  const previousConnectionState = useRef(connectionState)
  const hasCalledSuccess = useRef(false)

  // Watch for connection state changes to trigger success callback
  useEffect(() => {
    // If we transitioned from any other state to 'running', call the success callback
    if (
      previousConnectionState.current !== 'running' &&
      connectionState === 'running' &&
      onConnectionSuccess &&
      !hasCalledSuccess.current
    ) {
      console.log(
        '[ScreenpipeConnectionButton] Screenpipe is now running, calling onConnectionSuccess'
      )
      hasCalledSuccess.current = true
      onConnectionSuccess()
    }

    // Reset the flag when transitioning away from running
    if (connectionState !== 'running') {
      hasCalledSuccess.current = false
    }

    previousConnectionState.current = connectionState
  }, [connectionState, onConnectionSuccess])

  const handleButtonClick = async () => {
    try {
      await handlePrimaryAction()
    } catch (error) {
      console.error('Connection action failed:', error)
    }
  }

  const handleStartScreenpipeWithCallback = async () => {
    try {
      await handleStartScreenpipe()
      // The success callback will be triggered by the useEffect when state changes
    } catch (error) {
      console.error('Failed to start Screenpipe:', error)
    }
  }

  // Special version for modal auto-start that throws on permission issues
  const handleAutoStartForModal = async () => {
    try {
      // Always try to start Screenpipe - it will handle the system permission dialog
      const result = await window.api.screenpipe.start()

      if (!result.success) {
        // Check if the failure is due to permissions
        if (result.error && result.error.toLowerCase().includes('permission')) {
          throw new Error('Permission denied')
        }
        throw new Error(result.error || 'Failed to start Screenpipe')
      }

      // If successful, fetch the latest status
      await fetchStatus()
    } catch (error) {
      console.error('[ScreenpipeConnectionButton] Auto-start failed:', error)
      throw error // Re-throw to trigger the permissions UI in the modal
    }
  }

  // Get the appropriate icon based on state
  const getIcon = () => {
    if (isLoading) return <Loader2 className="w-4 h-4 mr-1 animate-spin" />

    switch (connectionState) {
      case 'not-installed':
        return <Download className="w-4 h-4 mr-1" />
      case 'permissions-required':
        return <PlugIcon className="w-4 h-4 mr-1" />
      case 'ready':
        return <PlayCircle className="w-4 h-4 mr-1" />
      case 'running':
        return <StopCircle className="w-4 h-4 mr-1" />
      default:
        return <PlugIcon className="w-4 h-4 mr-1" />
    }
  }

  const displayText = buttonText || getButtonLabel()

  return (
    <>
      <Button
        variant={variant}
        size={size}
        onClick={handleButtonClick}
        className={className}
        disabled={!canConnect || isLoading}
      >
        {getIcon()}
        {displayText}
      </Button>

      <ScreenpipeConnectionModal
        isOpen={showConnectionModal}
        onClose={() => setShowConnectionModal(false)}
        screenRecordingPermission={getSafeScreenRecordingPermission(permissions.screen)}
        isScreenpipeRunning={connectionState === 'running'}
        onRequestPermission={handleRequestPermission}
        onStartScreenpipe={handleAutoStartForModal}
        onStopScreenpipe={handleStopScreenpipe}
      />
    </>
  )
}
