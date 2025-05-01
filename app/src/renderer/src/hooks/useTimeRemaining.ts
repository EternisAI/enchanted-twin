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
  const progressUpdatesRef = useRef<number>(0)
  const MIN_UPDATES = 3 // Wait for at least 3 progress updates before showing estimates

  useEffect(() => {
    if (progress === undefined) {
      setState({ timeRemaining: '', isCalculating: false })
      return
    }

    // Reset if progress went backwards
    if (progress < lastProgressRef.current) {
      startTimeRef.current = null
      progressUpdatesRef.current = 0
    }

    const now = Date.now()

    // Initialize start time if not set
    if (startTimeRef.current === null) {
      startTimeRef.current = now
      lastProgressRef.current = progress
      progressUpdatesRef.current = 1
      setState({ timeRemaining: '...', isCalculating: true })
      return
    }

    // Count progress updates
    if (progress > lastProgressRef.current) {
      progressUpdatesRef.current += 1
    }

    // Calculate time remaining
    if (progress > 0 && progress < 100) {
      const elapsedTime = now - startTimeRef.current
      // If we've made progress and have enough updates, calculate time remaining
      if (progress > lastProgressRef.current && progressUpdatesRef.current >= MIN_UPDATES) {
        const totalTimeForCompletion = (elapsedTime * 100) / progress
        const remainingTime = totalTimeForCompletion - elapsedTime

        // Format time remaining
        const minutes = Math.floor(remainingTime / 60000)
        const seconds = Math.floor((remainingTime % 60000) / 1000)

        let timeRemaining = ''
        if (minutes > 0) {
          timeRemaining = `${minutes}m ${seconds}s`
        } else {
          timeRemaining = `${seconds}s`
        }

        setState({ timeRemaining, isCalculating: false })
      } else {
        // Still warming up
        setState({ timeRemaining: '...', isCalculating: true })
      }
    } else if (progress >= 100) {
      setState({ timeRemaining: 'Complete', isCalculating: false })
    }

    lastProgressRef.current = progress
  }, [progress])

  return state
}
