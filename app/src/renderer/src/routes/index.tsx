import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { createFileRoute, redirect, useRouterState } from '@tanstack/react-router'
import { client } from '@renderer/graphql/lib'
import { Chat, GetChatsDocument } from '@renderer/graphql/generated/graphql'
import { motion, AnimatePresence, LayoutGroup } from 'framer-motion'
import { Header } from '@renderer/components/chat/Header'
import { History } from 'lucide-react'
import { useState } from 'react'
import { Button } from '@renderer/components/ui/button'
import { HistoryChats } from '@renderer/components/chat/HistoryChats'

function IndexComponent() {
  const { data, error, success } = Route.useLoaderData()
  const chats: Chat[] = data?.getChats || []
  const { location } = useRouterState()
  const [showChats, setShowChats] = useState(false)

  return (
    <LayoutGroup>
      <motion.div className="flex h-full w-full">
        <motion.div className="flex-1 flex flex-col items-center justify-center p-6">
          <motion.div className="w-full max-w-4xl">
            <motion.div
              layout
              className="flex flex-col items-center gap-4"
              transition={{
                layout: { duration: 0.3, ease: [0.4, 0, 0.2, 1] }
              }}
            >
              <Header />
            </motion.div>
          </motion.div>

          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.5 }}
            className="flex flex-col gap-4 w-full max-w-4xl items-center mt-8"
          >
            {!success && (
              <div className="w-full flex justify-center items-center py-10">
                <div className="p-4 m-4 w-xl border border-red-300 bg-red-50 text-red-700 rounded-md">
                  <h3 className="font-medium">Error loading chats</h3>
                  <p className="text-sm">
                    {error instanceof Error ? error.message : 'An unexpected error occurred'}
                  </p>
                </div>
              </div>
            )}

            <AnimatePresence>
              {!showChats ? (
                <Button
                  onClick={() => setShowChats(true)}
                  variant="ghost"
                  size="lg"
                  className="text-muted-foreground hover:text-foreground"
                >
                  <History className="w-4 h-4 mr-2" />
                  <span>History</span>
                </Button>
              ) : (
                <HistoryChats
                  chats={chats}
                  isActive={(path) => location.pathname === path}
                  onClose={() => setShowChats(false)}
                />
              )}
            </AnimatePresence>
          </motion.div>
        </motion.div>
      </motion.div>
    </LayoutGroup>
  )
}

export const Route = createFileRoute('/')({
  loader: async () => {
    try {
      const { data, loading, error } = await client.query({
        query: GetChatsDocument,
        variables: { first: 20, offset: 0 }
      })
      return { data, loading, error, success: true }
    } catch (error) {
      console.error('Error loading chats:', error)
      return {
        data: null,
        loading: false,
        error: error instanceof Error ? error : new Error('An unexpected error occurred'),
        success: false
      }
    }
  },
  component: IndexComponent,
  beforeLoad: () => {
    const onboardingStore = useOnboardingStore.getState()
    if (!onboardingStore.isCompleted) {
      throw redirect({ to: '/onboarding' })
    }
  }
})
