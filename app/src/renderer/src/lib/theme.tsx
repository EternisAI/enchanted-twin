import { createContext, useContext, useEffect, useState } from 'react'

type Theme = 'dark' | 'light' | 'system'

type ThemeProviderProps = {
  children: React.ReactNode
  defaultTheme?: Theme
}

type ThemeProviderState = {
  theme: Theme
  setTheme: (theme: Theme) => void
}

const initialState: ThemeProviderState = {
  theme: 'system',
  setTheme: () => null
}

const ThemeProviderContext = createContext<ThemeProviderState>(initialState)

export function ThemeProvider({ children, defaultTheme = 'system', ...props }: ThemeProviderProps) {
  const [theme, setTheme] = useState<Theme>(defaultTheme)

  useEffect(() => {
    const root = window.document.documentElement

    root.classList.remove('light', 'dark')

    const updateTheme = async (newTheme: Theme) => {
      if (newTheme === 'system') {
        // Get the native theme from Electron
        const nativeTheme = await window.api.getNativeTheme()
        root.classList.add(nativeTheme)
        // Set the native theme source to system
        await window.api.setNativeTheme('system')
      } else {
        root.classList.add(newTheme)
        // Sync the native theme with our app theme
        await window.api.setNativeTheme(newTheme)
      }
    }

    // Update theme initially
    updateTheme(theme)

    // Listen for system theme changes when in system mode
    if (theme === 'system') {
      window.api.onNativeThemeUpdated((newTheme) => {
        root.classList.remove('light', 'dark')
        root.classList.add(newTheme)
      })
    }
  }, [theme])

  const value = {
    theme,
    setTheme: (theme: Theme) => {
      setTheme(theme)
      try {
        window.localStorage.setItem('theme', theme)
      } catch {
        // Ignore
      }
    }
  }

  return (
    <ThemeProviderContext.Provider {...props} value={value}>
      {children}
    </ThemeProviderContext.Provider>
  )
}

export const useTheme = () => {
  const context = useContext(ThemeProviderContext)

  if (context === undefined) throw new Error('useTheme must be used within a ThemeProvider')

  return context
}
