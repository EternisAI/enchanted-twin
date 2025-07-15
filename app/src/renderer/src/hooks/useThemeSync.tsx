import { useEffect } from 'react'
import { useTheme } from '@renderer/lib/theme'

/**
 * Hook that synchronizes theme changes across windows
 * Uses localStorage events and IPC to ensure all windows stay in sync
 */
export function useThemeSync() {
  const { theme, setTheme } = useTheme()

  useEffect(() => {
    // Listen for localStorage changes from other windows
    const handleStorageChange = (e: StorageEvent) => {
      if (e.key === 'theme' && e.newValue) {
        const newTheme = e.newValue as 'dark' | 'light' | 'system'
        if (newTheme !== theme) {
          setTheme(newTheme)
        }
      }
    }

    // Listen for theme changes from IPC (when another window changes theme)
    const cleanup = window.api.onThemeChanged((newTheme) => {
      if (newTheme !== theme) {
        setTheme(newTheme)
      }
    })

    window.addEventListener('storage', handleStorageChange)

    return () => {
      window.removeEventListener('storage', handleStorageChange)
      cleanup()
    }
  }, [theme, setTheme])

  return { theme, setTheme }
}
