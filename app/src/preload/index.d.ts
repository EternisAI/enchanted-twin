import { ElectronAPI } from '@electron-toolkit/preload'

interface IApi {
  getPathForFile: (file: File) => string
  copyDroppedFiles: (paths: string[]) => Promise<string[]>
  selectDirectory: () => Promise<Electron.OpenDialogReturnValue>
  selectFiles: (options?: {
    filters?: { name: string; extensions: string[] }[]
  }) => Promise<Electron.OpenDialogReturnValue>
  getNativeTheme: () => Promise<string>
  setNativeTheme: (theme: 'system' | 'light' | 'dark') => Promise<string>
  onNativeThemeUpdated: (callback: (theme: 'light' | 'dark') => void) => void
  openOAuthUrl: (url: string, redirectUri?: string) => void
  onOAuthCallback: (callback: (data: { state: string; code: string }) => void) => void
  openLogsFolder: () => Promise<boolean>
  openAppDataFolder: () => Promise<boolean>
  deleteAppData: () => Promise<boolean>
  isPackaged: () => Promise<boolean>
  restartApp: () => Promise<void>
  notify: (notification: any) => Promise<void>
  openUrl: (url: string) => void
  onDeepLink: (callback: (url: string) => void) => void
  getNotificationStatus: () => Promise<any>
  openSettings: () => Promise<void>
  queryMediaStatus: (type: any) => Promise<any>
  requestMediaAccess: (type: any) => Promise<any>
  accessibility: {
    getStatus: () => Promise<any>
    request: () => Promise<any>
  }
  checkForUpdates: (silent?: boolean) => Promise<boolean>
  onUpdateStatus: (callback: (status: string) => void) => () => void
  onUpdateProgress: (callback: (progress: unknown) => void) => () => void
  getAppVersion: () => Promise<string>
  onOpenSettings: (callback: () => void) => () => void
  screenpipe: {
    getStatus: () => Promise<any>
    install: () => Promise<any>
    start: () => Promise<any>
    stop: () => Promise<any>
    getAutoStart: () => Promise<boolean>
    setAutoStart: (enabled: boolean) => Promise<any>
  }
  launch: {
    onProgress: (callback: (data: {
      dependency: string
      status: string
      progress: number
      error?: string
    }) => void) => () => void
    notifyReady: () => void
    complete: () => void
    getCurrentState: () => Promise<any>
  }
  onLaunch: (
    channel: 'launch-complete' | 'launch-progress',
    callback: (data: { dependency: string; status: string; progress: number; error?: string } | void) => void
  ) => void
  analytics: {
    capture: (event: string, properties: Record<string, unknown>) => Promise<void>
    identify: (properties: Record<string, unknown>) => Promise<void>
    getDistinctId: () => Promise<string>
    getEnabled: () => Promise<boolean>
    setEnabled: (enabled: boolean) => Promise<void>
  }
  voiceStore: {
    get: (key: string) => any
    set: (key: string, value: unknown) => void
  }
  screenpipeStore: {
    get: (key: string) => any
    set: (key: string, value: unknown) => void
  }
  livekit: {
    setup: () => Promise<any>
    start: (chatId: string, isOnboarding?: boolean) => Promise<any>
    stop: () => Promise<any>
    isRunning: () => Promise<boolean>
    isSessionReady: () => Promise<boolean>
    getState: () => Promise<any>
    mute: () => Promise<boolean>
    unmute: () => Promise<boolean>
    getAgentState: () => Promise<string>
    onSessionStateChange: (callback: (data: { sessionReady: boolean }) => void) => () => void
    onAgentStateChange: (callback: (data: { state: string }) => void) => () => void
  }
  onGoLog: (callback: (data: { source: 'stdout' | 'stderr'; line: string }) => void) => () => void
  openMainWindowWithChat: (chatId?: string, initialMessage?: string) => Promise<{ success: boolean; error?: string }>
  onNavigateTo: (callback: (url: string) => void) => () => void
  resizeOmnibarWindow: (width: number, height: number) => Promise<{ success: boolean; error?: string }>
  hideOmnibarWindow: () => Promise<{ success: boolean; error?: string }>
  rendererReady: () => void
  keyboardShortcuts: {
    get: () => Promise<Record<string, { keys: string; default: string }>>
    set: (action: string, keys: string) => Promise<{ success: boolean; error?: string }>
    reset: (action: string) => Promise<{ success: boolean; error?: string }>
    resetAll: () => Promise<{ success: boolean; error?: string }>
  }
}

declare global {
  interface Window {
    electron: ElectronAPI
    api: IApi
  }
}
