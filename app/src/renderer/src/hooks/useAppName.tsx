import { useState, useEffect } from 'react'

export function useAppName() {
  const [appName, setAppName] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    async function fetchAppName() {
      try {
        const name = await window.api.getAppName()
        setAppName(name)
      } catch (error) {
        console.error('Failed to get app name:', error)
        setAppName('Unknown')
      } finally {
        setLoading(false)
      }
    }

    fetchAppName()
  }, [])

  return { appName, loading, isDevRelease: appName === 'Enchanted Dev' }
}
