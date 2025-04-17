/// <reference types="vite/client" />

export {}

declare global {
  interface Window {
    api: {
      getPathForFile: (file: File) => string
      copyDroppedFiles: (paths: string[]) => Promise<string[]>
    }
  }
}
