import { useState, useEffect } from 'react'
import {
  Monitor,
  CheckCircle2,
  Settings,
  Play,
  Shield,
  Zap,
  StopCircleIcon,
  Loader2
} from 'lucide-react'
import { Button } from '@renderer/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription
} from '@renderer/components/ui/dialog'
import { motion } from 'framer-motion'
import { toast } from 'sonner'

interface ScreenpipeConnectionModalProps {
  isOpen: boolean
  onClose: () => void
  screenRecordingPermission: 'granted' | 'denied' | 'not-determined' | 'restricted' | 'unavailable'
  isScreenpipeRunning: boolean
  onRequestPermission: () => Promise<void>
  onStartScreenpipe: () => Promise<void>
  onStopScreenpipe: () => Promise<void>
}

export default function ScreenpipeConnectionModal({
  isOpen,
  onClose,
  screenRecordingPermission,
  isScreenpipeRunning,
  onRequestPermission,
  onStartScreenpipe,
  onStopScreenpipe
}: ScreenpipeConnectionModalProps) {
  const [isRequestingPermission, setIsRequestingPermission] = useState(false)
  const [isStartingScreenpipe, setIsStartingScreenpipe] = useState(false)
  const [isStoppingScreenpipe, setIsStoppingScreenpipe] = useState(false)
  const [isAttemptingAutoStart, setIsAttemptingAutoStart] = useState(false)
  const [hasAttemptedAutoStart, setHasAttemptedAutoStart] = useState(false)
  const [showPermissionsUI, setShowPermissionsUI] = useState(false)

  const permissionDenied =
    screenRecordingPermission !== 'granted' && screenRecordingPermission !== 'not-determined'
  const needsScreenpipe = !isScreenpipeRunning

  // Attempt to start Screenpipe automatically when modal opens
  useEffect(() => {
    if (isOpen && needsScreenpipe && !hasAttemptedAutoStart && !isAttemptingAutoStart) {
      const attemptAutoStart = async () => {
        setIsAttemptingAutoStart(true)
        setHasAttemptedAutoStart(true)

        try {
          // Try to start Screenpipe
          await onStartScreenpipe()
          // If successful and permissions are granted, the modal will close automatically
        } catch (error) {
          console.error('[ScreenpipeConnectionModal] Auto-start failed:', error)
          // If starting fails, show the permissions UI
          setShowPermissionsUI(true)
        } finally {
          setIsAttemptingAutoStart(false)
        }
      }

      attemptAutoStart()
    }
  }, [isOpen, needsScreenpipe, hasAttemptedAutoStart, isAttemptingAutoStart, onStartScreenpipe])

  // Reset state when modal closes
  useEffect(() => {
    if (!isOpen) {
      setHasAttemptedAutoStart(false)
      setShowPermissionsUI(false)
    }
  }, [isOpen])

  // Close modal when Screenpipe is running and permissions are granted
  useEffect(() => {
    if (isScreenpipeRunning && screenRecordingPermission === 'granted' && isOpen) {
      onClose()
    }
  }, [isScreenpipeRunning, screenRecordingPermission, isOpen, onClose])

  // Show permissions UI if permission is denied after attempting to start
  useEffect(() => {
    if (hasAttemptedAutoStart && permissionDenied && !showPermissionsUI) {
      setShowPermissionsUI(true)
    }
  }, [hasAttemptedAutoStart, permissionDenied, showPermissionsUI])

  const handleRequestPermission = async () => {
    setIsRequestingPermission(true)
    try {
      await onRequestPermission()
      // After requesting permission, the app will restart
      // The modal will close automatically when the component unmounts
    } catch (error) {
      console.error('Error requesting permission:', error)
      toast.error('Failed to request permission')
    } finally {
      setIsRequestingPermission(false)
    }
  }

  const handleStartScreenpipe = async () => {
    setIsStartingScreenpipe(true)
    try {
      await onStartScreenpipe()
      // If successful, close the modal
      // The modal will close automatically via the useEffect that watches isScreenpipeRunning
    } catch (error) {
      console.error('Error starting Screenpipe:', error)
      const errorMessage = error instanceof Error ? error.message : 'Failed to start Screenpipe'

      // Only show toast for non-permission errors
      if (!errorMessage.toLowerCase().includes('permission')) {
        toast.error(errorMessage)
      }

      // Show permissions UI if we haven't already
      if (!showPermissionsUI) {
        setShowPermissionsUI(true)
      }
    } finally {
      setIsStartingScreenpipe(false)
    }
  }

  const handleStopScreenpipe = async () => {
    setIsStoppingScreenpipe(true)
    try {
      await onStopScreenpipe()
      toast.success('Screenpipe stopped successfully')
      // Optionally close modal after stopping
      // onClose()
    } catch (error) {
      console.error('Error stopping Screenpipe:', error)
      toast.error('Failed to stop Screenpipe')
    } finally {
      setIsStoppingScreenpipe(false)
    }
  }

  if (!isOpen) return null

  return (
    <Dialog open={isOpen} onOpenChange={onClose}>
      <DialogContent className="sm:max-w-md border-none bg-background">
        <DialogHeader className="space-y-1">
          <DialogTitle className="flex items-center gap-2 text-lg font-medium">
            <Monitor className="h-5 w-5 text-primary" />
            Screenpipe Setup
          </DialogTitle>
          <DialogDescription className="text-sm text-muted-foreground">
            Enable screen awareness for AI context
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-6 py-4">
          {/* Show loading state while attempting to start (either auto or manual) */}
          {(isAttemptingAutoStart || isStartingScreenpipe) && (
            <motion.div
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.3 }}
              className="flex flex-col items-center justify-center py-8"
            >
              <Loader2 className="h-8 w-8 animate-spin text-primary mb-4" />
              <p className="text-sm text-muted-foreground">Starting Screenpipe...</p>
            </motion.div>
          )}

          {/* Only show permissions UI if not currently starting and (auto-start failed or permissions are denied) */}
          {!isAttemptingAutoStart &&
            !isStartingScreenpipe &&
            (showPermissionsUI || permissionDenied) && (
              <>
                {/* Step 1: Permission */}
                <motion.div
                  initial={{ opacity: 0, y: 20 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ duration: 0.3 }}
                  className={`p-4 rounded-xl border ${permissionDenied ? 'border-border' : 'border-green-200 dark:border-green-800'}`}
                >
                  <div className="flex items-start gap-3">
                    <div
                      className={`p-2 rounded-lg border ${permissionDenied ? 'border-border' : 'border-green-200 dark:border-green-800'}`}
                    >
                      {permissionDenied ? (
                        <Shield className="h-5 w-5 text-muted-foreground" />
                      ) : (
                        <CheckCircle2 className="h-5 w-5 text-green-600 dark:text-green-400" />
                      )}
                    </div>
                    <div className="flex-1">
                      <h3 className="text-sm font-medium mb-2">Step 1: Screen Permission</h3>
                      <p className="text-xs text-muted-foreground mb-4">
                        {permissionDenied
                          ? 'Screen recording permission was denied. Grant access to continue.'
                          : 'Permission granted'}
                      </p>
                      {permissionDenied && (
                        <Button
                          onClick={handleRequestPermission}
                          disabled={isRequestingPermission}
                          size="sm"
                          className="text-sm"
                        >
                          <Settings className="h-4 w-4 mr-2" />
                          {isRequestingPermission ? 'Requesting...' : 'Open System Settings'}
                        </Button>
                      )}
                    </div>
                  </div>
                </motion.div>

                {/* Step 2: Start Screenpipe */}
                <motion.div
                  initial={{ opacity: 0, y: 20 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ duration: 0.3, delay: 0.1 }}
                  className={`p-4 rounded-xl border ${needsScreenpipe ? (permissionDenied ? 'border-border bg-transparent' : 'border-border') : 'border-green-200 dark:border-green-800'}`}
                >
                  <div className="flex items-start gap-3">
                    <div
                      className={`p-2 rounded-lg border ${needsScreenpipe ? (permissionDenied ? 'border-border' : 'border-border') : 'border-green-200 dark:border-green-800'}`}
                    >
                      {needsScreenpipe ? (
                        <Zap
                          className={`h-5 w-5 ${permissionDenied ? 'text-muted-foreground' : 'text-muted-foreground'}`}
                        />
                      ) : (
                        <CheckCircle2 className="h-5 w-5 text-green-600 dark:text-green-400" />
                      )}
                    </div>
                    <div className="flex-1">
                      <h3 className="text-sm font-medium mb-2">Step 2: Start Service</h3>
                      <p className="text-xs text-muted-foreground mb-4">
                        {needsScreenpipe ? 'Launch background recording' : 'Service running'}
                      </p>
                      {needsScreenpipe ? (
                        <Button
                          onClick={handleStartScreenpipe}
                          disabled={isStartingScreenpipe || permissionDenied}
                          size="sm"
                          className="text-sm"
                          variant={permissionDenied ? 'outline' : 'default'}
                        >
                          <Play className="h-4 w-4 mr-2" />
                          {isStartingScreenpipe ? 'Starting...' : 'Retry Start'}
                        </Button>
                      ) : (
                        <Button
                          variant="outline"
                          onClick={handleStopScreenpipe}
                          size="sm"
                          disabled={isStoppingScreenpipe}
                        >
                          <StopCircleIcon className="h-4 w-4 mr-2" />
                          {isStoppingScreenpipe ? 'Stopping...' : 'Stop Screenpipe'}
                        </Button>
                      )}
                    </div>
                  </div>
                </motion.div>

                {permissionDenied && (
                  <motion.div
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    transition={{ duration: 0.3, delay: 0.2 }}
                    className="p-3 rounded-lg bg-background border border-border text-xs text-muted-foreground"
                  >
                    After granting permission, the app will restart to apply changes.
                  </motion.div>
                )}
              </>
            )}

          {/* Show success state if Screenpipe is running */}
          {!isAttemptingAutoStart &&
            !isStartingScreenpipe &&
            isScreenpipeRunning &&
            !permissionDenied && (
              <motion.div
                initial={{ opacity: 0, y: 20 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.3 }}
                className="flex flex-col items-center justify-center py-8"
              >
                <CheckCircle2 className="h-12 w-12 text-green-600 dark:text-green-400 mb-4" />
                <p className="text-sm font-medium mb-2">Screenpipe is running</p>
                <p className="text-xs text-muted-foreground">
                  Successfully capturing screen activity
                </p>
              </motion.div>
            )}
        </div>

        <div className="flex justify-end gap-2 pt-4">
          <Button variant="ghost" size="sm" onClick={onClose}>
            Cancel
          </Button>
          {!permissionDenied && !needsScreenpipe && (
            <Button size="sm" onClick={onClose}>
              <CheckCircle2 className="h-4 w-4 mr-2" />
              Done
            </Button>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
