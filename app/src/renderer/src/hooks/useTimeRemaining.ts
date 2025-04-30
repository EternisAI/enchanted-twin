import { useState, useEffect, useRef } from 'react'

interface TimeRemainingState {
  timeRemaining: string
  isCalculating: boolean
}

export function useTimeRemaining(progress: number | undefined) {
  const [state, setState] = useState<TimeRemainingState>({
    timeRemaining: '',
    isCalculating: false
  })
  const startTimeRef = useRef<number | null>(null)
  const lastProgressRef = useRef<number>(0)

  useEffect(() => {
    if (progress === undefined) {
      setState({ timeRemaining: '', isCalculating: false })
      return
    }

    // Reset if progress went backwards
    if (progress < lastProgressRef.current) {
      startTimeRef.current = null
    }

    // Initialize start time if not set
    if (startTimeRef.current === null) {
      startTimeRef.current = Date.now()
      lastProgressRef.current = progress
      setState({ timeRemaining: 'Calculating...', isCalculating: true })
      return
    }

    // Calculate time remaining
    if (progress > 0 && progress < 100) {
      const elapsedTime = Date.now() - startTimeRef.current
      const progressPerMs = progress / elapsedTime
      const remainingProgress = 100 - progress
      const estimatedTimeRemaining = remainingProgress / progressPerMs

      // Format time remaining
      const minutes = Math.floor(estimatedTimeRemaining / 60000)
      const seconds = Math.floor((estimatedTimeRemaining % 60000) / 1000)

      let timeRemaining = ''
      if (minutes > 0) {
        timeRemaining = `${minutes}m ${seconds}s`
      } else {
        timeRemaining = `${seconds}s`
      }

      setState({ timeRemaining, isCalculating: false })
    } else if (progress >= 100) {
      setState({ timeRemaining: 'Complete', isCalculating: false })
    }

    lastProgressRef.current = progress
  }, [progress])

  return state
}
