import { contextBridge, ipcRenderer, webUtils } from 'electron'
import { electronAPI } from '@electron-toolkit/preload'

const api = {
  getPathForFile: (file) => webUtils.getPathForFile(file),
  copyDroppedFiles: (paths: string[]) => ipcRenderer.invoke('copy-dropped-files', paths),
  selectDirectory: () => ipcRenderer.invoke('select-directory'),
  selectFiles: () => ipcRenderer.invoke('select-files'),
  getNativeTheme: () => ipcRenderer.invoke('get-native-theme'),
  setNativeTheme: (theme: 'system' | 'light' | 'dark') => ipcRenderer.invoke('set-native-theme', theme),
  onNativeThemeUpdated: (callback: (theme: 'light' | 'dark') => void) => {
    ipcRenderer.on('native-theme-updated', (_, theme) => callback(theme))
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
