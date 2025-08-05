import { ipcMain, systemPreferences, shell } from 'electron'

export function queryAccessibilityStatus(): string {
  if (process.platform === 'darwin') {
    return systemPreferences.isTrustedAccessibilityClient(false) ? 'granted' : 'denied'
  }
  return 'unavailable' // Non-macOS platforms don't have this permission management
}

export async function requestAccessibilityAccess(): Promise<string> {
  if (process.platform !== 'darwin') return 'unavailable'

  const currentStatus = systemPreferences.isTrustedAccessibilityClient(false) ? 'granted' : 'denied'

  if (currentStatus === 'granted') {
    return 'granted'
  }

  // This will prompt the user with the system dialog to grant accessibility permissions
  const willPrompt = systemPreferences.isTrustedAccessibilityClient(true)

  // Only open settings if the system didn't show a prompt (user previously denied)
  if (!willPrompt) {
    shell.openExternal(
      'x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility'
    )
  }

  return currentStatus
}

export async function openAccessibilitySettings(): Promise<boolean> {
  try {
    if (process.platform === 'darwin') {
      await shell.openExternal(
        'x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility'
      )
      return true
    }
    // For other platforms, we could add support later
    return false
  } catch (error) {
    console.error('Failed to open accessibility settings:', error)
    return false
  }
}

export function registerAccessibilityIpc(): void {
  ipcMain.handle('accessibility:get-status', () => queryAccessibilityStatus())
  ipcMain.handle('accessibility:request', () => requestAccessibilityAccess())
  ipcMain.handle('accessibility:open-settings', () => openAccessibilitySettings())
}
