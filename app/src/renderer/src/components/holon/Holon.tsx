import { useState } from 'react'
import HolonJoinScreen from './HolonJoinScreen'
import HolonFeed from './HolonFeed'

export default function Holon() {
  // Mock boolean - this will be replaced with actual query later
  const [hasJoinedHolon] = useState(true)

  if (!hasJoinedHolon) {
    return <HolonJoinScreen />
  }

  return <HolonFeed />
}
