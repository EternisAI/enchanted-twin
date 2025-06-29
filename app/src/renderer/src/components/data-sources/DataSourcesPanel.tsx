import { useQuery, useMutation, useSubscription } from '@apollo/client'
import {
  GetDataSourcesDocument,
  IndexingState,
  IndexingStatusDocument
} from '@renderer/graphql/generated/graphql'
import { Button } from '../ui/button'
import {
  CheckCircle2,
  Loader2,
  X,
  Play,
  RefreshCw,
  ChevronDown,
  ChevronRight,
  History
} from 'lucide-react'
import { useState, useCallback, useEffect, ReactNode } from 'react'
import WhatsAppIcon from '@renderer/assets/icons/whatsapp'
import TelegramIcon from '@renderer/assets/icons/telegram'
import SlackIcon from '@renderer/assets/icons/slack'
import GmailIcon from '@renderer/assets/icons/gmail'
import XformerlyTwitterIcon from '@renderer/assets/icons/x'
import { DataSource, DataSourcesPanelProps, PendingDataSource, IndexedDataSource } from './types'
import { toast } from 'sonner'
import { gql } from '@apollo/client'
import { Card } from '../ui/card'
import OpenAI from '@renderer/assets/icons/openai'
import { format } from 'date-fns'
import { TooltipContent, TooltipTrigger } from '../ui/tooltip'
import { Tooltip, TooltipProvider } from '@radix-ui/react-tooltip'
import WhatsAppSync from './custom-view/WhatAppSync'
import { DataSourceDialog } from './DataSourceDialog'
import { Dialog, DialogContent } from '../ui/dialog'

const ADD_DATA_SOURCE = gql`
  mutation AddDataSource($name: String!, $path: String!) {
    addDataSource(name: $name, path: $path)
  }
`

const START_INDEXING = gql`
  mutation StartIndexing {
    startIndexing
  }
`

const SUPPORTED_DATA_SOURCES: DataSource[] = [
  {
    name: 'X',
    label: 'Twitter',
    description: 'Import your X tweets and messages',
    selectType: 'files',
    fileRequirement: 'Select X ZIP',
    icon: <XformerlyTwitterIcon className="h-4 w-4" />,
    fileFilters: [{ name: 'X Archive', extensions: ['zip'] }]
  },
  {
    name: 'ChatGPT',
    label: 'ChatGPT',
    description: 'Import your ChatGPT history',
    selectType: 'files',
    fileRequirement: 'Select ChatGPT JSON or ZIP export file',
    icon: <OpenAI className="h-4 w-4" />,
    fileFilters: [{ name: 'ChatGPT Files', extensions: ['json', 'zip'] }]
  },
  {
    name: 'WhatsApp',
    label: 'WhatsApp',
    description: 'Import your WhatsApp chat history',
    selectType: 'files',
    fileRequirement: 'Select WhatsApp SQLITE file',
    icon: <WhatsAppIcon className="h-4 w-4" />,
    fileFilters: [{ name: 'WhatsApp Database', extensions: ['db', 'sqlite'] }],
    customView: {
      name: 'QR Code',
      component: <WhatsAppSync />
    }
  },
  {
    name: 'Telegram',
    label: 'Telegram',
    description: 'Import your Telegram messages and media',
    selectType: 'files',
    fileRequirement: 'Select Telegram JSON export file',
    icon: <TelegramIcon className="h-4 w-4" />,
    fileFilters: [{ name: 'Telegram Export', extensions: ['json'] }]
  },
  {
    name: 'Slack',
    label: 'Slack',
    description: 'Import your Slack workspace data',
    selectType: 'files',
    fileRequirement: 'Select Slack ZIP file',
    icon: <SlackIcon className="h-5 w-5" />,
    fileFilters: [{ name: 'Slack Export', extensions: ['zip'] }]
  },
  {
    name: 'Gmail',
    label: 'Gmail',
    description: 'Import your Gmail emails and attachments',
    selectType: 'files',
    fileRequirement: 'Select Gmail MBOX or Google Takeout ZIP file',
    icon: <GmailIcon className="h-5 w-5" />,
    fileFilters: [{ name: 'Gmail Files', extensions: ['mbox', 'zip'] }]
  }
]

