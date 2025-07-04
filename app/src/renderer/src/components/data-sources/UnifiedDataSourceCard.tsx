import { CheckCircle2, Loader2, Play, RefreshCw, Clock, AlertCircle, Trash2 } from 'lucide-react'
import { format } from 'date-fns'
import { Button } from '../ui/button'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '../ui/tooltip'
import { Progress } from '../ui/progress'
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
    <div className="p-4 w-full">
      <div className="font-semibold text-lg flex flex-wrap items-center justify-between lg:flex-row flex-col gap-5">
        <div className="flex items-center gap-5">
          <div className="w-10 h-10 flex items-center justify-center">{source.icon}</div>
          <span className="font-semibold text-lg leading-none">{source.label}</span>
        </div>
        <div className="flex items-center gap-2">
          {/* Show status badges and button */}
          {latestIndexedSource && !importingSource && !pendingSource && (
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger asChild>
                  <CheckCircle2 className="w-6 h-6 text-green-600 dark:text-green-400 bg-green-500/20 rounded-full p-1" />
                </TooltipTrigger>
                <TooltipContent>
                  <p>Last imported {format(latestIndexedSource.updatedAt, 'MMM d, yyyy')}</p>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          )}

          {/* Importing status inline */}
          {(importingSource || isBeingProcessed) && !isStuck && (
            <div className="flex items-center gap-2">
              <Loader2 className="h-5 w-5 animate-spin text-primary" />
              <span className="text-sm text-muted-foreground">
                {importingSource?.isProcessed
                  ? `Indexing ${Math.round(importingSource.indexProgress ?? 0)}%`
                  : 'Processing'}
              </span>
            </div>
          )}

          {/* Error state inline */}
          {hasError && !importingSource && <AlertCircle className="w-5 h-5 text-destructive" />}

          {/* Stuck status inline */}
          {isStuck && <AlertCircle className="w-5 h-5 text-amber-600" />}

          {/* Action button */}
          <Button
            variant={pendingSource ? 'default' : 'outline'}
            size="sm"
            onClick={pendingSource ? onStartImport : () => onSelect(source)}
            disabled={!canImport}
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

          {/* Remove button for pending sources */}
          {pendingSource && (
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8 hover:bg-destructive/10 hover:text-destructive rounded-full"
                    onClick={onRemovePending}
                  >
                    <Trash2 className="w-3 h-3" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>
                  <p>Remove pending import</p>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          )}
        </div>
      </div>

      {/* Progress bar and additional info moved below main row */}
      {(importingSource || isBeingProcessed) && !isStuck && (
        <div className="mt-3 px-4">
          <Progress value={progress} className="h-1.5" />
          {startTimeMs && (
            <div className="flex items-center gap-1 text-muted-foreground mt-1">
              <Clock className="h-3 w-3" />
              <span className="text-xs">
                {estimateRemainingTime(source.name, progress, importStartTime, fileSize)}
              </span>
            </div>
          )}
        </div>
      )}

      {/* Pending file info */}
      {pendingSource &&
        !importingSource &&
        (pendingSource.fileSize || fileSize) &&
        source.name === 'WhatsApp' && (
          <div className="mt-2 px-4">
            <span className="text-xs text-muted-foreground">
              Estimated time:{' '}
              {estimateRemainingTime(source.name, 0, undefined, pendingSource.fileSize || fileSize)}
            </span>
          </div>
        )}
    </div>
  )
}
