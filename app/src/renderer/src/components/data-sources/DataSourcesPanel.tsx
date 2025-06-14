import { useQuery, useMutation } from '@apollo/client'
import { GetDataSourcesDocument, IndexingState } from '@renderer/graphql/generated/graphql'
import { useIndexingStatus } from '@renderer/hooks/useIndexingStatus'
import { History } from 'lucide-react'
import { useState, useCallback, useEffect, useMemo } from 'react'
import { DataSource, DataSourcesPanelProps, PendingDataSource, IndexedDataSource } from './types'
import { toast } from 'sonner'
import { gql } from '@apollo/client'
import { Card } from '../ui/card'
import { DataSourceDialog } from './DataSourceDialog'
import { Dialog, DialogContent } from '../ui/dialog'
import { UnifiedDataSourceCard } from './UnifiedDataSourceCard'
import { SUPPORTED_DATA_SOURCES } from './constants'

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

export function DataSourcesPanel({
  onDataSourceSelected,
  onDataSourceRemoved,
  onIndexingComplete,
  header = true
}: Omit<DataSourcesPanelProps, 'indexingStatus' | 'showStatus'> & {
  onIndexingComplete?: () => void
}) {
  const { data, refetch } = useQuery(GetDataSourcesDocument)
  const { data: indexingData } = useIndexingStatus()
  const [addDataSource] = useMutation(ADD_DATA_SOURCE)
  const [startIndexing] = useMutation(START_INDEXING)
  const [selectedSource, setSelectedSource] = useState<DataSource | null>(null)
  const [pendingDataSources, setPendingDataSources] = useState<Record<string, PendingDataSource>>(
    {}
  )
  const [isIndexingInitiated, setIsIndexingInitiated] = useState(false)
  // Store file sizes separately so they persist after clearing pending sources
  const [dataSourceFileSizes, setDataSourceFileSizes] = useState<Record<string, number>>({})

  // Workflow states
  const isIndexing = indexingData?.indexingStatus?.status === IndexingState.IndexingData
  const isProcessing = indexingData?.indexingStatus?.status === IndexingState.ProcessingData
  const isNotStarted = indexingData?.indexingStatus?.status === IndexingState.NotStarted
  const isGlobalProcessing = isIndexing || isProcessing || isNotStarted || isIndexingInitiated

  // Group data sources by type
  const groupedDataSources = useMemo(() => {
    const grouped: Record<string, IndexedDataSource[]> = {}

    if (data?.getDataSources) {
      data.getDataSources.forEach((source) => {
        if (!grouped[source.name]) {
          grouped[source.name] = []
        }
        grouped[source.name].push({
          ...source,
          indexProgress:
            indexingData?.indexingStatus?.dataSources?.find((s) => s.id === source.id)
              ?.indexProgress ?? undefined
        })
      })
    }

    return grouped
  }, [data?.getDataSources, indexingData?.indexingStatus?.dataSources])

  // Check if any source is currently being imported
  const hasActiveImport = Object.values(groupedDataSources).some((sources) =>
    sources.some((source) => source.isProcessed && !source.isIndexed)
  )

  const handleRemoveDataSource = useCallback(
    async (name: string) => {
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

    const result = (await window.api.selectFiles({
      filters: selectedSource.fileFilters
    })) as { canceled: boolean; filePaths: string[]; fileSizes?: number[] }

    if (result.canceled) return

    const path = result.filePaths[0]
    const fileSize = result.fileSizes?.[0] || 0
    console.log('Selected file:', path, 'Size:', fileSize, 'Sizes array:', result.fileSizes)

    // Store file size separately
    if (fileSize > 0) {
      setDataSourceFileSizes((prev) => ({
        ...prev,
        [selectedSource.name]: fileSize
      }))
    }

    setPendingDataSources((prev) => ({
      ...prev,
      [selectedSource.name]: {
        name: selectedSource.name,
        path,
        fileSize
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

  const handleStartImport = useCallback(
    async (sourceName: string) => {
      const pendingSource = pendingDataSources[sourceName]
      if (!pendingSource) return

      try {
        setIsIndexingInitiated(true)

        // Add the data source
        await addDataSource({
          variables: {
            name: pendingSource.name,
            path: pendingSource.path
          }
        })

        window.api.analytics.capture('data_source_added', {
          source: pendingSource.name
        })

        // Clear the pending source
        setPendingDataSources((prev) => {
          const newState = { ...prev }
          delete newState[sourceName]
          return newState
        })

        // Refetch data sources
        await refetch()

        // Start indexing
        await startIndexing()

        toast.success(`Started importing ${sourceName}`)
      } catch (error) {
        console.error('Error starting import:', error)
        toast.error('Failed to start import. Please try again.')
      } finally {
        setIsIndexingInitiated(false)
      }
    },
    [addDataSource, pendingDataSources, refetch, startIndexing]
  )

  // Reset loading state when actual status is received
  useEffect(() => {
    if (isIndexing || isProcessing || isNotStarted) {
      setIsIndexingInitiated(false)
    }
  }, [isIndexing, isProcessing, isNotStarted])

  // Handle indexing completion
  useEffect(() => {
    const hasDataSources = (indexingData?.indexingStatus?.dataSources?.length ?? 0) > 0
    const allSourcesIndexed =
      indexingData?.indexingStatus?.dataSources?.every((source) => source.isIndexed) ?? false
    if (!isIndexing && allSourcesIndexed && hasDataSources) {
      onIndexingComplete?.()
    }
  }, [isIndexing, onIndexingComplete, indexingData?.indexingStatus?.dataSources])

  return (
    <div className="flex flex-col gap-6 max-w-6xl">
      {header && (
        <Card className="p-6">
          <div className="flex flex-col gap-2 items-center">
            <History className="w-6 h-6 text-primary" />
            <h2 className="text-2xl font-semibold">Import your history</h2>
            <p className="text-muted-foreground text-balance max-w-md text-center">
              Your imported data will be used to power your Twin&apos;s understanding of you.
            </p>
          </div>
        </Card>
      )}

      <div className="grid grid-cols-1 gap-4">
        {SUPPORTED_DATA_SOURCES.map((source) => (
          <UnifiedDataSourceCard
            key={source.name}
            source={source}
            indexedSources={groupedDataSources[source.name] || []}
            pendingSource={pendingDataSources[source.name]}
            fileSize={dataSourceFileSizes[source.name]}
            isImporting={hasActiveImport}
            isGlobalProcessing={isGlobalProcessing}
            onSelect={handleSourceSelected}
            onRemovePending={() => handleRemoveDataSource(source.name)}
            onStartImport={() => handleStartImport(source.name)}
          />
        ))}
      </div>

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
    </div>
  )
}
