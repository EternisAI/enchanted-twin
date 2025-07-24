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
import { motion } from 'framer-motion'

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

  const permissionDenied =
    screenRecordingPermission !== 'granted' && screenRecordingPermission !== 'not-determined'
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
      if (!permissionDenied) {
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
                    ? 'Grant access to capture screen activity'
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
                    {isRequestingPermission ? 'Requesting...' : 'Grant Access'}
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
                {needsScreenpipe && (
                  <Button
                    onClick={handleStartScreenpipe}
                    disabled={isStartingScreenpipe || permissionDenied}
                    size="sm"
                    className="text-sm"
                    variant={permissionDenied ? 'outline' : 'default'}
                  >
                    <Play className="h-4 w-4 mr-2" />
                    {isStartingScreenpipe ? 'Starting...' : 'Start Now'}
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
