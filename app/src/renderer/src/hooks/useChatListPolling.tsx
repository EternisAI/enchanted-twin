import { useEffect, useState } from 'react'
import { useQuery } from '@apollo/client'
import { GetChatsDocument, Chat } from '@renderer/graphql/generated/graphql'

interface UseChatListPollingOptions {
  /** Polling interval in milliseconds when tab is visible (default: 30000ms = 30s) */
  pollInterval?: number
  /** First/limit parameter for pagination (default: 20) */
  first?: number
  /** Offset parameter for pagination (default: 0) */
  offset?: number
}

export function useChatListPolling(options: UseChatListPollingOptions = {}) {
  const { pollInterval = 30000, first = 20, offset = 0 } = options
  const [isVisible, setIsVisible] = useState(!document.hidden)

  // Use Apollo useQuery with smart polling
  const { data, loading, error, refetch } = useQuery(GetChatsDocument, {
    variables: { first, offset },

    // Only poll when tab is visible to save resources
    pollInterval: isVisible ? pollInterval : 0,

    // Use cache-first to avoid unnecessary network requests
    fetchPolicy: 'cache-first',

    // Don't show loading state during polling to avoid UI flicker
    notifyOnNetworkStatusChange: false,

    // Don't break UI on network errors during polling
    errorPolicy: 'ignore'
  })

  // Track visibility changes
  useEffect(() => {
    const handleVisibilityChange = () => {
      const visible = !document.hidden
      setIsVisible(visible)

      // If tab becomes visible, refetch immediately to get latest data
      if (visible && refetch) {
        refetch()
      }
    }

    document.addEventListener('visibilitychange', handleVisibilityChange)
    return () => document.removeEventListener('visibilitychange', handleVisibilityChange)
  }, [refetch])

  const chats: Chat[] = data?.getChats || []

  return {
    chats,
    loading,
    error,
    refetch,
    isPolling: isVisible && pollInterval > 0
  }
}
