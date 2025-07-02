import { CheckCircle2, Loader2, Play, RefreshCw, Clock, AlertCircle } from 'lucide-react'
import { format } from 'date-fns'
import { Button } from '../ui/button'
import { Card } from '../ui/card'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '../ui/tooltip'
import { Progress } from '../ui/progress'
import { Badge } from '../ui/badge'
import { DataSource, PendingDataSource, IndexedDataSource } from './types'
import { estimateRemainingTime } from './utils'
import { useIndexingStore } from '@renderer/stores/indexingStore'

interface UnifiedDataSourceCardProps {
  source: DataSource
  indexedSources: IndexedDataSource[]
  pendingSource?: PendingDataSource
  fileSize?: number
  isImporting: boolean
  isGlobalProcessing: boolean
  isInitiating?: boolean
  onSelect: (source: DataSource) => void
  onRemovePending: () => void
  onStartImport: () => void
}

export const UnifiedDataSourceCard = ({
  source,
  indexedSources,
  pendingSource,
  fileSize,
  isImporting,
  isGlobalProcessing,
  isInitiating,
  onSelect,
  onRemovePending,
  onStartImport
}: UnifiedDataSourceCardProps) => {
  const { getDataSourceProgress } = useIndexingStore()

  // Find the currently importing source
  const importingSource = indexedSources.find((s) => s.isProcessed && !s.isIndexed && !s.hasError)

  // Also check if this source is being processed (handles the delay in subscription updates)
  // But exclude sources with errors as they're not actively processing
  const isBeingProcessed =
    indexedSources.some(
      (s) => s.name === source.name && !s.isProcessed && !s.isIndexed && !s.hasError
    ) || isInitiating

  // Get the most recent indexed source
  const latestIndexedSource = indexedSources
    .filter((s) => s.isIndexed)
    .sort((a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime())[0]

  // Calculate progress
  const progress = importingSource
    ? importingSource.isProcessed
      ? 10 + (importingSource.indexProgress ?? 0) * 0.9
      : 5
    : 0

  // Get start time from the store
  const startTimeMs = importingSource
    ? getDataSourceProgress(importingSource.id)?.startTime
    : undefined
  const importStartTime = startTimeMs ? new Date(startTimeMs) : undefined

  const hasError = indexedSources.some((s) => s.hasError)

  // Check if this source appears to be stuck (has been in processing state for too long)
  const isStuck = importingSource && startTimeMs && Date.now() - startTimeMs > 300000 // 5 minutes

  const canImport = !isImporting && !isGlobalProcessing && !importingSource && !isBeingProcessed

  return (
    <Card className="p-4 flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
      <div className="flex-1 flex flex-col gap-3">
        <div className="flex items-start justify-between">
          <div className="flex items-center gap-3">
            {source.icon}
            <div>
              <h3 className="font-medium">{source.label}</h3>
              <p className="text-sm text-muted-foreground">{source.description}</p>
            </div>
          </div>
        </div>

        {/* Status Section */}
        <div className="flex flex-col gap-2">
          {/* Previous imports */}
          {latestIndexedSource && !importingSource && !pendingSource && (
            <div className="flex items-center justify-between text-sm">
              <div className="flex items-center gap-2">
                <CheckCircle2 className="h-4 w-4 text-primary" />
                <span className="text-muted-foreground">Last imported</span>
              </div>
              <TooltipProvider>
                <Tooltip delayDuration={0}>
                  <TooltipTrigger>
                    <span className="text-muted-foreground">
                      {format(latestIndexedSource.updatedAt, 'MMM d, yyyy')}
                    </span>
                  </TooltipTrigger>
                  <TooltipContent>
                    <p>{format(latestIndexedSource.updatedAt, 'h:mm a')}</p>
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
            </div>
          )}

          {/* Pending selection */}
          {pendingSource && !importingSource && (
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Badge variant="secondary" className="text-xs">
                  <CheckCircle2 className="h-3 w-3 mr-1" />
                  Ready to import
                </Badge>
                {(pendingSource.fileSize || fileSize) && source.name === 'WhatsApp' && (
                  <span className="text-xs text-muted-foreground">
                    {estimateRemainingTime(
                      source.name,
                      0,
                      undefined,
                      pendingSource.fileSize || fileSize
                    )}
                  </span>
                )}
              </div>
              <Button variant="ghost" size="sm" onClick={onRemovePending} className="h-7 text-xs">
                Remove
              </Button>
            </div>
          )}

          {/* Importing status */}
          {(importingSource || isBeingProcessed) && !isStuck && (
            <div className="flex flex-col gap-2">
              <div className="flex items-center justify-between text-sm">
                <div className="flex items-center gap-2">
                  <Loader2 className="h-4 w-4 animate-spin text-primary" />
                  <span>
                    {importingSource?.isProcessed
                      ? `Indexing ${Math.round(importingSource.indexProgress ?? 0)}%`
                      : 'Processing'}
                  </span>
                </div>
                <div className="flex items-center gap-1 text-muted-foreground">
                  <Clock className="h-3 w-3" />
                  <span className="text-xs">
                    {estimateRemainingTime(source.name, progress, importStartTime, fileSize)}
                  </span>
                </div>
              </div>
              <Progress value={progress} className="h-1.5" />
            </div>
          )}

          {/* Stuck status */}
          {isStuck && (
            <div className="flex items-center gap-2 text-sm text-amber-600">
              <AlertCircle className="h-4 w-4" />
              <span>Import appears to be stuck</span>
            </div>
          )}

          {/* Error state */}
          {hasError && !importingSource && (
            <div className="flex items-center gap-2 text-sm text-destructive">
              <AlertCircle className="h-4 w-4" />
              <span>Import failed</span>
            </div>
          )}
        </div>
      </div>

      {/* Action button */}
      <Button
        variant={pendingSource ? 'default' : 'outline'}
        size="sm"
        onClick={pendingSource ? onStartImport : () => onSelect(source)}
        disabled={!canImport}
        className="w-full sm:w-auto"
      >
        {importingSource || isBeingProcessed ? (
          <>
            <Loader2 className="mr-2 h-4 w-4 animate-spin" />
            Importing...
          </>
        ) : pendingSource ? (
          <>
            Start Import
            <Play className="ml-2 h-4 w-4" />
          </>
        ) : latestIndexedSource ? (
          <>
            <RefreshCw className="mr-2 h-4 w-4" />
            Import Again
          </>
        ) : (
          <>Import</>
        )}
      </Button>
    </Card>
  )
}
