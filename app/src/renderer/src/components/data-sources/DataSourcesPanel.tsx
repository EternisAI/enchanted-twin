import { useQuery, useMutation, useSubscription } from '@apollo/client'
import { GetDataSourcesDocument, IndexingStatusDocument } from '@renderer/graphql/generated/graphql'
import { Button } from '../ui/button'
import { CheckCircle2, Loader2, X, Play, RefreshCw } from 'lucide-react'
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
    name: 'WhatsApp',
    label: 'WhatsApp',
    description: 'Import your WhatsApp chat history',
    selectType: 'files',
    fileRequirement: 'Select WhatsApp SQLITE file',
    icon: <WhatsAppIcon className="h-5 w-5" />,
    fileFilters: [{ name: 'WhatsApp Database', extensions: ['db', 'sqlite'] }]
  },
  {
    name: 'Telegram',
    label: 'Telegram',
    description: 'Import your Telegram messages and media',
    selectType: 'files',
    fileRequirement: 'Select Telegram JSON export file',
    icon: <TelegramIcon className="h-5 w-5" />,
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
  },
  {
    name: 'X',
    label: 'X',
    description: 'Import your X tweets and messages',
    selectType: 'files',
    fileRequirement: 'Select X ZIP',
    icon: <XformerlyTwitterIcon className="h-5 w-5" />,
    fileFilters: [{ name: 'X Archive', extensions: ['zip'] }]
  }
]

const DataSourceCard = ({ source, onClick }: { source: DataSource; onClick: () => void }) => (
  <Button
    variant="outline"
    size="lg"
    className="h-auto py-4 px-4 flex flex-col items-center gap-2 hover:bg-accent/50 bg-card"
    onClick={onClick}
  >
    <div className="flex items-center gap-2">
      {source.icon}
      <span className="font-medium">{source.label}</span>
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
          <p className="text-xs text-muted-foreground">{sourceDetails.description}</p>
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

const IndexedDataSourceCard = ({ source }: { source: IndexedDataSource }) => {
  const sourceDetails = SUPPORTED_DATA_SOURCES.find((s) => s.name === source.name)
  if (!sourceDetails) return null

  return (
    <div className="p-4 rounded-lg bg-muted/50 border h-full flex items-center justify-between gap-3">
      <div className="flex items-center gap-3">
        {sourceDetails.icon}
        <div>
          <h3 className="font-medium">{source.name}</h3>
          {source.indexProgress !== undefined && (
            <div className="w-full bg-secondary rounded-full h-1 mt-2">
              <div
                className="bg-primary h-1 rounded-full transition-all duration-300"
                style={{
                  width: `${source.indexProgress}%`
                }}
              />
            </div>
          )}
        </div>
      </div>
      <div className="flex items-center gap-1 text-xs text-muted-foreground">
        {source.isIndexed ? (
          <CheckCircle2 className="h-3 w-3 text-green-500" />
        ) : (
          <Loader2 className="h-3 w-3 text-amber-500 animate-spin" />
        )}
        <span>{source.isIndexed ? 'Indexed' : source.isProcessed ? 'Processing' : 'Pending'}</span>
      </div>
    </div>
  )
}

export function DataSourcesPanel({
  onDataSourceSelected,
  onDataSourceRemoved,
  showStatus = false,
  onIndexingComplete
}: Omit<DataSourcesPanelProps, 'indexingStatus'> & { onIndexingComplete?: () => void }) {
  const { data } = useQuery(GetDataSourcesDocument)
  const { data: indexingData } = useSubscription(IndexingStatusDocument)
  const [addDataSource] = useMutation(ADD_DATA_SOURCE)
  const [startIndexing] = useMutation(START_INDEXING)
  const [selectedSource, setSelectedSource] = useState<DataSource | null>(null)
  const [pendingDataSources, setPendingDataSources] = useState<Record<string, PendingDataSource>>(
    {}
  )

  // Derived states from subscription data
  const isIndexing =
    indexingData?.indexingStatus?.dataSources?.some((source) => !source.isIndexed) ?? false
  const isProcessing =
    indexingData?.indexingStatus?.dataSources?.some((source) => !source.isProcessed) ?? false
  const hasError = indexingData?.indexingStatus?.error ?? false
  const hasPendingDataSources = Object.keys(pendingDataSources).length > 0
  const allSourcesIndexed = indexingData?.indexingStatus?.dataSources?.every(
    (source) => source.isIndexed
  )

  const handleRemoveDataSource = useCallback(
    (name: string) => {
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
    <div className="space-y-6">
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {SUPPORTED_DATA_SOURCES.map((source) => (
          <DataSourceCard
            key={source.name}
            source={source}
            onClick={() => handleSourceSelected(source)}
          />
        ))}
      </div>

      {(Object.keys(pendingDataSources).length > 0 ||
        (showStatus && data?.getDataSources && data.getDataSources.length > 0)) && (
        <div className="space-y-4">
          <h3 className="text-sm font-medium text-muted-foreground">
            {showStatus ? 'Data Sources' : 'Selected Data Sources'}
          </h3>
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
                <IndexedDataSourceCard key={source.id} source={source} />
              ))}
          </div>
        </div>
      )}

      {renderIndexingProgress()}

      {!isIndexing && (
        <Button
          onClick={handleStartIndexing}
          disabled={isProcessing || !hasPendingDataSources}
          className="w-full"
        >
          {isProcessing ? (
            <>
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              Processing...
            </>
          ) : (
            <>
              Start Indexing <Play className="ml-2 h-4 w-4" />
            </>
          )}
        </Button>
      )}

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
    </div>
  )
}
