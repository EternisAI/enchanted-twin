import { useEffect } from 'react'

import { useState } from 'react'

interface InstallationStatus {
  dependency: string
  progress: number
  status: string
  error?: string
}

export default function useKokoroInstallationStatus() {
  const [installationStatus, setInstallationStatus] = useState<InstallationStatus>({
    dependency: 'TTS',
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

  return { installationStatus }
}
