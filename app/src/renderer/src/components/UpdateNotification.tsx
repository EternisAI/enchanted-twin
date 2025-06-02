/* eslint-disable @typescript-eslint/no-explicit-any */
import { useEffect, useState } from 'react'
import { toast } from 'sonner'

const UpdateNotification = () => {
  const [updateStatus, setUpdateStatus] = useState<string | null>(null)
  const [progress, setProgress] = useState<number>(0)

  useEffect(() => {
    // Register update status listener
    const removeStatusListener = window.api.onUpdateStatus((status: string) => {
      setUpdateStatus(status)

      if (status === 'Update available') {
        toast.info('A new update is available')
      } else if (status === 'Update downloaded') {
        toast.success('Update downloaded. Restart to install.')
      } else if (status.startsWith('Error')) {
        toast.error(`Update error: ${status}`)
      }
    })

    // Register update progress listener
    const removeProgressListener = window.api.onUpdateProgress((progressData: any) => {
      setProgress(progressData.percent || 0)
    })

    // Check for updates silently on component mount
    setTimeout(() => {
      window.api.checkForUpdates(true).catch(console.error)
    }, 5000)

    // Clean up listeners
    return () => {
      removeStatusListener()
      removeProgressListener()
    }
  }, [])

  if (updateStatus === 'Update downloaded') {
    return (
      <div className="fixed bottom-4 right-4 bg-green-50 dark:bg-green-900 p-4 rounded-lg shadow-lg z-50">
        <h3 className="font-medium text-green-800 dark:text-green-200">Update Ready</h3>
        <p className="text-sm text-green-700 dark:text-green-300 mt-1">
          A new version is ready to install
        </p>
        <div className="mt-2 flex space-x-2">
          <button
            onClick={() => window.api.restartApp()}
            className="px-3 py-1 bg-green-600 text-white rounded-md text-sm hover:bg-green-700"
          >
            Restart Now
          </button>
          <button
            onClick={() => setUpdateStatus(null)}
            className="px-3 py-1 bg-transparent text-green-700 dark:text-green-300 border border-green-600 rounded-md text-sm"
          >
            Later
          </button>
        </div>
      </div>
    )
  }

  if (updateStatus === 'Checking for update...' || updateStatus?.includes('downloading')) {
    return (
      <div className="fixed bottom-4 right-4 bg-blue-50 dark:bg-blue-900 p-4 rounded-lg shadow-lg z-50">
        <h3 className="font-medium text-blue-800 dark:text-blue-200">
          {updateStatus === 'Checking for update...'
            ? 'Checking for updates...'
            : 'Downloading update...'}
        </h3>
        {progress > 0 && (
          <div className="mt-2 w-full bg-blue-200 dark:bg-blue-700 rounded-full h-2">
            <div
              className="bg-blue-600 dark:bg-blue-400 h-2 rounded-full"
              style={{ width: `${progress}%` }}
            />
          </div>
        )}
      </div>
    )
  }

  return null
}

export default UpdateNotification
