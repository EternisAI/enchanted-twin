import { PlusCircle, Loader2 } from 'lucide-react'
import { motion } from 'framer-motion'
import { useMutation, useQuery } from '@apollo/client'
import { useNavigate, useRouter } from '@tanstack/react-router'
import { useState } from 'react'

import { Button } from '../ui/button'
import HolonFeedThread from './HolonFeedThread'
import {
  ChatCategory,
  CreateChatDocument,
  GetThreadsDocument
} from '@renderer/graphql/generated/graphql'
import { client } from '@renderer/graphql/lib'

const THREADS_PER_PAGE = 10

export default function HolonFeed() {
  const [fetchingMore, setFetchingMore] = useState(false)
  const [hasMore, setHasMore] = useState(true)

  const { data, loading, error, fetchMore } = useQuery(GetThreadsDocument, {
    variables: {
      network: null,
      first: THREADS_PER_PAGE,
      offset: 0
    },
    fetchPolicy: 'network-only',
    pollInterval: 20000
  })

  const router = useRouter()
  const navigate = useNavigate()
  const [createChat] = useMutation(CreateChatDocument)

  if (loading) {
    return (
      <div className="flex flex-col items-center justify-center min-h-screen">
        <div className="text-muted-foreground">Loading threads...</div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center min-h-screen">
        <div className="text-destructive">Error loading threads: {error?.message}</div>
      </div>
    )
  }

  const threads = data?.getThreads || []

  const handleCreateChat = async () => {
    try {
      const { data: createData } = await createChat({
        variables: { name: 'holon-new-feed', category: ChatCategory.Text }
      })
      const newChatId = createData?.createChat?.id

      if (newChatId) {
        navigate({
          to: `/chat/${newChatId}`
        })

        await client.cache.evict({ fieldName: 'getChats' })
        await router.invalidate({
          filter: (match) => match.routeId === '/chat/$chatId'
        })
      }
    } catch (error) {
      console.error('Failed to create chat:', error)
    }
  }

  const handleFetchMore = async () => {
    if (fetchingMore) return

    setFetchingMore(true)

    try {
      await fetchMore({
        variables: {
          network: null,
          first: THREADS_PER_PAGE,
          offset: threads.length
        },
        updateQuery: (prev, { fetchMoreResult }) => {
          if (!fetchMoreResult) return prev

          const newThreads = fetchMoreResult.getThreads || []

          if (newThreads.length < THREADS_PER_PAGE) {
            setHasMore(false)
          }

          return {
            ...prev,
            getThreads: [...(prev.getThreads || []), ...newThreads]
          }
        }
      })
    } catch (error) {
      console.error('Failed to fetch more threads:', error)
    } finally {
      setFetchingMore(false)
    }
  }

  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{ duration: 0.5 }}
      className="flex w-full justify-center overflow-y-auto"
    >
      <div className="max-w-2xl mx-auto p-6 flex flex-col gap-16">
        <div className="flex items-center justify-between">
          <h1 className="text-4xl font-bold text-foreground">Discover & Connect</h1>
          <Button size="sm" className="text-md font-semibold" onClick={handleCreateChat}>
            Create
          </Button>
        </div>

        <div className="flex flex-col gap-6 pb-12">
          {threads.length === 0 ? (
            <div className="text-center w-2xl py-12 text-muted-foreground">
              No threads available yet. <br /> Be the first to create one!
            </div>
          ) : (
            threads.map((thread) => <HolonFeedThread key={thread.id} thread={thread} />)
          )}

          {threads.length >= THREADS_PER_PAGE && hasMore && (
            <div className="flex justify-center py-6">
              <Button variant="default" size="sm" onClick={handleFetchMore} disabled={fetchingMore}>
                {fetchingMore ? (
                  <>
                    <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                    Loading More...
                  </>
                ) : (
                  <>
                    <PlusCircle className="w-4 h-4 mr-2" />
                    Fetch More
                  </>
                )}
              </Button>
            </div>
          )}
        </div>
      </div>
    </motion.div>
  )
}
