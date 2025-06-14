import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { IndexingState } from '@renderer/graphql/generated/graphql'

export interface DataSourceProgress {
  id: string
  name: string
  isProcessed?: boolean
  isIndexed?: boolean
  indexProgress?: number | null
}

export interface IndexingStatus {
  status: IndexingState
  dataSources?: DataSourceProgress[]
  lastUpdated: number
}

interface IndexingStore {
  indexingStatus: IndexingStatus | null
  updateIndexingStatus: (status: Omit<IndexingStatus, 'lastUpdated'>) => void
  clearIndexingStatus: () => void
  getDataSourceProgress: (id: string) => DataSourceProgress | undefined
}

export const useIndexingStore = create<IndexingStore>()(
  persist(
    (set, get) => ({
      indexingStatus: null,
      
      updateIndexingStatus: (status) => {
        set({ 
          indexingStatus: {
            ...status,
            lastUpdated: Date.now()
          }
        })
      },
      
      clearIndexingStatus: () => {
        set({ indexingStatus: null })
      },
      
      getDataSourceProgress: (id) => {
        const state = get()
        return state.indexingStatus?.dataSources?.find(ds => ds.id === id)
      }
    }),
    {
      name: 'indexing-storage',
      partialize: (state) => ({ indexingStatus: state.indexingStatus })
    }
  )
) 