import { ElectronAPI } from '@electron-toolkit/preload'

interface FileDialogResult {
  canceled: boolean
  filePaths: string[]
  fileSizes?: number[]
}

interface WindowAPI {
  selectFiles: (options?: { filters?: { name: string; extensions: string[] }[] }) => Promise<FileDialogResult>
  // Other API methods will be typed as needed
  [key: string]: any
}

declare global {
  interface Window {
    electron: ElectronAPI
    api: WindowAPI
  }
}
