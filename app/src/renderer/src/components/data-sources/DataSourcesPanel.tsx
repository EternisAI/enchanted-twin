import { useQuery, useMutation } from '@apollo/client'
import { GetDataSourcesDocument, IndexingState } from '@renderer/graphql/generated/graphql'
import { useIndexingStatus } from '@renderer/hooks/useIndexingStatus'
import { useIndexingStore } from '@renderer/stores/indexingStore'
import { History } from 'lucide-react'
import { useState, useCallback, useEffect, useMemo, useReducer, useRef } from 'react'
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

// Reducer for managing initiating data sources
type InitiatingAction =
  | { type: 'ADD'; name: string }
  | { type: 'REMOVE'; name: string }
  | { type: 'REMOVE_MULTIPLE'; names: string[] }
  | { type: 'CLEAR' }
  | { type: 'SYNC_WITH_BACKEND'; backendSources: string[] }

function initiatingReducer(state: Set<string>, action: InitiatingAction): Set<string> {
  switch (action.type) {
    case 'ADD': {
      // Don't modify state if the item already exists
      if (state.has(action.name)) return state
      const newState = new Set(state)
      newState.add(action.name)
      return newState
    }
    case 'REMOVE': {
      // Don't modify state if the item doesn't exist
      if (!state.has(action.name)) return state
      const newState = new Set(state)
      newState.delete(action.name)
      return newState
    }
    case 'REMOVE_MULTIPLE': {
      // Only create new state if there's something to remove
      const itemsToRemove = action.names.filter((name) => state.has(name))
      if (itemsToRemove.length === 0) return state
      const newState = new Set(state)
      itemsToRemove.forEach((name) => newState.delete(name))
      return newState
    }
    case 'CLEAR':
      // Don't create new state if already empty
      return state.size === 0 ? state : new Set()
    case 'SYNC_WITH_BACKEND': {
      // Only create new state if there's something to remove
      const itemsToRemove = action.backendSources.filter((name) => state.has(name))
      if (itemsToRemove.length === 0) return state
      const newState = new Set(state)
      itemsToRemove.forEach((name) => newState.delete(name))
      return newState
    }
    default:
      return state
  }
}

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
  const { clearStartTimes } = useIndexingStore()
  const [addDataSource] = useMutation(ADD_DATA_SOURCE)
  const [startIndexing] = useMutation(START_INDEXING)
  const [selectedSource, setSelectedSource] = useState<DataSource | null>(null)
  const [pendingDataSources, setPendingDataSources] = useState<Record<string, PendingDataSource>>(
    {}
  )
  // Track which data sources are being initiated (waiting for backend confirmation)
  const [initiatingDataSources, dispatchInitiating] = useReducer(
    initiatingReducer,
    new Set<string>()
  )
  // Store file sizes separately so they persist after clearing pending sources
  const [dataSourceFileSizes, setDataSourceFileSizes] = useState<Record<string, number>>({})

  // Workflow states
  const isIndexing = indexingData?.indexingStatus?.status === IndexingState.IndexingData
  const isProcessing = indexingData?.indexingStatus?.status === IndexingState.ProcessingData
  const isNotStarted = indexingData?.indexingStatus?.status === IndexingState.NotStarted
  const isGlobalProcessing =
    isIndexing || isProcessing || isNotStarted || initiatingDataSources.size > 0

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

  const handleFileDrop = useCallback(async (files: File[], sourceName: string) => {
    const source = SUPPORTED_DATA_SOURCES.find((s) => s.name === sourceName)
    if (!source) return

    if (source.fileFilters && source.fileFilters.length > 0) {
      const allowedExtensions = source.fileFilters.flatMap((filter) =>
        filter.extensions.map((ext) => ext.toLowerCase())
      )

      const invalidFiles = files.filter((file) => {
        const extension = file.name.split('.').pop()?.toLowerCase()
        return !extension || !allowedExtensions.includes(extension)
      })

      if (invalidFiles.length > 0) {
        throw new Error(
          `Invalid file type. Please select ${source.fileFilters.map((f) => f.extensions.join(', ')).join(' or ')} files.`
        )
      }
    }

    const firstFile = files[0]

    const filePath = (window.api.getPathForFile as unknown as (file: File) => string)(firstFile)

    const savedPaths = await window.api.copyDroppedFiles([filePath])

    if (savedPaths.length > 0) {
      setPendingDataSources((prev) => ({
        ...prev,
        [sourceName]: {
          name: sourceName,
          path: savedPaths[0]
        }
      }))
    }
  }, [])

  const handleStartImport = useCallback(
    async (sourceName: string) => {
      const pendingSource = pendingDataSources[sourceName]
      if (!pendingSource) return

      try {
        // Mark this data source as initiating
        dispatchInitiating({ type: 'ADD', name: sourceName })

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
        // Remove from initiating set on error
        dispatchInitiating({ type: 'REMOVE', name: sourceName })
      }
    },
    [addDataSource, pendingDataSources, refetch, startIndexing]
  )

  // Use a ref to track the last known backend sources to avoid unnecessary dispatches
  const lastBackendSourcesRef = useRef<string[]>([])

  // Consolidated effect for managing initiating data sources
  useEffect(() => {
    // Check if any initiating sources now exist in the backend
    if (data?.getDataSources) {
      const backendSourceNames = data.getDataSources.map((s) => s.name)
      const backendSourcesChanged =
        backendSourceNames.length !== lastBackendSourcesRef.current.length ||
        backendSourceNames.some((name, idx) => name !== lastBackendSourcesRef.current[idx])
      if (backendSourcesChanged && initiatingDataSources.size > 0) {
        // Only dispatch if there are actually sources to sync
        const sourcesToRemove = Array.from(initiatingDataSources).filter((name) =>
          backendSourceNames.includes(name)
        )
        if (sourcesToRemove.length > 0) {
          dispatchInitiating({ type: 'REMOVE_MULTIPLE', names: sourcesToRemove })
        }
        lastBackendSourcesRef.current = backendSourceNames
      }
    }

    // Check if workflow is complete
    const status = indexingData?.indexingStatus?.status
    const isWorkflowComplete =
      status === IndexingState.Completed ||
      status === IndexingState.Failed ||
      (!isIndexing && !isProcessing && !isNotStarted)

    if (isWorkflowComplete && initiatingDataSources.size > 0) {
      dispatchInitiating({ type: 'CLEAR' })
      clearStartTimes()
    }
  }, [
    data?.getDataSources,
    indexingData?.indexingStatus?.status,
    isIndexing,
    isProcessing,
    isNotStarted,
    initiatingDataSources.size,
    clearStartTimes
  ])

  // Separate timeout mechanism with cleanup
  useEffect(() => {
    if (initiatingDataSources.size === 0) return

    const timeouts = new Map<string, NodeJS.Timeout>()
    // Set individual timeouts for each initiating source
    initiatingDataSources.forEach((sourceName) => {
      const timeout = setTimeout(() => {
        dispatchInitiating({ type: 'REMOVE', name: sourceName })
        console.warn(`Removing stuck initiating data source: ${sourceName}`)
      }, 30000) // 30 seconds timeout
      timeouts.set(sourceName, timeout)
    })

    return () => {
      // Clear all timeouts on cleanup
      timeouts.forEach((timeout) => clearTimeout(timeout))
    }
  }, [initiatingDataSources])

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
            isInitiating={initiatingDataSources.has(source.name)}
            onSelect={handleSourceSelected}
            onRemovePending={() => handleRemoveDataSource(source.name)}
            onStartImport={() => handleStartImport(source.name)}
          />
        ))}
      </div>

      <Dialog open={!!selectedSource} onOpenChange={() => setSelectedSource(null)}>
        <DialogContent>
          <DataSourceDialog
            selectedSource={selectedSource}
            onClose={() => setSelectedSource(null)}
            pendingDataSources={pendingDataSources}
            onFileSelect={handleFileSelect}
            onAddSource={handleAddSource}
            onFileDrop={handleFileDrop}
            customComponent={selectedSource?.customView ? selectedSource.customView : undefined}
          />
        </DialogContent>
      </Dialog>
    </div>
  )
}
