import { useState } from 'react'

import HolonJoinScreen from './HolonJoinScreen'
import HolonFeed from './HolonFeed'

export default function Holon() {
  // this will be replaced with actual query
  const [hasJoinedHolon, setHasJoinedHolon] = useState(true)

  const joinHolon = () => {
    setHasJoinedHolon(true)
  }

  if (!hasJoinedHolon) {
    return <HolonJoinScreen joinHolon={joinHolon} />
  }

  return <HolonFeed />
}
