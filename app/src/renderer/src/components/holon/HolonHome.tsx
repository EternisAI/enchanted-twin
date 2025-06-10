import HolonJoinScreen from './HolonJoinScreen'
import HolonFeed from './HolonFeed'
import { useMutation, useQuery } from '@apollo/client'
import { GetHolonsDocument, JoinHolonDocument } from '@renderer/graphql/generated/graphql'
import { useEffect, useState } from 'react'
import { toast } from 'sonner'

const DEFAULT_HOLON_NETWORK = 'holon-default-network'

export default function Holon() {
  const { distinctId } = useDistinctId()

  const { data: hasJoinedHolon, refetch: refetchHasJoinedHolon } = useQuery(GetHolonsDocument, {
    variables: { userId: distinctId },
    skip: !distinctId
  })

  const [joinHolonMutation, { loading: joinHolonLoading }] = useMutation(JoinHolonDocument, {
    onCompleted: () => {
      refetchHasJoinedHolon()
      toast.success('Joined holon!')
    },
    onError: (error) => {
      toast.error(`Error joining holon: ${error.message}`)
    }
  })

  const joinHolon = () => {
    joinHolonMutation({ variables: { userId: distinctId, network: DEFAULT_HOLON_NETWORK } })
  }

  if (!hasJoinedHolon?.getHolons?.length) {
    return <HolonJoinScreen joinHolon={joinHolon} joinHolonLoading={joinHolonLoading} />
  }

  return <HolonFeed />
}

function useDistinctId() {
  const [distinctId, setDistinctId] = useState<string>('')

  useEffect(() => {
    const getDistinctId = async () => {
      const id = await window.api.analytics.getDistinctId()
      setDistinctId(id)
    }
    getDistinctId()
  }, [])

  return { distinctId }
}
