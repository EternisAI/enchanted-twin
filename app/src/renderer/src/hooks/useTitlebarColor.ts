import { useCallback } from 'react'

type TitlebarPreset = 'onboarding' | 'app'

interface TitlebarColors {
  light: string
  dark: string
}

const TITLEBAR_PRESETS: Record<TitlebarPreset, TitlebarColors> = {
  onboarding: {
    light: '#6068E9',
    dark: '#18181B'
  },
  app: {
    light: 'var(--background)',
    dark: 'var(--background)'
  }
}

function getCurrentTheme(): 'light' | 'dark' {
  if (document.documentElement.classList.contains('dark')) {
    return 'dark'
  }
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

export function updateNavbarColor(preset: TitlebarPreset): void {
  const titlebarElement = document.querySelector('.titlebar') as HTMLElement
  if (!titlebarElement) {
    console.warn('Titlebar element not found')
    return
  }

  const currentTheme = getCurrentTheme()
  const colorConfig = TITLEBAR_PRESETS[preset]
  const color = colorConfig[currentTheme]

  if (color) {
    titlebarElement.style.background = color
  } else {
    titlebarElement.style.background = ''
  }
}

export function resetNavbarColor(): void {
  updateNavbarColor('app')
}

export function getTitlebarPresets(): TitlebarPreset[] {
  return Object.keys(TITLEBAR_PRESETS) as TitlebarPreset[]
}

export type { TitlebarPreset, TitlebarColors }

export function useTitlebarColor() {
  const updateTitlebarColor = useCallback((preset: TitlebarPreset) => {
    updateNavbarColor(preset)
  }, [])

  const resetTitlebar = useCallback(() => {
    resetNavbarColor()
  }, [])

  return {
    updateTitlebarColor,
    resetTitlebar,
    presets: getTitlebarPresets()
  }
}
