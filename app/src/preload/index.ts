import { contextBridge, ipcRenderer, webUtils } from 'electron'
import { electronAPI } from '@electron-toolkit/preload'

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
  deleteDBData: () => ipcRenderer.invoke('delete-db-data'),
  isPackaged: () => ipcRenderer.invoke('isPackaged'),
  restartApp: () => ipcRenderer.invoke('restart-app')
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
