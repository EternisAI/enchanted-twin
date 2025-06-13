import { useEffect, useState } from 'react'

interface InstallationStatus {
  dependency: string
  progress: number
  status: string
  error?: string
}

export default function useDependencyStatus() {
  const [installationStatus, setInstallationStatus] = useState<InstallationStatus>({
    dependency: 'TTS',
    progress: 0,
    status: 'Not started'
  })
  const [isLiveKitSessionReady, setIsLiveKitSessionReady] = useState(false)

  const fetchCurrentState = async () => {
    try {
      const currentState = await window.api.launch.getCurrentState()
      if (currentState) {
        setInstallationStatus(currentState)
      }

      const sessionReady = await window.api.livekit.isSessionReady()
      setIsLiveKitSessionReady(sessionReady)
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

    const cleanupSessionState = window.api.livekit.onSessionStateChange((data) => {
      setIsLiveKitSessionReady(data.sessionReady)
    })

    window.api.launch.notifyReady()

    return () => {
      removeListener()
      cleanupSessionState()
    }
  }, [])

  const isVoiceReady =
    installationStatus.status?.toLowerCase() === 'ready' || installationStatus.progress === 100

  return { installationStatus, isVoiceReady, isLiveKitSessionReady }
}
