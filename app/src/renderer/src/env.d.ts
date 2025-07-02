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
  restartApp: () => Promise<void>
  notify: (notification: { id?: string; title?: string; message?: string }) => void
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
  onOpenSettings: (callback: () => void) => () => void
  onNewChat: (callback: () => void) => () => void
  onToggleSidebar: (callback: () => void) => () => void
  screenpipe: {
    getStatus: () => Promise<ScreenpipeStatus>
    install: () => Promise<ScreenpipeResult>
    start: () => Promise<ScreenpipeResult>
    stop: () => Promise<boolean>
    getAutoStart: () => Promise<boolean>
    setAutoStart: (enabled: boolean) => Promise<boolean>
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
  livekit: {
    setup: () => Promise<{ success: boolean; error?: string }>
    start: (chatId: string, isOnboarding?: boolean) => Promise<{ success: boolean; error?: string }>
    stop: () => Promise<{ success: boolean; error?: string }>
    isRunning: () => Promise<boolean>
    isSessionReady: () => Promise<boolean>
    getState: () => Promise<{
      dependency: string
      progress: number
      status: string
      error?: string
    }>
    mute: () => Promise<boolean>
    unmute: () => Promise<boolean>
    getAgentState: () => Promise<'initializing' | 'idle' | 'listening' | 'thinking' | 'speaking'>
    onSessionStateChange: (callback: (data: { sessionReady: boolean }) => void) => () => void
    onAgentStateChange: (callback: (data: { state: string }) => void) => () => void
  }
  screenpipeStore: {
    get: (key: string) => unknown
    set: (key: string, value: unknown) => void
  }
  onGoLog: (callback: (data: { source: 'stdout' | 'stderr'; line: string }) => void) => () => void
  openMainWindowWithChat: (
    chatId?: string,
    initialMessage?: string
  ) => Promise<{ success: boolean; error?: string }>
  onNavigateTo: (callback: (url: string) => void) => () => void
  resizeOmnibarWindow: (
    width: number,
    height: number
  ) => Promise<{ success: boolean; error?: string }>
  hideOmnibarWindow: () => Promise<{ success: boolean; error?: string }>
  rendererReady: () => void
  keyboardShortcuts: {
    get: () => Promise<Record<string, { keys: string; default: string; global?: boolean }>>
    set: (action: string, keys: string) => Promise<{ success: boolean; error?: string }>
    reset: (action: string) => Promise<{ success: boolean; error?: string }>
    resetAll: () => Promise<{ success: boolean; error?: string }>
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
