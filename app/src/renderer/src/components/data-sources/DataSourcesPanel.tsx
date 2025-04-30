import { useQuery, useMutation, useSubscription } from '@apollo/client'
import {
  GetDataSourcesDocument,
  IndexingState,
  IndexingStatusDocument
} from '@renderer/graphql/generated/graphql'
import { Button } from '../ui/button'
import { CheckCircle2, Loader2, X, Play, RefreshCw, Import } from 'lucide-react'
import { useState, useCallback, useEffect } from 'react'
import WhatsAppIcon from '@renderer/assets/icons/whatsapp'
import TelegramIcon from '@renderer/assets/icons/telegram'
import SlackIcon from '@renderer/assets/icons/slack'
import GmailIcon from '@renderer/assets/icons/gmail'
import XformerlyTwitterIcon from '@renderer/assets/icons/x'
import { DataSource, DataSourcesPanelProps, PendingDataSource, IndexedDataSource } from './types'
import { toast } from 'sonner'
import { gql } from '@apollo/client'
import { DataSourceDialog } from './DataSourceDialog'
import { Card } from '../ui/card'
import OpenAI from '@renderer/assets/icons/openai'
import { format } from 'date-fns'

const ADD_DATA_SOURCE = gql`
  mutation AddDataSource($name: String!, $path: String!) {
    addDataSource(name: $name, path: $path)
  }
`

