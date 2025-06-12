import { useEffect, useRef, useState } from 'react'
import { Card } from '../ui/card'
import { Badge } from '../ui/badge'
import { Button } from '../ui/button'
import { Trash2, Download, ScrollText } from 'lucide-react'
import { useGoLogs } from '../../contexts/GoLogsContext'

export default function GoLogsViewer() {
  const { logs, clearLogs, downloadLogs } = useGoLogs()
  const [autoScroll, setAutoScroll] = useState(true)
  const logsEndRef = useRef<HTMLDivElement>(null)
  const logsContainerRef = useRef<HTMLDivElement>(null)

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

  return (
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
  )
}
