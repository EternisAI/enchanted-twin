import { useEffect, useMemo, useState } from 'react'
import { useModalZIndexStore } from '@renderer/lib/stores/modalZIndexStore'

let modalInstanceCounter = 0
const generateModalId = (): string => `managed-modal-${modalInstanceCounter++}`

const CONTENT_Z_OFFSET = 5 // Content will be this much higher than its overlay

export function useManagedModalZIndex() {
  const [modalId] = useState(generateModalId)
  const [assignedBaseZIndex, setAssignedBaseZIndex] = useState<number | null>(null)

  const requestZIndex = useModalZIndexStore((state) => state.requestZIndex)
  const releaseZIndex = useModalZIndexStore((state) => state.releaseZIndex)

  useEffect(() => {
    const newZ = requestZIndex(modalId)
    setAssignedBaseZIndex(newZ)

    return () => {
      releaseZIndex(modalId)
    }
  }, [modalId, requestZIndex, releaseZIndex])

  return useMemo(() => {
    if (assignedBaseZIndex === null) {
      return { overlayZIndex: undefined, contentZIndex: undefined }
    }
    return {
      overlayZIndex: assignedBaseZIndex,
      contentZIndex: assignedBaseZIndex + CONTENT_Z_OFFSET
    }
  }, [assignedBaseZIndex])
} 