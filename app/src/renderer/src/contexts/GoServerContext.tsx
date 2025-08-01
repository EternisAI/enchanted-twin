import React, { createContext, useContext, useEffect, useState, useCallback, useMemo } from 'react'
import { toast } from 'sonner'
import { useGoServer } from '@renderer/hooks/useGoServer'

export interface LogEntry {
  id: string
  source: 'stdout' | 'stderr'
  line: string
  timestamp: Date
}

interface GoLogsContextType {
  logs: LogEntry[]
  errorCount: number
  clearLogs: () => void
  downloadLogs: () => void
  goServerState: ReturnType<typeof useGoServer>['state']
  goServerActions: Omit<ReturnType<typeof useGoServer>, 'state'>
}

const GoLogsContext = createContext<GoLogsContextType | undefined>(undefined)

export function useGoServerContext() {
  const context = useContext(GoLogsContext)
  if (context === undefined) {
    throw new Error('useGoServerContext must be used within a GoServerProvider')
  }
  return context
}

interface GoLogsProviderProps {
  children: React.ReactNode
}

export function GoServerProvider({ children }: GoLogsProviderProps) {
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [errorCount, setErrorCount] = useState(0)

  const { state, checkStatus, start, stop, initializeIfNeeded, retry } = useGoServer()

  useEffect(() => {
    const cleanup = window.api.onGoLog((data) => {
      const newLog: LogEntry = {
        id: `${Date.now()}-${Math.random()}`,
        source: data.source,
        line: data.line,
        timestamp: new Date()
      }

      setLogs((prev) => {
        const updated = [...prev, newLog]
        return updated.length > 1000 ? updated.slice(-1000) : updated
      })

      if (data.source === 'stderr') {
        setErrorCount((prev) => prev + 1)
      }
    })

    return cleanup
  }, [])

  const clearLogs = useCallback(() => {
    setLogs([])
    setErrorCount(0)
  }, [])

  const downloadLogs = useCallback(() => {
    const logsText = logs
      .map((log) => `[${log.timestamp.toISOString()}] ${log.source.toUpperCase()}: ${log.line}`)
      .join('\n')

    const blob = new Blob([logsText], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `go-server-logs-${new Date().toISOString().split('T')[0]}.txt`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)

    toast.success('Logs downloaded initiated successfully')
  }, [logs])

  const value = useMemo(
    () => ({
      logs,
      errorCount,
      clearLogs,
      downloadLogs,
      goServerState: state,
      goServerActions: {
        checkStatus,
        start,
        stop,
        initializeIfNeeded,
        retry
      }
    }),
    [
      logs,
      errorCount,
      clearLogs,
      downloadLogs,
      state,
      checkStatus,
      start,
      stop,
      initializeIfNeeded,
      retry
    ]
  )

  return <GoLogsContext.Provider value={value}>{children}</GoLogsContext.Provider>
}
