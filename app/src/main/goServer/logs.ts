import { BrowserWindow } from 'electron'

export interface LogEntry {
  source: 'stdout' | 'stderr'
  line: string
  timestamp: number
}

let logBatch: LogEntry[] = []
let logBatchTimeout: NodeJS.Timeout | null = null
const LOG_BATCH_SIZE = 10
const LOG_BATCH_TIMEOUT = 1000

function flushLogBatch() {
  if (logBatch.length === 0) return

  const batchToSend = [...logBatch]
  logBatch = []

  if (logBatchTimeout) {
    clearTimeout(logBatchTimeout)
    logBatchTimeout = null
  }

  BrowserWindow.getAllWindows().forEach((win) => win.webContents.send('go-logs-batch', batchToSend))
}

export function addLogToBatch(source: 'stdout' | 'stderr', line: string) {
  logBatch.push({
    source,
    line,
    timestamp: Date.now()
  })

  if (logBatch.length >= LOG_BATCH_SIZE) {
    flushLogBatch()
    return
  }

  if (!logBatchTimeout) {
    logBatchTimeout = setTimeout(flushLogBatch, LOG_BATCH_TIMEOUT)
  }
}

export function cleanupLogBatching() {
  if (logBatchTimeout) {
    clearTimeout(logBatchTimeout)
    logBatchTimeout = null
  }
  flushLogBatch()
}
