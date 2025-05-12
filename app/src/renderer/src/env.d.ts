/// <reference types="vite/client" />

export {}

interface IElectronAPI {
  ipcRenderer: {
    send(channel: string, ...args: unknown[]): void
    on(channel: string, func: (...args: unknown[]) => void): void
    once(channel: string, func: (...args: unknown[]) => void): void
    invoke(channel: string, ...args: unknown[]): Promise<{ canceled: boolean; filePaths: string[] }>
  }
  process: {
    versions: {
      electron: string
      chrome: string
      node: string
    }
  }
}

interface IApi {
  ping: () => void
  copyDroppedFiles: (filePaths: string[]) => Promise<void>
  selectDirectory: () => Promise<{ canceled: boolean; filePaths: string[] }>
  selectFiles: (options?: {
    filters?: { name: string; extensions: string[] }[]
  }) => Promise<{ canceled: boolean; filePaths: string[] }>
  getNativeTheme: () => Promise<'light' | 'dark'>
  setNativeTheme: (theme: 'system' | 'light' | 'dark') => Promise<'light' | 'dark'>
  onNativeThemeUpdated: (callback: (theme: 'light' | 'dark') => void) => void
  openOAuthUrl: (url: string, redirectUri?: string) => Promise<void>
  onOAuthCallback: (callback: (data: { state: string; code: string }) => void) => void
  openLogsFolder: () => Promise<boolean>
  openAppDataFolder: () => Promise<boolean>
  deleteAppData: () => Promise<boolean>
  isPackaged: () => Promise<boolean>
  restartApp: () => void
  notify: (notification: AppNotification) => void
  onDeepLink: (cb: (url: string) => void) => void
  getNotificationStatus: () => Promise<string>
  openSettings: () => Promise<void>
  queryMediaStatus: (type: MediaType) => Promise<string>
  requestMediaAccess: (type: MediaType) => Promise<string>
  accessibility: {
    getStatus: () => Promise<string>
    request: () => Promise<string>
  }
  checkForUpdates: (silent?: boolean) => Promise<boolean>
  onUpdateStatus: (callback: (status: string) => void) => () => void
  onUpdateProgress: (callback: (progress: unknown) => void) => () => void
  checkForUpdates: (silent: boolean) => Promise<void>
  getAppVersion: () => Promise<string>
  screenpipe: {
    getStatus: () => Promise<boolean>
    start: () => Promise<boolean>
    stop: () => Promise<boolean>
  }
}

declare global {
  interface Window {
    electron: IElectronAPI
    api: IApi
  }
}
