import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { IndexingState } from '@renderer/graphql/generated/graphql'

export interface DataSourceProgress {
  id: string
  name: string
  isProcessed?: boolean
  isIndexed?: boolean
  indexProgress?: number | null
  hasError?: boolean
  startTime?: number
}

export interface IndexingStatus {
  status: IndexingState
  dataSources?: DataSourceProgress[]
  lastUpdated: number
  globalStartTime?: number
}

interface IndexingStore {
  indexingStatus: IndexingStatus | null
  updateIndexingStatus: (status: Omit<IndexingStatus, 'lastUpdated'>) => void
  clearIndexingStatus: () => void
  clearStartTimes: () => void
  getDataSourceProgress: (id: string) => DataSourceProgress | undefined
}

export const useIndexingStore = create<IndexingStore>()(
  persist(
    (set, get) => ({
      indexingStatus: null,

      updateIndexingStatus: (status) => {
        const currentState = get()
        const currentStatus = currentState.indexingStatus

        // Preserve globalStartTime if already set, or set it when transitioning to active state
        let globalStartTime = currentStatus?.globalStartTime
        if (
          !globalStartTime &&
          (status.status === IndexingState.ProcessingData ||
            status.status === IndexingState.IndexingData ||
            status.status === IndexingState.DownloadingModel)
        ) {
          globalStartTime = Date.now()
        }

        // Clear global start time when indexing is complete or not started
        if (
          status.status === IndexingState.NotStarted ||
          status.status === IndexingState.Completed ||
          ((status.dataSources?.length ?? 0) > 0 && status.dataSources?.every((ds) => ds.isIndexed))
        ) {
          globalStartTime = undefined
        }

        // Preserve start times for data sources
        const dataSources = status.dataSources?.map((ds) => {
          const existingDs = currentStatus?.dataSources?.find((d) => d.id === ds.id)
          let startTime = existingDs?.startTime

          // Set start time when data source begins processing
          if (!startTime && (ds.isProcessed || ds.isIndexed)) {
            startTime = Date.now()
          }

          return {
            ...ds,
            startTime
          }
        })

        set({
          indexingStatus: {
            ...status,
            dataSources,
            lastUpdated: Date.now(),
            globalStartTime
          }
        })
      },

      clearIndexingStatus: () => {
        set({ indexingStatus: null })
      },

      clearStartTimes: () => {
        const state = get()
        if (state.indexingStatus) {
          set({
            indexingStatus: {
              ...state.indexingStatus,
              globalStartTime: undefined,
              dataSources: state.indexingStatus.dataSources?.map((ds) => ({
                ...ds,
                startTime: undefined
              }))
            }
          })
        }
      },

      getDataSourceProgress: (id) => {
        const state = get()
        return state.indexingStatus?.dataSources?.find((ds) => ds.id === id)
      }
    }),
    {
      name: 'indexing-storage',
      partialize: (state) => ({ indexingStatus: state.indexingStatus })
    }
  )
)
