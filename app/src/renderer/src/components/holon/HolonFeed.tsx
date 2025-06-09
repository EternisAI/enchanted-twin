import { PlusCircle } from 'lucide-react'
import { motion } from 'framer-motion'
import { useMutation, useQuery } from '@apollo/client'
import { useNavigate, useRouter } from '@tanstack/react-router'

import { Button } from '../ui/button'
import HolonFeedThread from './HolonFeedThread'
import {
  ChatCategory,
  CreateChatDocument,
  GetThreadsDocument
} from '@renderer/graphql/generated/graphql'
import { client } from '@renderer/graphql/lib'

export default function HolonFeed() {
  const { data, loading, error } = useQuery(GetThreadsDocument, {
    variables: { network: null },
    fetchPolicy: 'network-only'
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

          {threads.length > 3 && (
            <div className="flex justify-center py-6">
              <Button variant="default" size="sm">
                <PlusCircle className="w-4 h-4" />
                Fetch More
              </Button>
            </div>
          )}
        </div>
      </div>
    </motion.div>
  )
}
