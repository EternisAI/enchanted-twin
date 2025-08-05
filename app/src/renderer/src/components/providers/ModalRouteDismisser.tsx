import { useEffect, useRef } from 'react'
import { useLocation } from '@tanstack/react-router'
import { useModal } from '@renderer/hooks/useModal'

export function ModalRouteDismisser() {
  const location = useLocation()
  const { closeModal } = useModal()
  const isInitialMount = useRef(true)
  const previousLocation = useRef<string>('')

  useEffect(() => {
    // Create a unique identifier for the current location
    const currentLocationKey = `${location.pathname}${location.search}${location.hash}`

    // Skip the very first mount
    if (isInitialMount.current) {
      isInitialMount.current = false
      previousLocation.current = currentLocationKey
      return
    }

    // If location changed, close any open modal
    if (previousLocation.current !== currentLocationKey) {
      closeModal()
      previousLocation.current = currentLocationKey
    }
  }, [location.pathname, location.search, location.hash, closeModal])

  return null
}
