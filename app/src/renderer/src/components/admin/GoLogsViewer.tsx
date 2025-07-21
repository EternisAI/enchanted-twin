import { useEffect, useRef, useState } from 'react'
import { Card } from '../ui/card'
import { Badge } from '../ui/badge'
import { Button } from '../ui/button'
import {
  Trash2,
  Download,
  ScrollText,
  Play,
  Square,
  RefreshCw,
  Server,
  ExternalLink
} from 'lucide-react'
import { useGoServerContext } from '../../contexts/GoServerContext'
import { useGoServer } from '../../hooks/useGoServer'

export default function GoLogsViewer() {
  const { logs, clearLogs, downloadLogs } = useGoServerContext()
  const { state, checkStatus, start, stop, retry } = useGoServer()
  const [autoScroll, setAutoScroll] = useState(true)
  const logsEndRef = useRef<HTMLDivElement>(null)
  const logsContainerRef = useRef<HTMLDivElement>(null)

  const isDev = process.env.NODE_ENV === 'development'

  useEffect(() => {
    checkStatus()
  }, [checkStatus])

  useEffect(() => {
    if (autoScroll && logsEndRef.current) {
      logsEndRef.current.scrollIntoView({ behavior: 'smooth' })
    }
  }, [logs, autoScroll])

  const handleScroll = () => {
    if (logsContainerRef.current) {
      const { scrollTop, scrollHeight, clientHeight } = logsContainerRef.current
      const isAtBottom = scrollHeight - scrollTop - clientHeight < 50
      setAutoScroll(isAtBottom)
    }
  }

  const getStatusBadge = () => {
    if (isDev) {
      return <Badge variant="secondary">External (Dev Mode)</Badge>
    }
    return (
      <Badge variant={state.isRunning ? 'default' : 'destructive'}>
        {state.isRunning ? 'Running' : 'Stopped'}
      </Badge>
    )
  }

  const getActionButtons = () => {
    if (isDev) {
      return (
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={checkStatus} disabled={state.initializing}>
            <RefreshCw className="w-4 h-4 mr-1" />
            Check Status
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled
            title="In development mode, run the server externally"
          >
            <ExternalLink className="w-4 h-4 mr-1" />
            External Server
          </Button>
        </div>
      )
    }

    return (
      <div className="flex items-center gap-2">
        <Button variant="outline" size="sm" onClick={checkStatus} disabled={state.initializing}>
          <RefreshCw className="w-4 h-4 mr-1" />
          Check Status
        </Button>

        {!state.isRunning && (
          <Button variant="outline" size="sm" onClick={start} disabled={state.initializing}>
            <Play className="w-4 h-4 mr-1" />
            Start
          </Button>
        )}

        {state.isRunning && (
          <Button variant="outline" size="sm" onClick={stop} disabled={state.initializing}>
            <Square className="w-4 h-4 mr-1" />
            Stop
          </Button>
        )}

        {state.error && (
          <Button variant="outline" size="sm" onClick={retry} disabled={state.initializing}>
            <RefreshCw className="w-4 h-4 mr-1" />
            Retry
          </Button>
        )}
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-4">
      <Card className="p-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Server className="w-5 h-5" />
            <h3>Go Server Status</h3>
            {getStatusBadge()}
            {state.initializing && <Badge variant="secondary">Initializing...</Badge>}
          </div>

          {getActionButtons()}
        </div>

        {isDev && (
          <div className="mt-3 p-2 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded text-sm text-blue-700 dark:text-blue-300">
            <strong>Development Mode:</strong> Server is expected to run externally. Start it with
            your usual dev command.
          </div>
        )}

        {state.error && !isDev && (
          <div className="mt-3 p-2 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded text-sm text-red-700 dark:text-red-300">
            <strong>Error:</strong> {state.error}
          </div>
        )}
      </Card>

      <Card className="p-4 flex flex-col gap-4 min-w-2xl">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <ScrollText className="w-5 h-5" />
            <h3>Server Logs</h3>
            <Badge variant="outline">{logs.length} entries</Badge>
          </div>

          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setAutoScroll(!autoScroll)}
              className={autoScroll ? 'bg-green-50 border-green-200' : ''}
            >
              Auto-scroll: {autoScroll ? 'ON' : 'OFF'}
            </Button>
            <Button variant="outline" size="sm" onClick={downloadLogs} disabled={logs.length === 0}>
              <Download className="w-4 h-4 mr-1" />
              Download
            </Button>
            <Button variant="outline" size="sm" onClick={clearLogs} disabled={logs.length === 0}>
              <Trash2 className="w-4 h-4 mr-1" />
              Clear
            </Button>
          </div>
        </div>

        <div
          ref={logsContainerRef}
          onScroll={handleScroll}
          className="h-96 overflow-y-auto bg-gray-50 dark:bg-gray-900 rounded border p-3 text-sm w-full"
        >
          {logs.length === 0 ? (
            <div className="text-gray-500 text-center py-8">
              No logs received yet. Go server logs will appear here when available.
            </div>
          ) : (
            <div className="flex flex-col gap-1">
              {logs.map((log) => (
                <div
                  key={log.id}
                  className={`flex items-start gap-2 ${
                    log.source === 'stderr'
                      ? 'text-red-600 dark:text-red-400'
                      : 'text-gray-700 dark:text-gray-300'
                  }`}
                >
                  <span className="text-xs text-gray-500 shrink-0">
                    {log.timestamp.toLocaleTimeString()}
                  </span>
                  <Badge
                    variant={log.source === 'stderr' ? 'destructive' : 'secondary'}
                    className="text-xs shrink-0"
                  >
                    {log.source}
                  </Badge>
                  <span className="break-all">{log.line}</span>
                </div>
              ))}
              <div ref={logsEndRef} />
            </div>
          )}
        </div>
      </Card>
    </div>
  )
}