const DataSourceCard = ({
  icon,
  label,
  onClick,
  disabled
}: {
  icon: ReactNode
  label: string
  onClick: () => void
  disabled: boolean
}) => (
  <Button
    variant="outline"
    size="lg"
    className="h-auto py-4 rounded-xl"
    onClick={onClick}
    disabled={disabled}
  >
    <div className="flex flex-col items-center gap-3 text-base">
      {icon}
      <span className="font-semibold text-sm">{label}</span>
    </div>
  </Button>
)

const PendingDataSourceCard = ({
  source,
  onRemove
}: {
  source: PendingDataSource
  onRemove: () => void
}) => {
  const sourceDetails = SUPPORTED_DATA_SOURCES.find((s) => s.name === source.name)
  if (!sourceDetails) return null

  return (
    <div className="p-4 rounded-lg h-full flex items-center justify-between gap-3">
      <div className="flex items-center gap-3">
        {sourceDetails.icon}
        <div>
          <h3 className="font-medium">{source.name}</h3>
        </div>
      </div>
      <div className="flex items-center gap-2">
        <div className="flex items-center gap-1 text-xs text-muted-foreground">
          <CheckCircle2 className="h-3 w-3 text-primary" />
          <span>Selected</span>
        </div>
        <Button variant="ghost" size="icon" className="h-6 w-6" onClick={onRemove}>
          <X className="h-4 w-4 text-muted-foreground" />
        </Button>
      </div>
    </div>
  )
}

const IndexedDataSourceCard = ({
  source
  // onRemove
}: {
  source: IndexedDataSource
  // onRemove: () => void
}) => {
  const sourceDetails = SUPPORTED_DATA_SOURCES.find((s) => s.name === source.name)
  if (!sourceDetails) return null

  // Calculate progress:
  // - 10% for processing
  // - 90% for indexing
  const processingProgress = source.isProcessed ? 10 : 0
  const indexingProgress = source.isIndexed
    ? 90
    : source.indexProgress !== undefined
      ? source.indexProgress * 0.9
      : 0
  const totalProgress = processingProgress + indexingProgress

  const showProgressBar =
    !source.isIndexed && (source.isProcessed ? (source.indexProgress ?? 0) >= 0 : true)

  // Format status text with percentage
  const getStatusText = () => {
    if (source.isIndexed) return ''
    if (!source.isProcessed) return 'Processing'
    if (source.indexProgress !== undefined && source.indexProgress >= 0) {
      return `Indexing ${Math.round(source.indexProgress)}%`
    }
    return 'Pending'
  }

  return (
    <div className="p-4 rounded-lg bg-transparent h-full flex flex-col gap-3">
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <div className="flex shrink-0 items-center gap-2">{sourceDetails.icon}</div>
          <div className="flex flex-col gap-0 justify-start">
            <h3 className="font-medium">{source.name}</h3>
            {source.hasError && <p className="text-xs text-red-500">Error</p>}
            {!showProgressBar && (
              <TooltipProvider>
                <Tooltip delayDuration={0}>
                  <TooltipTrigger>
                    <p className="text-xs text-start text-muted-foreground">
                      {format(source.updatedAt, 'MMM d, yyyy')}
                    </p>
                    <TooltipContent>
                      <p>{format(source.updatedAt, 'h:mm a')}</p>
                    </TooltipContent>
                  </TooltipTrigger>
                </Tooltip>
              </TooltipProvider>
            )}
          </div>
        </div>
        <div className="flex items-center gap-1 text-xs text-muted-foreground">
          <span>{getStatusText()}</span>
        </div>
      </div>
      {showProgressBar && (
        <div className="w-full bg-secondary rounded-full h-1.5">
          <div
            className="bg-primary h-1.5 rounded-full transition-all duration-300"
            style={{
              width: `${totalProgress}%`
            }}
          />
        </div>
      )}
    </div>
  )
}

