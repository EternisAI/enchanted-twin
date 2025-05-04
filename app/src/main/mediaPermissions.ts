import { ipcMain, systemPreferences, shell, WebContents, Session } from 'electron'

export type MediaType = 'camera' | 'microphone' | 'screen'

export function registerMediaPermissionHandlers(ses: Session) {
  ses.setPermissionRequestHandler((_wc: WebContents, permission, callback, details) => {
    if (permission === 'media') {
      const mediaDetails = details as unknown
      const hasMediaTypes =
        mediaDetails && typeof mediaDetails === 'object' && 'mediaTypes' in mediaDetails
      const mediaTypes = hasMediaTypes
        ? (mediaDetails as { mediaTypes: string[] }).mediaTypes
        : undefined

      const isPermissionGranted =
        !mediaTypes || mediaTypes.some((type: string) => ['video', 'audio'].includes(type))

      return callback(isPermissionGranted)
    }
    if (permission === 'display-capture') return callback(true)
    callback(false)
  })
}

export function queryMediaStatus(type: MediaType): string {
  if (process.platform === 'darwin') {
    return systemPreferences.getMediaAccessStatus(type)
  }
  return 'unavailable' // Win / Linux: no per-app switch
}

export async function requestMediaAccess(type: MediaType): Promise<string> {
  if (process.platform !== 'darwin') return 'unavailable'

  // screen permission can't be requested via askForMediaAccess so we just open settings
  if (type === 'screen') {
    const status = systemPreferences.getMediaAccessStatus(type)
    shell.openExternal(
      'x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture'
    )
    return status
  }

  let status = systemPreferences.getMediaAccessStatus(type)

  // only ask if user hasn't responded yet - If he has it won't pop up again
  if (status === 'not-determined') {
    const ok = await systemPreferences.askForMediaAccess(type)
    status = ok ? 'granted' : systemPreferences.getMediaAccessStatus(type)
  }

  // if denied, open the right Settings pane so the user can flip the switch
  if (status === 'denied' || status === 'restricted' || status === 'granted') {
    const pane = type === 'camera' ? 'Privacy_Camera' : 'Privacy_Microphone'
    shell.openExternal(`x-apple.systempreferences:com.apple.preference.security?${pane}`)
  }

  return status
}

export function registerPermissionIpc() {
  ipcMain.handle('permissions:get-status', (_e, type: MediaType) => queryMediaStatus(type))
  ipcMain.handle('permissions:request', (_e, type: MediaType) => requestMediaAccess(type))
}
