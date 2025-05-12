import { contextBridge, ipcRenderer, webUtils } from 'electron'
import { electronAPI } from '@electron-toolkit/preload'
import { AppNotification } from '../renderer/src/graphql/generated/graphql'
import { MediaType } from '../main/mediaPermissions'

const api = {
  getPathForFile: (file) => webUtils.getPathForFile(file),
  copyDroppedFiles: (paths: string[]) => ipcRenderer.invoke('copy-dropped-files', paths),
  selectDirectory: () => ipcRenderer.invoke('select-directory'),
  selectFiles: (options?: { filters?: { name: string; extensions: string[] }[] }) =>
    ipcRenderer.invoke('select-files', options),
  getNativeTheme: () => ipcRenderer.invoke('get-native-theme'),
  setNativeTheme: (theme: 'system' | 'light' | 'dark') =>
    ipcRenderer.invoke('set-native-theme', theme),
  onNativeThemeUpdated: (callback: (theme: 'light' | 'dark') => void) => {
    ipcRenderer.on('native-theme-updated', (_, theme) => callback(theme))
  },
  openOAuthUrl: (url: string, redirectUri?: string) => {
    console.log('openOAuthUrl', url, redirectUri)
    ipcRenderer.send('open-oauth-url', url, redirectUri)
  },
  onOAuthCallback: (callback: (data: { state: string; code: string }) => void) => {
    console.log('onOAuthCallback called with callback!')
    ipcRenderer.on('oauth-callback', (_, data) => callback(data))
  },
  openLogsFolder: () => ipcRenderer.invoke('open-logs-folder'),
  openAppDataFolder: () => ipcRenderer.invoke('open-appdata-folder'),
  deleteAppData: () => ipcRenderer.invoke('delete-app-data'),
  isPackaged: () => ipcRenderer.invoke('isPackaged'),
  restartApp: () => ipcRenderer.invoke('restart-app'),
  notify: (notification: AppNotification) => ipcRenderer.invoke('notify', notification),
  openUrl: (url: string) => ipcRenderer.send('open-url', url),
  onDeepLink: (cb: (url: string) => void) =>
    ipcRenderer.on('open-deeplink', (_evt, url) => cb(url)),
  getNotificationStatus: () => ipcRenderer.invoke('notification-status'),
  openSettings: () => ipcRenderer.invoke('open-notification-settings'),
  queryMediaStatus: (type: MediaType) => ipcRenderer.invoke('permissions:get-status', type),
  requestMediaAccess: (type: MediaType) => ipcRenderer.invoke('permissions:request', type),
  accessibility: {
    getStatus: () => ipcRenderer.invoke('accessibility:get-status'),
    request: () => ipcRenderer.invoke('accessibility:request')
  },
  checkForUpdates: (silent: boolean = false) => ipcRenderer.invoke('check-for-updates', silent),
  onUpdateStatus: (callback: (status: string) => void) => {
    const listener = (_: unknown, status: string) => callback(status)
    ipcRenderer.on('update-status', listener)
    return () => ipcRenderer.removeListener('update-status', listener)
  },
  onUpdateProgress: (callback: (progress: unknown) => void) => {
    const listener = (_: unknown, progress: unknown) => callback(progress)
    ipcRenderer.on('update-progress', listener)
    return () => ipcRenderer.removeListener('update-progress', listener)
  },
  getAppVersion: () => ipcRenderer.invoke('get-app-version'),
  screenpipe: {
    getStatus: () => ipcRenderer.invoke('screenpipe:get-status'),
    start: () => ipcRenderer.invoke('screenpipe:start'),
    stop: () => ipcRenderer.invoke('screenpipe:stop')
  }
}

// Use `contextBridge` APIs to expose Electron APIs to
// renderer only if context isolation is enabled, otherwise
// just add to the DOM global.
if (process.contextIsolated) {
  try {
    contextBridge.exposeInMainWorld('electron', electronAPI)
    contextBridge.exposeInMainWorld('api', api)
  } catch (error) {
    console.error(error)
  }
} else {
  // @ts-ignore (define in dts)
  window.electron = electronAPI
  // @ts-ignore (define in dts)
  window.api = api
}