const CollapsibleDataSourceGroup = ({
  title,
  icon,
  sources,
  isExpanded,
  onToggle
}: {
  title: string
  icon: ReactNode
  sources: IndexedDataSource[]
  isExpanded: boolean
  onToggle: () => void
}) => {
  const hasIndexingSource = sources.some(
    (source) => source.isProcessed && (source.indexProgress ?? 0) > 0 && !source.isIndexed
  )

  return (
    <div className="flex flex-col gap-2">
      <Button variant="ghost" className="w-full justify-start gap-2" onClick={onToggle}>
        {isExpanded ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
        {icon}
        <span className="font-medium">{title}</span>
        <span className="ml-auto text-sm text-muted-foreground">
          {sources.length} {sources.length === 1 ? 'source' : 'sources'}
        </span>
        {hasIndexingSource && <Loader2 className="h-4 w-4 animate-spin text-primary" />}
      </Button>
      {isExpanded && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 pl-6">
          {sources.map((source) => (
            <IndexedDataSourceCard key={source.id} source={source} />
          ))}
        </div>
      )}
    </div>
  )
}

export function DataSourcesPanel({
  onDataSourceSelected,
  onDataSourceRemoved,
  showStatus = false,
  onIndexingComplete,
  header = true
}: Omit<DataSourcesPanelProps, 'indexingStatus'> & { onIndexingComplete?: () => void }) {
  const { data, refetch } = useQuery(GetDataSourcesDocument)
  const { data: indexingData } = useSubscription(IndexingStatusDocument)
  const [addDataSource] = useMutation(ADD_DATA_SOURCE)
  const [startIndexing] = useMutation(START_INDEXING)
  const [selectedSource, setSelectedSource] = useState<DataSource | null>(null)
  const [pendingDataSources, setPendingDataSources] = useState<Record<string, PendingDataSource>>(
    {}
  )
  const [expandedGroups, setExpandedGroups] = useState<Record<string, boolean>>({})
  const [isIndexingInitiated, setIsIndexingInitiated] = useState(false)

  // Workflow states
  const isIndexing = indexingData?.indexingStatus?.status === IndexingState.IndexingData
  const isProcessing = indexingData?.indexingStatus?.status === IndexingState.ProcessingData
  const isNotStarted = indexingData?.indexingStatus?.status === IndexingState.NotStarted
  const hasGlobalError = indexingData?.indexingStatus?.error ?? false
  const hasPendingDataSources = Object.keys(pendingDataSources).length > 0

  // Check if there are sources with errors
  const hasSourcesWithErrors = data?.getDataSources?.some((source) => source.hasError) ?? false

  // Check if any source is currently being processed or indexed
  const hasActiveIndexing =
    indexingData?.indexingStatus?.dataSources?.some(
      (source) => source.isProcessed && !source.isIndexed
    ) ?? false

  // Show loading state if indexing was initiated but we haven't received status yet
  const showLoadingState = isIndexingInitiated && !isIndexing && !isProcessing && !isNotStarted

  // Reset local state when we receive actual indexing status
  useEffect(() => {
    if (isIndexing || isProcessing || isNotStarted) {
      setIsIndexingInitiated(false)
    }
  }, [isIndexing, isProcessing, isNotStarted])

  const handleRemoveDataSource = useCallback(
    async (name: string) => {
      // await removeDataSource({ variables: { name } })
      setPendingDataSources((prev) => {
        const newState = { ...prev }
        delete newState[name]
        return newState
      })
      onDataSourceRemoved?.(name)
    },
    [onDataSourceRemoved]
  )

  const handleSourceSelected = useCallback(
    (source: DataSource) => {
      setSelectedSource(source)
      onDataSourceSelected?.(source)
    },
    [onDataSourceSelected]
  )

  const handleFileSelect = useCallback(async () => {
    if (!selectedSource) return

    const result = await window.api.selectFiles({
      filters: selectedSource.fileFilters
    })

    if (result.canceled) return

    const path = result.filePaths[0]
    setPendingDataSources((prev) => ({
      ...prev,
      [selectedSource.name]: {
        name: selectedSource.name,
        path
      }
    }))
    setSelectedSource(null)
  }, [selectedSource])

  const handleAddSource = useCallback(() => {
    if (!selectedSource) return
    setPendingDataSources((prev) => ({
      ...prev,
      [selectedSource.name]: {
        name: selectedSource.name,
        path: ''
      }
    }))
    setSelectedSource(null)
  }, [selectedSource])

  const addDataSources = useCallback(async () => {
    try {
      for (const source of Object.values(pendingDataSources)) {
        await addDataSource({
          variables: {
            name: source.name,
            path: source.path
          }
        })
        window.api.analytics.capture('data_source_added', {
          source: source.name
        })
      }
      // Clear pending data sources after successful addition
      setPendingDataSources({})
      // Refetch data sources to get the latest state
      await refetch()
      return true
    } catch (error) {
      console.error('Error adding data sources:', error)
      toast.error('Failed to add data sources. Please try again.')
      return false
    }
  }, [addDataSource, pendingDataSources, refetch])

  const startIndexingProcess = useCallback(async () => {
    try {
      await startIndexing()
      return true
    } catch (error) {
      console.error('Error starting indexing:', error)
      toast.error('Failed to start indexing. Please try again.')
      return false
    }
  }, [startIndexing])

  const handleStartIndexing = useCallback(async () => {
    if (!hasPendingDataSources) {
      toast.error('Please select at least one data source to index')
      return
    }

    try {
      setIsIndexingInitiated(true)
      const sourcesAdded = await addDataSources()
      if (!sourcesAdded) {
        setIsIndexingInitiated(false)
        return
      }

      toast.success('Data sources added successfully')

      const indexingStarted = await startIndexingProcess()
      if (!indexingStarted) {
        setIsIndexingInitiated(false)
        return
      }
    } catch (error) {
      console.error('Error in indexing process:', error)
      toast.error('An error occurred during the indexing process')
      setIsIndexingInitiated(false)
    }
  }, [addDataSources, hasPendingDataSources, startIndexingProcess])

  const handleRetryIndexing = useCallback(async () => {
    try {
      const indexingStarted = await startIndexingProcess()
      if (!indexingStarted) return
      toast.success('Indexing process restarted successfully')
    } catch (error) {
      console.error('Error retrying indexing:', error)
      toast.error('Failed to restart indexing')
    }
  }, [startIndexingProcess])

  useEffect(() => {
    // Only trigger completion when:
    // 1. We're not in an indexing state
    // 2. All sources are indexed
    // 3. We have at least one data source
    const hasDataSources = (indexingData?.indexingStatus?.dataSources?.length ?? 0) > 0
    const allSourcesIndexed =
      indexingData?.indexingStatus?.dataSources?.every((source) => source.isIndexed) ?? false
    if (!isIndexing && allSourcesIndexed && hasDataSources) {
      onIndexingComplete?.()
    }
  }, [isIndexing, onIndexingComplete, indexingData?.indexingStatus?.dataSources])

  const toggleGroup = useCallback((groupName: string) => {
    setExpandedGroups((prev) => ({
      ...prev,
      [groupName]: !prev[groupName]
    }))
  }, [])

  const groupedDataSources = useCallback(() => {
    if (!data?.getDataSources) return {}

    return data.getDataSources.reduce(
      (acc, source) => {
        const sourceDetails = SUPPORTED_DATA_SOURCES.find((s) => s.name === source.name)
        if (!sourceDetails) return acc

        const groupName = source.name
        if (!acc[groupName]) {
          acc[groupName] = {
            title: sourceDetails.label,
            icon: sourceDetails.icon,
            sources: []
          }
        }
        acc[groupName].sources.push({
          ...source,
          indexProgress: indexingData?.indexingStatus?.dataSources?.find((s) => s.id === source.id)
            ?.indexProgress
        })
        return acc
      },
      {} as Record<string, { title: string; icon: ReactNode; sources: IndexedDataSource[] }>
    )
  }, [data?.getDataSources, indexingData?.indexingStatus?.dataSources])

  return (
    <Card className="flex flex-col gap-6 p-6 max-w-4xl">
      {header && (
        <div className="flex flex-col gap-2 items-center">
          <History className="w-6 h-6 text-primary" />
          <h2 className="text-2xl font-semibold">Import your history</h2>
          <p className="text-muted-foreground text-balance max-w-md text-center">
            Your imported data will be used to power your Twin&apos;s understanding of you.
          </p>
        </div>
      )}
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-6">
        {SUPPORTED_DATA_SOURCES.map((source) => (
          <DataSourceCard
            key={source.name}
            icon={source.icon}
            label={source.label}
            disabled={
              isIndexing || isProcessing || isNotStarted || hasActiveIndexing || showLoadingState
            }
            onClick={() => handleSourceSelected(source)}
          />
        ))}
      </div>

      {(Object.keys(pendingDataSources).length > 0 || (showStatus && data?.getDataSources)) && (
        <div className="flex flex-col gap-4">
          <div className="flex gap-8 items-center">
            <div className="h-0.5 w-full bg-secondary rounded-full" />
            <h3 className="shrink-0 text-sm font-medium text-muted-foreground text-center">
              {showStatus ? 'Imported Data Sources' : 'Selected Data Sources'}
            </h3>
            <div className="h-0.5 w-full bg-secondary rounded-full" />
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {Object.values(pendingDataSources).map((source) => (
              <PendingDataSourceCard
                key={source.name}
                source={source}
                onRemove={() => handleRemoveDataSource(source.name)}
              />
            ))}
          </div>

          {showStatus && data?.getDataSources && (
            <div className="flex flex-col gap-4">
              {Object.entries(groupedDataSources()).map(([groupName, group]) => {
                // Only show collapsible group if there's more than one source
                if (group.sources.length > 1) {
                  return (
                    <CollapsibleDataSourceGroup
                      key={groupName}
                      title={group.title}
                      icon={group.icon}
                      sources={group.sources}
                      isExpanded={expandedGroups[groupName] ?? true}
                      onToggle={() => toggleGroup(groupName)}
                    />
                  )
                } else if (group.sources.length === 1) {
                  // Show single source directly without collapsible wrapper
                  return (
                    <div
                      key={groupName}
                      className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4"
                    >
                      <IndexedDataSourceCard source={group.sources[0]} />
                    </div>
                  )
                }
                return null
              })}
              {/* Show loading state for recently added sources if indexing was just initiated */}
              {showLoadingState &&
                Object.values(pendingDataSources).length === 0 &&
                data?.getDataSources?.length === 0 && (
                  <div className="flex items-center justify-center gap-2 text-muted-foreground py-4">
                    <Loader2 className="h-4 w-4 animate-spin" />
                    <span>Initializing indexing process...</span>
                  </div>
                )}
            </div>
          )}
        </div>
      )}

      {hasSourcesWithErrors &&
        !isIndexing &&
        !isProcessing &&
        !isNotStarted &&
        !hasActiveIndexing && (
          <Button size="lg" onClick={handleRetryIndexing} className="w-full">
            <RefreshCw className="mr-2 h-4 w-4" />
            Retry Import
          </Button>
        )}

      {!hasSourcesWithErrors && (
        <Button
          size="lg"
          onClick={handleStartIndexing}
          disabled={
            isIndexing ||
            isProcessing ||
            isNotStarted ||
            !hasPendingDataSources ||
            hasActiveIndexing ||
            showLoadingState
          }
          className="w-fit"
        >
          {isIndexing || showLoadingState ? (
            <>
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              Indexing...
            </>
          ) : isProcessing ? (
            <>
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              Processing...
            </>
          ) : isNotStarted ? (
            <>
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              Starting...
            </>
          ) : (
            <>
              Begin import <Play className="ml-2 h-4 w-4" />
            </>
          )}
        </Button>
      )}

      {hasGlobalError && (
        <div className="text-center text-sm text-red-500">
          {indexingData?.indexingStatus?.error}
        </div>
      )}

      <Dialog open={!!selectedSource} onOpenChange={() => setSelectedSource(null)}>
        <DialogContent className="fixed left-[50%] top-[50%] z-[200] grid w-full max-w-lg translate-x-[-50%] translate-y-[-50%] gap-4 border bg-background p-6 shadow-lg duration-200 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95 data-[state=closed]:slide-out-to-center data-[state=open]:slide-in-from-center sm:rounded-lg">
          <DataSourceDialog
            selectedSource={selectedSource}
            onClose={() => setSelectedSource(null)}
            pendingDataSources={pendingDataSources}
            onFileSelect={handleFileSelect}
            onAddSource={handleAddSource}
            customComponent={selectedSource?.customView ? selectedSource.customView : undefined}
          />
        </DialogContent>
      </Dialog>
    </Card>
  )
}
