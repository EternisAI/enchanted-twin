import { createFileRoute } from '@tanstack/react-router'

import HolonThreadDetail from '@renderer/components/holon/HolonThreadDetail'

export const Route = createFileRoute('/holon/$threadId')({
  component: HolonThreadDetailPage
})

export default function HolonThreadDetailPage() {
  const { threadId } = Route.useParams()

  return <HolonThreadDetail threadId={threadId} />
}
