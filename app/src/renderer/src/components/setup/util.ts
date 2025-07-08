export const initialDownloadState = {
  embeddings: {
    downloading: false,
    percentage: 0,
    completed: false,
    totalBytes: 0,
    downloadedBytes: 0
  },
  anonymizer: {
    downloading: false,
    percentage: 0,
    completed: false,
    totalBytes: 0,
    downloadedBytes: 0
  },
  onnx: {
    downloading: false,
    percentage: 0,
    completed: false,
    totalBytes: 0,
    downloadedBytes: 0
  }
}

export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'

  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))

  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))}${sizes[i]}`
}
