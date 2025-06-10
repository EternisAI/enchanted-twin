import { createFileRoute } from '@tanstack/react-router'
import { client } from '@renderer/graphql/lib'
import { GetThreadDocument } from '@renderer/graphql/generated/graphql'
import HolonThreadDetail from '@renderer/components/holon/HolonThreadDetail'

export const Route = createFileRoute('/holon/$threadId')({
  component: HolonThreadDetailPage,
  loader: async ({ params }) => {
    try {
      const { data } = await client.query({
        query: GetThreadDocument,
        variables: { id: params.threadId, network: null },
        fetchPolicy: 'network-only'
      })

      return {
        data: data.getThread,
        loading: false,
        error: null
      }
    } catch (error: unknown) {
      return {
        data: null,
        loading: false,
        error: error instanceof Error ? error.message : 'An unknown error occurred'
      }
    }
  },
  pendingComponent: () => {
    return (
      <div className="flex flex-col items-center justify-center h-full">
        <div className="flex items-center justify-center gap-2 h-20">
          {[...Array(3)].map((_, i) => (
            <div
              key={i}
              className="h-3 w-3 bg-green-500 rounded-full animate-bounce"
              style={{ animationDelay: `${i * 0.15}s` }}
            />
          ))}
        </div>
      </div>
    )
  },
  pendingMs: 100,
  pendingMinMs: 300
})

export default function HolonThreadDetailPage() {
  const { data, error } = Route.useLoaderData()

  if (!data) return <div className="p-4">Invalid thread ID.</div>
  if (error) return <div className="p-4 text-red-500">Error loading thread.</div>

  return <HolonThreadDetail thread={data} />
}
