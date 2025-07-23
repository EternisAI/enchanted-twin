import { useState, useEffect } from 'react'

export function useAppName() {
  const [buildChannel, setBuildChannel] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    async function fetchBuildChannel() {
      try {
        const channel = await window.api.getBuildChannel()
        setBuildChannel(channel)
      } catch (error) {
        console.error('Failed to get build channel:', error)
        setBuildChannel('latest')
      } finally {
        setLoading(false)
      }
    }

    fetchBuildChannel()
  }, [])

  return {
    buildChannel,
    loading,
    isDevRelease: buildChannel === 'dev'
  }
}
