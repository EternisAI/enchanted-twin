import { contextBridge, ipcRenderer, webUtils } from 'electron'
import { electronAPI } from '@electron-toolkit/preload'
import { AppNotification } from '../renderer/src/graphql/generated/graphql'
import { MediaType } from '../main/mediaPermissions'
import { voiceStore, screenpipeStore } from '../main/stores'
import { ModelName } from '../main/types/models'

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
  onThemeChanged: (callback: (theme: 'system' | 'light' | 'dark') => void) => {
    const listener = (_: unknown, theme: 'system' | 'light' | 'dark') => callback(theme)
    ipcRenderer.on('theme-changed', listener)
    return () => ipcRenderer.removeListener('theme-changed', listener)
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
    request: () => ipcRenderer.invoke('accessibility:request'),
    openSettings: () => ipcRenderer.invoke('accessibility:open-settings')
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
  getBuildChannel: () => ipcRenderer.invoke('get-build-channel'),
  onOpenSettings: (callback: () => void) => {
    const listener = () => callback()
    ipcRenderer.on('open-settings', listener)
    return () => ipcRenderer.removeListener('open-settings', listener)
  },
  onNewChat: (callback: () => void) => {
    const listener = () => callback()
    ipcRenderer.on('new-chat', listener)
    return () => ipcRenderer.removeListener('new-chat', listener)
  },
  onToggleSidebar: (callback: () => void) => {
    const listener = () => callback()
    ipcRenderer.on('toggle-sidebar', listener)
    return () => ipcRenderer.removeListener('toggle-sidebar', listener)
  },
  getEnvVar: (key: string) => ipcRenderer.invoke('get-env-var', key),
  screenpipe: {
    getStatus: () => ipcRenderer.invoke('screenpipe:get-status'),
    install: () => ipcRenderer.invoke('screenpipe:install'),
    start: () => ipcRenderer.invoke('screenpipe:start'),
    stop: () => ipcRenderer.invoke('screenpipe:stop'),
    getAutoStart: () => ipcRenderer.invoke('screenpipe:get-auto-start'),
    setAutoStart: (enabled: boolean) => ipcRenderer.invoke('screenpipe:set-auto-start', enabled),
    storeRestartIntent: (route: string, showModal: boolean) =>
      ipcRenderer.invoke('screenpipe:store-restart-intent', route, showModal)
  },
  launch: {
    onProgress: (
      callback: (data: {
        dependency: string
        status: string
        progress: number
        error?: string
      }) => void
    ) => {
      const listener = (
        _: unknown,
        data: { dependency: string; status: string; progress: number; error?: string }
      ) => callback(data)
      ipcRenderer.on('launch-progress', listener)
      return () => ipcRenderer.removeListener('launch-progress', listener)
    },
    notifyReady: () => ipcRenderer.send('launch-ready'),
    complete: () => ipcRenderer.send('launch-complete'),
    getCurrentState: () => ipcRenderer.invoke('launch-get-current-state')
  },
  onLaunch: (
    channel: 'launch-complete' | 'launch-progress',
    callback: (
      data: { dependency: string; status: string; progress: number; error?: string } | void
    ) => void
  ) => {
    ipcRenderer.on(channel, (_, data) => callback(data))
  },
  analytics: {
    capture: (event: string, properties: Record<string, unknown>) =>
      ipcRenderer.invoke('analytics:capture', event, properties),
    captureFeedback: (event: string, properties: Record<string, unknown>) =>
      ipcRenderer.invoke('analytics:capture-feedback', event, properties),
    identify: (properties: Record<string, unknown>) =>
      ipcRenderer.invoke('analytics:identify', properties),
    getDistinctId: () => ipcRenderer.invoke('analytics:get-distinct-id'),
    getEnabled: () => ipcRenderer.invoke('analytics:is-enabled'),
    setEnabled: (enabled: boolean) => ipcRenderer.invoke('analytics:set-enabled', enabled)
  },
  voiceStore: {
    get: (key: string) => voiceStore.get(key),
    set: (key: string, value: unknown) => voiceStore.set(key, value)
  },
  screenpipeStore: {
    get: (key: string) => screenpipeStore.get(key),
    set: (key: string, value: unknown) => screenpipeStore.set(key, value)
  },
  livekit: {
    start: (chatId: string, isOnboarding?: boolean, jwtToken?: string) =>
      ipcRenderer.invoke('livekit:start', chatId, isOnboarding, jwtToken),
    stop: () => ipcRenderer.invoke('livekit:stop'),
    isRunning: () => ipcRenderer.invoke('livekit:is-running'),
    isSessionReady: () => ipcRenderer.invoke('livekit:is-session-ready'),
    getState: () => ipcRenderer.invoke('livekit:get-state'),
    mute: () => ipcRenderer.invoke('livekit:mute'),
    unmute: () => ipcRenderer.invoke('livekit:unmute'),
    getAgentState: () => ipcRenderer.invoke('livekit:get-agent-state'),
    onSessionStateChange: (callback: (data: { sessionReady: boolean }) => void) => {
      const cleanup = () => ipcRenderer.removeAllListeners('livekit-session-state')
      ipcRenderer.on('livekit-session-state', (_event, data) => callback(data))
      return cleanup
    },
    onAgentStateChange: (callback: (data: { state: string }) => void) => {
      const cleanup = () => ipcRenderer.removeAllListeners('livekit-agent-state')
      ipcRenderer.on('livekit-agent-state', (_event, data) => callback(data))
      return cleanup
    }
  },
  onGoLog: (callback: (data: { source: 'stdout' | 'stderr'; line: string }) => void) => {
    const batchLogListener = (
      _: unknown,
      logs: Array<{ source: 'stdout' | 'stderr'; line: string; timestamp: number }>
    ) => {
      logs.forEach((log) => callback({ source: log.source, line: log.line }))
    }

    ipcRenderer.on('go-logs-batch', batchLogListener)

    return () => {
      ipcRenderer.removeListener('go-logs-batch', batchLogListener)
    }
  },
  openMainWindowWithChat: (chatId?: string, initialMessage?: string, reasoning?: boolean) =>
    ipcRenderer.invoke('open-main-window-with-chat', chatId, initialMessage, reasoning),
  onNavigateTo: (callback: (url: string) => void) => {
    const listener = (_: unknown, url: string) => callback(url)
    ipcRenderer.on('navigate-to', listener)
    return () => ipcRenderer.removeListener('navigate-to', listener)
  },
  resizeOmnibarWindow: (width: number, height: number) =>
    ipcRenderer.invoke('resize-omnibar-window', width, height),
  hideOmnibarWindow: () => ipcRenderer.invoke('hide-omnibar-window'),
  rendererReady: () => ipcRenderer.send('renderer-ready'),
  keyboardShortcuts: {
    get: () => ipcRenderer.invoke('keyboard-shortcuts:get'),
    set: (action: string, keys: string) =>
      ipcRenderer.invoke('keyboard-shortcuts:set', action, keys),
    reset: (action: string) => ipcRenderer.invoke('keyboard-shortcuts:reset', action),
    resetAll: () => ipcRenderer.invoke('keyboard-shortcuts:reset-all')
  },
  models: {
    hasModelsDownloaded: () => ipcRenderer.invoke('models:has-models-downloaded'),
    downloadModels: (modelName: ModelName) => ipcRenderer.invoke('models:download', modelName),
    onProgress: (
      callback: (data: {
        modelName: string
        pct: number
        totalBytes: number
        downloadedBytes: number
      }) => void
    ) => {
      const listener = (
        _: unknown,
        data: {
          modelName: string
          pct: number
          totalBytes: number
          downloadedBytes: number
        }
      ) => callback(data)
      ipcRenderer.on('models:progress', listener)
      return () => ipcRenderer.removeListener('models:progress', listener)
    }
  },
  dependencies: {
    download: (dependencyName: string) =>
      ipcRenderer.invoke('dependencies:download', dependencyName)
  },
  goServer: {
    initialize: () => ipcRenderer.invoke('go-server:initialize'),
    cleanup: () => ipcRenderer.invoke('go-server:cleanup'),
    getStatus: () => ipcRenderer.invoke('go-server:status')
  },
  llamacpp: {
    start: () => ipcRenderer.invoke('llamacpp:start'),
    cleanup: () => ipcRenderer.invoke('llamacpp:cleanup'),
    getStatus: () => ipcRenderer.invoke('llamacpp:status')
  },
  clipboard: {
    writeText: (text: string) => ipcRenderer.invoke('clipboard:writeText', text),
    readText: () => ipcRenderer.invoke('clipboard:readText')
  },
  tts: {
    generate: (text: string, firebaseToken: string) =>
      ipcRenderer.invoke('tts:generate', text, firebaseToken)
  }
}

// Use `contextBridge` APIs to expose Electron APIs to
// renderer only if context isolation is enabled, otherwise
// just add to the DOM global.
if (process.contextIsolated) {
  try {
    contextBridge.exposeInMainWorld('electron', electronAPI)
    contextBridge.exposeInMainWorld('api', api)
    console.log('Preload: APIs exposed via contextBridge')
  } catch (error) {
    console.error('Preload: Failed to expose APIs:', error)
  }
} else {
  // @ts-ignore (define in dts)
  window.electron = electronAPI
  // @ts-ignore (define in dts)
  window.api = api
  console.log('Preload: APIs exposed directly to window')
}
