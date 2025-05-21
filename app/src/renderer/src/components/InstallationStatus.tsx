import { useEffect, useState } from 'react'

interface InstallationStatus {
  dependency: string
  progress: number
  status: string
  error?: string
}

export default function InstallationStatus() {
  const [installationStatus, setInstallationStatus] = useState<InstallationStatus>({
    dependency: 'Kokoro',
    progress: 0,
    status: 'Not started'
  })

  const fetchCurrentState = async () => {
    try {
      const currentState = await window.api.launch.getCurrentState()
      if (currentState) {
        setInstallationStatus(currentState)
      }
    } catch (error) {
      console.error('Failed to fetch current state:', error)
    }
  }

  useEffect(() => {
    fetchCurrentState()

    const removeListener = window.api.launch.onProgress((data) => {
      console.log('Launch progress update received:', data)
      setInstallationStatus(data)
    })

    window.api.launch.notifyReady()

    return () => {
      removeListener()
    }
  }, [])

  return (
    <div className="flex flex-col gap-4 rounded-lg border bg-card p-6 shadow-sm">
      <h3 className="text-xl font-semibold mb-4">Dependencies</h3>
      <div className="flex flex-col gap-2">
        <div className="flex justify-between text-sm">
          <span>
            {installationStatus.dependency}: {installationStatus.status}
          </span>
          <span>{installationStatus.progress}%</span>
        </div>
        <div className="w-full h-2 bg-gray-200 rounded-full overflow-hidden">
          <div
            className={`h-full transition-all duration-300 ease-in-out ${
              installationStatus.error ? 'bg-destructive' : 'bg-primary'
            }`}
            style={{ width: `${installationStatus.progress}%` }}
          />
        </div>
        {installationStatus.error && (
          <div className="text-sm text-destructive mt-2">Error: {installationStatus.error}</div>
        )}
      </div>
    </div>
  )
}