const DELETE_DATA_SOURCE = gql`
  mutation DeleteDataSource($name: String!) {
    deleteDataSource(name: $name)
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
    icon: <XformerlyTwitterIcon className="h-6 w-6" />,
    fileFilters: [{ name: 'X Archive', extensions: ['zip'] }]
  },
  {
    name: 'ChatGPT',
    label: 'ChatGPT',
    description: 'Import your ChatGPT history',
    selectType: 'files',
    fileRequirement: 'Select ChatGPT export file',
    icon: <OpenAI className="h-6 w-6" />,
    fileFilters: [{ name: 'ChatGPT', extensions: ['zip'] }]
  },
  {
    name: 'WhatsApp',
    label: 'WhatsApp',
    description: 'Import your WhatsApp chat history',
    selectType: 'files',
    fileRequirement: 'Select WhatsApp SQLITE file',
    icon: <WhatsAppIcon className="h-6 w-6" />,
    fileFilters: [{ name: 'WhatsApp Database', extensions: ['db', 'sqlite'] }]
  },
  {
    name: 'Telegram',
    label: 'Telegram',
    description: 'Import your Telegram messages and media',
    selectType: 'files',
    fileRequirement: 'Select Telegram JSON export file',
    icon: <TelegramIcon className="h-6 w-6" />,
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
    fileRequirement: 'Select Google Takeout ZIP file',
    icon: <GmailIcon className="h-5 w-5" />,
    fileFilters: [{ name: 'Google Takeout', extensions: ['zip'] }]
  }
]

const DataSourceCard = ({
  source,
  onClick,
  disabled
}: {
  source: DataSource
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
      {source.icon}
      <span className="font-semibold text-sm">{source.label}</span>
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
    <div className="p-4 rounded-lg bg-card border h-full flex items-center justify-between gap-3">
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
  if (!sourceDetails || !source.isIndexed) return null

  return (
    <div className="p-4 rounded-lg bg-muted/50 border h-full flex items-center justify-between gap-3">
      <div className="flex items-center gap-3">
        <div className="flex shrink-0 items-center gap-2">{sourceDetails.icon}</div>
        <div>
          <h3 className="font-medium">{source.name}</h3>
          {source.hasError && <p className="text-xs text-red-500">Error</p>}
          {!source.isIndexed ? (
            <div className="w-full bg-secondary rounded-full h-1 mt-2">
              <div
                className="bg-primary h-1 rounded-full transition-all duration-300"
                style={{
                  width: `${source.indexProgress}%`
                }}
              />
            </div>
          ) : (
            <p className="text-xs text-muted-foreground">
              {format(source.updatedAt, 'MMM d, yyyy')}
            </p>
          )}
        </div>
      </div>
      <div className="flex items-center gap-1 text-xs text-muted-foreground">
        {source.isIndexed && <CheckCircle2 className="h-3 w-3 text-green-500" />}
        <span>{source.isIndexed ? '' : source.isProcessed ? 'Processing' : 'Pending'}</span>
      </div>
      {/* {!source.isProcessed && !source.isIndexed && (
        <Button variant="ghost" size="icon" className="h-6 w-6" onClick={onRemove}>
          <X className="h-4 w-4 text-muted-foreground" />
        </Button>
      )} */}
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
  const { data } = useQuery(GetDataSourcesDocument)
  const { data: indexingData } = useSubscription(IndexingStatusDocument)
  const [addDataSource] = useMutation(ADD_DATA_SOURCE)
  const [removeDataSource] = useMutation(DELETE_DATA_SOURCE)
  const [startIndexing] = useMutation(START_INDEXING)
  const [selectedSource, setSelectedSource] = useState<DataSource | null>(null)
  const [pendingDataSources, setPendingDataSources] = useState<Record<string, PendingDataSource>>(
    {}
  )

  // Derived states from subscription data
  const isIndexing =
    indexingData?.indexingStatus?.status === IndexingState.IndexingData ||
    indexingData?.indexingStatus?.status === IndexingState.DownloadingModel ||
    indexingData?.indexingStatus?.status === IndexingState.ProcessingData

  const isProcessing =
    indexingData?.indexingStatus?.dataSources?.some((source) => !source.isProcessed) ?? false

  console.log('processing', isProcessing)
  const hasError = indexingData?.indexingStatus?.error ?? false
  const hasPendingDataSources = Object.keys(pendingDataSources).length > 0
  const allSourcesIndexed =
    indexingData?.indexingStatus?.dataSources?.every((source) => source.isIndexed) ?? false

  const handleRemoveDataSource = useCallback(
    async (name: string) => {
      await removeDataSource({ variables: { name } })
      setPendingDataSources((prev) => {
        const newState = { ...prev }
        delete newState[name]
        return newState
      })
      onDataSourceRemoved?.(name)
    },
    [onDataSourceRemoved, removeDataSource]
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
      }
      return true
    } catch (error) {
      console.error('Error adding data sources:', error)
      toast.error('Failed to add data sources. Please try again.')
      return false
    }
  }, [addDataSource, pendingDataSources])

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
      const sourcesAdded = await addDataSources()
      if (!sourcesAdded) return

      toast.success('Data sources added successfully')

      const indexingStarted = await startIndexingProcess()
      if (!indexingStarted) return
    } catch (error) {
      console.error('Error in indexing process:', error)
      toast.error('An error occurred during the indexing process')
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
    if (allSourcesIndexed && isIndexing) {
      onIndexingComplete?.()
    }
  }, [allSourcesIndexed, isIndexing, onIndexingComplete])

  const renderIndexingProgress = () => {
    if (!isIndexing) return null

    return (
      <div className="flex flex-col gap-4">
        {indexingData?.indexingStatus?.dataSources?.map((source) => {
          // Calculate progress:
          // - 10% for processing
          // - 90% for indexing
          const processingProgress = source.isProcessed ? 10 : 0
          const indexingProgress = source.isIndexed
            ? 90
            : source.indexProgress
              ? source.indexProgress * 0.9
              : 0
          const totalProgress = processingProgress + indexingProgress

          return (
            <div key={source.id} className="flex flex-col gap-2">
              <div className="flex items-center justify-between">
                <span className="font-medium">{source.name}</span>
                <span className="text-sm text-muted-foreground">
                  {source.isIndexed ? 'Indexed' : source.isProcessed ? 'Processing' : 'Pending'}
                </span>
              </div>
              <div className="w-full bg-secondary rounded-full h-2">
                <div
                  className="bg-primary h-2 rounded-full transition-all duration-300"
                  style={{
                    width: `${totalProgress}%`
                  }}
                />
              </div>
            </div>
          )
        })}
      </div>
    )
  }

  return (
    <Card className="flex flex-col gap-6 p-6 max-w-3xl">
      {header && (
        <div className="flex flex-col gap-2 items-center">
          <Import className="w-6 h-6 text-primary" />
          <h2 className="text-2xl font-medium">Import your history</h2>
          <p className="text-muted-foreground text-balance max-w-md text-center">
            Your imported data will be used to power your Twin&apos;s understanding of you.
          </p>
        </div>
      )}
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-6">
        {SUPPORTED_DATA_SOURCES.map((source) => (
          <DataSourceCard
            key={source.name}
            source={source}
            disabled={isIndexing}
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
            {showStatus &&
              data?.getDataSources?.map((source) => (
                <IndexedDataSourceCard
                  key={source.id}
                  // onRemove={() => handleRemoveDataSource(source.name)}
                  source={{
                    ...source,
                    indexProgress: indexingData?.indexingStatus?.dataSources?.find(
                      (s) => s.id === source.id
                    )?.indexProgress
                  }}
                />
              ))}
          </div>
        </div>
      )}

      {renderIndexingProgress()}

      <Button
        size="lg"
        onClick={handleStartIndexing}
        disabled={isIndexing || isProcessing || !hasPendingDataSources}
        className="w-full"
      >
        {isProcessing ? (
          <>
            <Loader2 className="mr-2 h-4 w-4 animate-spin" />
            Processing...
          </>
        ) : (
          <>
            Begin import <Play className="ml-2 h-4 w-4" />
          </>
        )}
      </Button>

      {hasError && (
        <div className="flex gap-2">
          <Button variant="outline" onClick={handleRetryIndexing} className="flex-1">
            <RefreshCw className="mr-2 h-4 w-4" />
            Retry Indexing
          </Button>
        </div>
      )}

      <DataSourceDialog
        selectedSource={selectedSource}
        onClose={() => setSelectedSource(null)}
        pendingDataSources={pendingDataSources}
        onFileSelect={handleFileSelect}
        onAddSource={handleAddSource}
      />
    </Card>
  )
}
