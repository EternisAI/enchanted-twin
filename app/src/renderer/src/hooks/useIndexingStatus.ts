import { useEffect } from 'react'
import { useSubscription } from '@apollo/client'
import { IndexingStatusDocument } from '@renderer/graphql/generated/graphql'
import { useIndexingStore } from '@renderer/stores/indexingStore'

export function useIndexingStatus() {
  const { data: subscriptionData, loading, error } = useSubscription(IndexingStatusDocument)
  const { indexingStatus, updateIndexingStatus } = useIndexingStore()

  // Update the store whenever we receive new subscription data
  useEffect(() => {
    if (subscriptionData?.indexingStatus) {
      updateIndexingStatus({
        status: subscriptionData.indexingStatus.status,
        dataSources: subscriptionData.indexingStatus.dataSources?.map((ds) => ({
          id: ds.id,
          name: ds.name,
          isProcessed: ds.isProcessed,
          isIndexed: ds.isIndexed,
          indexProgress: ds.indexProgress,
          hasError: ds.hasError,
          startTime: undefined // Will be set by the store's updateIndexingStatus method
          // Note: The subscription doesn't include all DataSource fields (e.g., path, updatedAt)
          // These are only available from the getDataSources query
        }))
      })
    }
  }, [subscriptionData, updateIndexingStatus])

  // Return the most recent data (either from subscription or store)
  const currentStatus = subscriptionData?.indexingStatus || indexingStatus

  return {
    data: currentStatus ? { indexingStatus: currentStatus } : undefined,
    loading,
    error
  }
}
