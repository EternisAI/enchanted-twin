import { useQuery } from '@apollo/client'
import HolonFeedThread from './HolonFeedThread'
import { GetThreadDocument } from '@renderer/graphql/generated/graphql'

interface HolonThreadContextProps {
  threadId: string
}

export default function HolonThreadContext({ threadId }: HolonThreadContextProps) {
  const { data: threadData } = useQuery(GetThreadDocument, {
    variables: { id: threadId }
  })

  if (!threadData?.getThread) {
    return null
  }

  return <HolonFeedThread thread={threadData.getThread} collapsed />
}
