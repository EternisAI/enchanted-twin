import { useEffect, useRef } from 'react'
import { useTheme } from '@renderer/lib/theme'

/**
 * Hook that synchronizes theme changes across windows
 * Uses localStorage events and IPC to ensure all windows stay in sync
 */
export function useThemeSync() {
  const { theme, setTheme } = useTheme()
  const isSetting = useRef(false)

  useEffect(() => {
    // Listen for localStorage changes from other windows
    const handleStorageChange = (e: StorageEvent) => {
      if (e.key === 'theme' && e.newValue) {
        const newTheme = e.newValue as 'dark' | 'light' | 'system'
        if (newTheme !== theme && !isSetting.current) {
          isSetting.current = true
          setTheme(newTheme)
          isSetting.current = false
        }
      }
    }

    // Listen for theme changes from IPC (when another window changes theme)
    const cleanup = window.api.onThemeChanged((newTheme) => {
      if (newTheme !== theme && !isSetting.current) {
        isSetting.current = true
        setTheme(newTheme)
        isSetting.current = false
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
