import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { createFileRoute, redirect } from '@tanstack/react-router'
import { client } from '@renderer/graphql/lib'
import { GetChatsDocument } from '@renderer/graphql/generated/graphql'
import { motion } from 'framer-motion'
import { Home } from '@renderer/components/chat/ChatHome'

// Define expected search params for this route
interface IndexRouteSearch {
  focusInput?: string // boolean as string e.g. "true"
}

function IndexComponent() {
  const { error, success } = Route.useLoaderData()

  return (
    <div className="flex-1 flex flex-col items-center justify-center p-6">
      <div className="w-full max-w-4xl h-full">
        <div className="flex flex-col justify-center items-center gap-4 h-full">
          <Home />
        </div>
      </div>

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
      </motion.div>
    </div>
  )
}

export const Route = createFileRoute('/')({
  validateSearch: (search: Record<string, unknown>): IndexRouteSearch => {
    // Validate and cast search params
    return {
      focusInput: search.focusInput === 'true' ? 'true' : undefined
    }
  },
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
