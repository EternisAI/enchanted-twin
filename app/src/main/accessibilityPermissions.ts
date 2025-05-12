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

  if (currentStatus === 'denied') {
    systemPreferences.isTrustedAccessibilityClient(true)

    shell.openExternal(
      'x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility'
    )
  }

  return currentStatus
}

export function registerAccessibilityIpc(): void {
  ipcMain.handle('accessibility:get-status', () => queryAccessibilityStatus())
  ipcMain.handle('accessibility:request', () => requestAccessibilityAccess())
}
