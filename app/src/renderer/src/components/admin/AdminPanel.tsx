import { Button } from '@renderer/components/ui/button'
import { FolderOpen, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { ContinueSetupButton } from '../ContinueSetupButton'
import { Card } from '../ui/card'

export default function AdminPanel() {
  const [isLoading, setIsLoading] = useState({
    logs: false,
    app: false,
    data: false
  })

  const openLogsFolder = async () => {
    try {
      setIsLoading((prev) => ({ ...prev, logs: true }))
      const success = await window.api.openLogsFolder()
      if (success) {
        toast.success('Logs folder opened')
      } else {
        toast.error('Could not open logs folder')
      }
    } catch (error) {
      toast.error('Failed to open logs folder')
      console.error(error)
    } finally {
      setIsLoading((prev) => ({ ...prev, logs: false }))
    }
  }

  const openAppFolder = async () => {
    try {
      setIsLoading((prev) => ({ ...prev, app: true }))
      const success = await window.api.openAppDataFolder()
      if (success) {
        toast.success('Application folder opened')
      } else {
        toast.error('Could not open application folder')
      }
    } catch (error) {
      toast.error('Failed to open application folder')
      console.error(error)
    } finally {
      setIsLoading((prev) => ({ ...prev, app: false }))
    }
  }

  const deleteAppData = async () => {
    if (confirm('Are you sure you want to delete all application data? The app will restart.')) {
      try {
        setIsLoading((prev) => ({ ...prev, data: true }))
        const result = await window.api.deleteAppData()

        if (result) {
          toast.success('Application data deleted')

          setTimeout(async () => {
            const isPackaged = await window.api.isPackaged()

            if (isPackaged) {
              window.api.restartApp()
            } else {
              toast.warning('Running in development mode. Please restart the app manually.', {
                duration: 10000
              })
              setIsLoading((prev) => ({ ...prev, data: false }))
            }
          }, 1500)
        } else {
          toast.info('No application data found to delete')
          setIsLoading((prev) => ({ ...prev, data: false }))
        }
      } catch (error) {
        toast.error('Failed to delete application data')
        console.error(error)
        setIsLoading((prev) => ({ ...prev, data: false }))
      }
    }
  }

  return (
    <div className="w-full h-full flex justify-center">
      <div className="w-4xl">
        <Card className="grid grid-cols-1 gap-4 w-full p-6">
          <Button
            variant="outline"
            className="flex items-center justify-start h-14"
            onClick={openLogsFolder}
            disabled={isLoading.logs}
          >
            <FolderOpen className="mr-2" />
            {isLoading.logs ? 'Opening...' : 'Open Logs Folder'}
          </Button>

          <Button
            variant="outline"
            className="flex items-center justify-start h-14"
            onClick={openAppFolder}
            disabled={isLoading.app}
          >
            <FolderOpen className="mr-2" />
            {isLoading.app ? 'Opening...' : 'Open Application Folder'}
          </Button>

          <Button
            variant="destructive"
            className="flex items-center justify-start h-14"
            onClick={deleteAppData}
            disabled={isLoading.data}
          >
            <Trash2 className="mr-2" />
            {isLoading.data ? 'Deleting...' : 'Delete App Data'}
          </Button>
          <ContinueSetupButton />
        </Card>
      </div>
    </div>
  )
}
