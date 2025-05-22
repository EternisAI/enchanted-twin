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
  getPathForFile: (file: string) => string
  copyDroppedFiles: (paths: string[]) => Promise<string[]>
  selectDirectory: () => Promise<{ canceled: boolean; filePaths: string[] }>
  selectFiles: (options?: {
    filters?: { name: string; extensions: string[] }[]
  }) => Promise<{ canceled: boolean; filePaths: string[] }>
  getNativeTheme: () => Promise<'light' | 'dark'>
  setNativeTheme: (theme: 'system' | 'light' | 'dark') => Promise<'light' | 'dark'>
  onNativeThemeUpdated: (callback: (theme: 'light' | 'dark') => void) => void
  openOAuthUrl: (url: string, redirectUri?: string) => void
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
  restartApp: () => Promise<void>
  onOpenSettings: (callback: () => void) => void
  screenpipe: {
    getStatus: () => Promise<ScreenpipeStatus>
    install: () => Promise<ScreenpipeResult>
    start: () => Promise<ScreenpipeResult>
    stop: () => Promise<boolean>
  }
  launch: {
    onProgress: (
      callback: (data: {
        dependency: string
        status: string
        progress: number
        error?: string
      }) => void
    ) => () => void
    notifyReady: () => void
    complete: () => Promise<void>
    getCurrentState: () => Promise<{
      dependency: string
      status: string
      progress: number
      error?: string
    } | null>
  }
  onLaunch: (
    channel: 'launch-complete' | 'launch-progress',
    callback: (
      data: { dependency: string; status: string; progress: number; error?: string } | void
    ) => void
  ) => void
  analytics: {
    capture: (event: string, properties: Record<string, unknown>) => void
    identify: (properties: Record<string, unknown>) => void
    getDistinctId: () => string
    getEnabled: () => Promise<boolean>
    setEnabled: (enabled: boolean) => Promise<void>
  }
  voiceStore: {
    get: (key: string) => unknown
    set: (key: string, value: unknown) => void
  }
}

interface ScreenpipeStatus {
  isRunning: boolean
  isInstalled: boolean
}

interface ScreenpipeResult {
  success: boolean
  error?: string
}

declare global {
  interface Window {
    electron: IElectronAPI
    api: IApi
  }
}
