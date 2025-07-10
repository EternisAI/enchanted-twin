import { useState } from 'react'
import { Monitor, CheckCircle2, Settings, Play, Shield, Zap } from 'lucide-react'
import { Button } from '@renderer/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription
} from '@renderer/components/ui/dialog'

interface ScreenpipeConnectionModalProps {
  isOpen: boolean
  onClose: () => void
  screenRecordingPermission: 'granted' | 'denied' | 'not-determined' | 'restricted' | 'unavailable'
  isScreenpipeRunning: boolean
  onRequestPermission: () => Promise<void>
  onStartScreenpipe: () => Promise<void>
}

export default function ScreenpipeConnectionModal({
  isOpen,
  onClose,
  screenRecordingPermission,
  isScreenpipeRunning,
  onRequestPermission,
  onStartScreenpipe
}: ScreenpipeConnectionModalProps) {
  const [isRequestingPermission, setIsRequestingPermission] = useState(false)
  const [isStartingScreenpipe, setIsStartingScreenpipe] = useState(false)

  const needsPermission = screenRecordingPermission !== 'granted'
  const needsScreenpipe = !isScreenpipeRunning

  const handleRequestPermission = async () => {
    setIsRequestingPermission(true)
    try {
      await onRequestPermission()
      // After requesting permission, the app will restart
      // The modal will close automatically when the component unmounts
    } catch (error) {
      console.error('Error requesting permission:', error)
    } finally {
      setIsRequestingPermission(false)
    }
  }

  const handleStartScreenpipe = async () => {
    setIsStartingScreenpipe(true)
    try {
      await onStartScreenpipe()
      // If successful, close the modal
      if (!needsPermission) {
        onClose()
      }
    } catch (error) {
      console.error('Error starting Screenpipe:', error)
    } finally {
      setIsStartingScreenpipe(false)
    }
  }

  if (!isOpen) return null

  return (
    <Dialog open={isOpen} onOpenChange={onClose}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2 text-xl">
            <Monitor className="h-6 w-6" />
            Set up Screenpipe Connection
          </DialogTitle>
          <DialogDescription className="text-base">
            Complete these steps to enable screen recording and AI context awareness
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          {/* Step 1: Permission */}
          <div
            className={`p-4 rounded-lg border-2 transition-all ${
              needsPermission
                ? 'border-blue-200 bg-blue-50/50 dark:border-blue-800 dark:bg-blue-950/20'
                : 'border-green-200 bg-green-50/50 dark:border-green-800 dark:bg-green-950/20'
            }`}
          >
            <div className="flex items-start gap-3">
              <div
                className={`flex-shrink-0 w-8 h-8 rounded-full flex items-center justify-center ${
                  needsPermission
                    ? 'bg-blue-100 dark:bg-blue-900'
                    : 'bg-green-100 dark:bg-green-900'
                }`}
              >
                {needsPermission ? (
                  <Shield className="h-4 w-4 text-blue-600 dark:text-blue-400" />
                ) : (
                  <CheckCircle2 className="h-4 w-4 text-green-600 dark:text-green-400" />
                )}
              </div>
              <div className="flex-1">
                <h3 className="font-semibold text-sm mb-1">Screen Recording Permission</h3>
                <p className="text-sm text-muted-foreground mb-3">
                  {needsPermission
                    ? 'Allow access to capture your screen activity for AI context'
                    : 'Permission granted - ready to capture screen content'}
                </p>
                {needsPermission && (
                  <Button
                    onClick={handleRequestPermission}
                    disabled={isRequestingPermission}
                    size="sm"
                    className="w-full"
                  >
                    <Settings className="h-4 w-4 mr-2" />
                    {isRequestingPermission ? 'Opening Settings...' : 'Grant Permission'}
                  </Button>
                )}
              </div>
            </div>
          </div>

          {/* Step 2: Start Screenpipe */}
          <div
            className={`p-4 rounded-lg border-2 transition-all ${
              needsScreenpipe
                ? needsPermission
                  ? 'border-gray-200 bg-gray-50/50 dark:border-gray-700 dark:bg-gray-800/20'
                  : 'border-blue-200 bg-blue-50/50 dark:border-blue-800 dark:bg-blue-950/20'
                : 'border-green-200 bg-green-50/50 dark:border-green-800 dark:bg-green-950/20'
            }`}
          >
            <div className="flex items-start gap-3">
              <div
                className={`flex-shrink-0 w-8 h-8 rounded-full flex items-center justify-center ${
                  needsScreenpipe
                    ? needsPermission
                      ? 'bg-gray-100 dark:bg-gray-800'
                      : 'bg-blue-100 dark:bg-blue-900'
                    : 'bg-green-100 dark:bg-green-900'
                }`}
              >
                {needsScreenpipe ? (
                  <Zap
                    className={`h-4 w-4 ${
                      needsPermission
                        ? 'text-gray-400 dark:text-gray-500'
                        : 'text-blue-600 dark:text-blue-400'
                    }`}
                  />
                ) : (
                  <CheckCircle2 className="h-4 w-4 text-green-600 dark:text-green-400" />
                )}
              </div>
              <div className="flex-1">
                <h3 className="font-semibold text-sm mb-1">Start Screenpipe Service</h3>
                <p className="text-sm text-muted-foreground mb-3">
                  {needsScreenpipe
                    ? 'Launch the background service to begin screen recording'
                    : 'Service is running and capturing screen activity'}
                </p>
                {needsScreenpipe && (
                  <Button
                    onClick={handleStartScreenpipe}
                    disabled={isStartingScreenpipe || needsPermission}
                    size="sm"
                    className="w-full"
                    variant={needsPermission ? 'secondary' : 'default'}
                  >
                    <Play className="h-4 w-4 mr-2" />
                    {isStartingScreenpipe ? 'Starting Service...' : 'Start Service'}
                  </Button>
                )}
              </div>
            </div>
          </div>

          {/* Info box for permission restart */}
          {needsPermission && (
            <div className="p-3 bg-blue-50/50 dark:bg-blue-950/20 rounded-md border border-blue-200 dark:border-blue-800">
              <p className="text-sm text-blue-800 dark:text-blue-200">
                <strong>Note:</strong> After granting permission, the app will restart automatically
                and return you to this setup screen.
              </p>
            </div>
          )}
        </div>

        <div className="flex justify-between items-center pt-4">
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          {!needsPermission && !needsScreenpipe && (
            <Button onClick={onClose} className="bg-green-600 hover:bg-green-700">
              <CheckCircle2 className="h-4 w-4 mr-2" />
              All Set!
            </Button>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
