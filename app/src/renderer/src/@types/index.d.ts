interface IApi {
  // ... existing methods ...
  openOauthUrl: (url: string, redirectUri?: string) => void
  getNativeTheme: () => Promise<string>
  setNativeTheme: (theme: 'system' | 'light' | 'dark') => Promise<string>
  selectDirectory: () => Promise<Electron.OpenDialogReturnValue>
  selectFiles: (options?: { filters?: { name: string; extensions: string[] }[] }) => Promise<Electron.OpenDialogReturnValue>
  copyDroppedFiles: (filePaths: string[]) => Promise<string[]>
  getStoredFilesPath: () => Promise<string>
  restartApp: () => Promise<void>
  openLogsFolder: () => Promise<boolean>
  openAppDataFolder: () => Promise<boolean>
  deleteAppData: () => Promise<boolean>
  isPackaged: () => Promise<boolean>
  
  // Update related methods
  checkForUpdates: (silent?: boolean) => Promise<boolean>
  onUpdateStatus: (callback: (status: string) => void) => () => void
  onUpdateProgress: (callback: (progress: any) => void) => () => void
} 