import { getThreadById } from './data'
import HolonFeedThread from './HolonFeedThread'

interface HolonThreadContextProps {
  threadId: string
}

export default function HolonThreadContext({ threadId }: HolonThreadContextProps) {
  const threadData = getThreadById(threadId)

  if (!threadData) {
    return null
  }

  return <HolonFeedThread thread={threadData} collapsed />
}
