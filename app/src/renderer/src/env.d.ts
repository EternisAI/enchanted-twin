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
  selectFiles: () => Promise<{ canceled: boolean; filePaths: string[] }>
  getNativeTheme: () => Promise<'light' | 'dark'>
  setNativeTheme: (theme: 'system' | 'light' | 'dark') => Promise<'light' | 'dark'>
  onNativeThemeUpdated: (callback: (theme: 'light' | 'dark') => void) => void
  openOAuthUrl: (url: string, redirectUri?: string) => Promise<void>
  onOAuthCallback: (callback: (data: { state: string; code: string }) => void) => void
}

declare global {
  interface Window {
    electron: IElectronAPI
    api: IApi
  }
}
