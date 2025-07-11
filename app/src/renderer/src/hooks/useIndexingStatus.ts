import { useEffect, useState } from 'react'
import { useQuery, useSubscription } from '@apollo/client'
import {
  GetDataSourcesDocument,
  IndexingStatusDocument,
  IndexingState,
  type IndexingStatus as GeneratedIndexingStatus,
  type DataSource as GeneratedDataSource
} from '@renderer/graphql/generated/graphql'

interface ExtendedDataSource extends GeneratedDataSource {
  startTime: number
}

interface ExtendedIndexingStatus extends GeneratedIndexingStatus {
  globalStartTime: number
  dataSources: ExtendedDataSource[]
}

interface IndexingStatusType {
  status: IndexingState
  dataSources: (Omit<ExtendedDataSource, 'path' | 'updatedAt'> & {
    path?: string
    updatedAt?: string
  })[]
  globalStartTime: number
}

export function useIndexingStatus() {
  const { data: sourcesData } = useQuery(GetDataSourcesDocument)
  const { data: subscriptionData, loading, error } = useSubscription(IndexingStatusDocument)
  const [indexingStatus, setIndexingStatus] = useState<IndexingStatusType | null>(null)

  useEffect(() => {
    if (subscriptionData?.indexingStatus) {
      const subStatus = subscriptionData.indexingStatus as ExtendedIndexingStatus
      const mergedSources =
        subStatus.dataSources?.map((ds) => ({
          ...ds,
          path: sourcesData?.getDataSources.find((s) => s.id === ds.id)?.path,
          updatedAt: sourcesData?.getDataSources.find((s) => s.id === ds.id)?.updatedAt
        })) || []

      setIndexingStatus({
        status: subStatus.status,
        dataSources: mergedSources,
        globalStartTime: subStatus.globalStartTime
      })
    }
  }, [subscriptionData, sourcesData])

  return {
    data: indexingStatus ? { indexingStatus } : undefined,
    loading,
    error
  }
}
