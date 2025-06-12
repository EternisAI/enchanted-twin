import React, { createContext, useContext, useEffect, useState, useCallback } from 'react'
import { toast } from 'sonner'

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
}

const GoLogsContext = createContext<GoLogsContextType | undefined>(undefined)

export function useGoLogs() {
  const context = useContext(GoLogsContext)
  if (context === undefined) {
    throw new Error('useGoLogs must be used within a GoLogsProvider')
  }
  return context
}

interface GoLogsProviderProps {
  children: React.ReactNode
}

export function GoLogsProvider({ children }: GoLogsProviderProps) {
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [errorCount, setErrorCount] = useState(0)

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

  const value: GoLogsContextType = {
    logs,
    errorCount,
    clearLogs,
    downloadLogs
  }

  return <GoLogsContext.Provider value={value}>{children}</GoLogsContext.Provider>
}
