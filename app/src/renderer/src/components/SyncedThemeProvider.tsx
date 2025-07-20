import { ReactNode } from 'react'
import { ThemeProvider } from '@renderer/lib/theme'
import { useThemeSync } from '@renderer/hooks/useThemeSync'

type Theme = 'dark' | 'light' | 'system'

interface SyncedThemeProviderProps {
  children: ReactNode
}

/**
 * A wrapper around ThemeProvider that includes theme synchronization across windows
 */
export function SyncedThemeProvider({ children }: SyncedThemeProviderProps) {
  // Get saved theme from localStorage
  const savedTheme = (() => {
    try {
      return (localStorage.getItem('theme') as Theme) || 'system'
    } catch {
      return 'system'
    }
  })()

  console.log('savedTheme', savedTheme)

  return (
    <ThemeProvider defaultTheme={savedTheme}>
      <ThemeProviderContent>{children}</ThemeProviderContent>
    </ThemeProvider>
  )
}

// Separate component to use the hook inside the provider
function ThemeProviderContent({ children }: { children: ReactNode }) {
  // This hook sets up the synchronization
  useThemeSync()
  return <>{children}</>
}
