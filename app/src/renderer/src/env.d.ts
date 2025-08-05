/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_DISABLE_ONBOARDING: string
  readonly VITE_DISABLE_HOLONS: string
  readonly VITE_DISABLE_TASKS: string
  readonly VITE_DISABLE_CONNECTORS: string
  readonly VITE_DISABLE_VOICE: string
  readonly VITE_ENABLE_BROWSER: string
  readonly VITE_FIREBASE_API_KEY: string
  readonly VITE_FIREBASE_AUTH_DOMAIN: string
  readonly VITE_FIREBASE_PROJECT_ID: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

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
  getPathForFile: (file: File) => string
  copyDroppedFiles: (paths: string[]) => Promise<string[]>
  selectDirectory: () => Promise<{ canceled: boolean; filePaths: string[] }>
  selectFiles: (options?: {
    filters?: { name: string; extensions: string[] }[]
  }) => Promise<{ canceled: boolean; filePaths: string[] }>
  getNativeTheme: () => Promise<'light' | 'dark'>
  setNativeTheme: (theme: 'system' | 'light' | 'dark') => Promise<'light' | 'dark'>
  onNativeThemeUpdated: (callback: (theme: 'light' | 'dark') => void) => void
  onThemeChanged: (callback: (theme: 'system' | 'light' | 'dark') => void) => () => void
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
    openSettings: () => Promise<boolean>
  }
  checkForUpdates: (silent?: boolean) => Promise<boolean>
  onUpdateStatus: (callback: (status: string) => void) => () => void
  onUpdateProgress: (callback: (progress: unknown) => void) => () => void
  checkForUpdates: (silent: boolean) => Promise<void>
  getAppVersion: () => Promise<string>
  getBuildChannel: () => Promise<string>
  restartApp: () => Promise<void>
  onOpenSettings: (callback: () => void) => () => void
  onNewChat: (callback: () => void) => () => void
  onToggleSidebar: (callback: () => void) => () => void
  getEnvVar: (key: string) => Promise<string | null>
  screenpipe: {
    getStatus: () => Promise<ScreenpipeStatus>
    install: () => Promise<ScreenpipeResult>
    start: () => Promise<ScreenpipeResult>
    stop: () => Promise<boolean>
    getAutoStart: () => Promise<boolean>
    setAutoStart: (enabled: boolean) => Promise<boolean>
    storeRestartIntent: (route: string, showModal: boolean) => Promise<void>
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
    capture: (event: string, properties: Record<string, unknown>) => Promise<void>
    captureFeedback: (event: string, properties: Record<string, unknown>) => Promise<void>
    identify: (properties: Record<string, unknown>) => Promise<void>
    getDistinctId: () => Promise<string>
    getEnabled: () => Promise<boolean>
    setEnabled: (enabled: boolean) => Promise<void>
  }
  voiceStore: {
    get: (key: string) => unknown
    set: (key: string, value: unknown) => void
  }
  livekit: {
    setup: () => Promise<{ success: boolean; error?: string }>
    start: (
      chatId: string,
      isOnboarding?: boolean,
      jwtToken?: string
    ) => Promise<{ success: boolean; error?: string }>
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
    initialMessage?: string,
    reasoning?: boolean
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
  models: {
    hasModelsDownloaded: () => Promise<Record<DependencyName, boolean>>
    downloadModels: (modelName: DependencyName) => Promise<{ success: boolean; error?: string }>
    onProgress: (
      callback: (data: {
        modelName: string
        pct: number
        totalBytes?: number
        downloadedBytes?: number
        error?: string
      }) => void
    ) => () => void
  }
  goServer: {
    initialize: () => Promise<{ success: boolean; error?: string }>
    cleanup: () => Promise<{ success: boolean; error?: string }>
    getStatus: () => Promise<{
      success: boolean
      isRunning: boolean
      isInitializing: boolean
      message: string
    }>
  }
  clipboard: {
    readText: () => Promise<{ success: boolean; text: string; error?: string }>
    writeText: (text: string) => Promise<{ success: boolean; error?: string }>
  }
  tts: {
    generate: (
      text: string,
      firebaseToken: string
    ) => Promise<{
      success: boolean
      audioBuffer?: Buffer
      error?: string
    }>
  }
  llamacpp: {
    start: () => Promise<{ success: boolean; error?: string }>
    cleanup: () => Promise<{ success: boolean; error?: string }>
    getStatus: () => Promise<{
      success: boolean
      isRunning: boolean
      isSetup: boolean
      setupInProgress: boolean
      error?: string
    }>
  }
  invoke: (channel: string, ...args: unknown[]) => Promise<any>
  browser: {
    createSession: (
      sessionId: string,
      url: string,
      partition: string,
      securitySettings: object
    ) => Promise<{ success: boolean; error?: string }>
    destroySession: (sessionId: string) => Promise<{ success: boolean; error?: string }>
    setBounds: (
      sessionId: string,
      rect: { x: number; y: number; width: number; height: number }
    ) => void
    loadUrl: (sessionId: string, url: string) => Promise<{ success: boolean; error?: string }>
    goBack: (sessionId: string) => Promise<{ success: boolean; error?: string }>
    goForward: (sessionId: string) => Promise<{ success: boolean; error?: string }>
    reload: (sessionId: string) => Promise<{ success: boolean; error?: string }>
    stop: (sessionId: string) => Promise<{ success: boolean; error?: string }>
    onDidStartLoading: (callback: (sid: string) => void) => () => void
    onDidStopLoading: (callback: (sid: string) => void) => () => void
    onDidFailLoad: (
      callback: (
        sid: string,
        details: {
          errorCode: number
          errorDescription: string
          validatedURL: string
          isMainFrame: boolean
        }
      ) => void
    ) => () => void
    onDidNavigate: (callback: (sid: string, newUrl: string) => void) => () => void
    onPageTitleUpdated: (callback: (sid: string, title: string) => void) => () => void
    onNavigationState: (
      callback: (sid: string, state: { canGoBack: boolean; canGoForward: boolean }) => void
    ) => () => void
    onSessionUpdated: (
      callback: (
        sid: string,
        content: {
          text: string
          html: string
          metadata: { title: string; description?: string; keywords?: string[]; author?: string }
        }
      ) => void
    ) => () => void
    onNavigationOccurred: (callback: (sid: string, url: string) => void) => () => void
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
