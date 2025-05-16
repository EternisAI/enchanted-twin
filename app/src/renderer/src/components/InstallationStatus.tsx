import { useEffect, useState } from 'react'

interface InstallationStatus {
  progress: number
  status: string
}

export default function InstallationStatus() {
  const [installationStatus, setInstallationStatus] = useState<InstallationStatus>({
    progress: 0,
    status: 'Not started'
  })

  useEffect(() => {
    window.api.launch.notifyReady()

    const removeListener = window.api.launch.onProgress((data) => {
      console.log('Launch progress update:', data)
      setInstallationStatus(data)
    })

    return () => {
      removeListener()
    }
  }, [])

  return (
    <div className="flex flex-col gap-4 rounded-lg border bg-card p-6 shadow-sm">
      <h3 className="text-xl font-semibold mb-4">Dependecies</h3>
      <div className="flex flex-col gap-2">
        <div className="flex justify-between text-sm">
          <span>{installationStatus.status}</span>
          <span>{installationStatus.progress}%</span>
        </div>
        <div className="w-full h-2 bg-gray-200 rounded-full overflow-hidden">
          <div
            className="h-full bg-primary transition-all duration-300 ease-in-out"
            style={{ width: `${installationStatus.progress}%` }}
          />
        </div>
      </div>
    </div>
  )
}
