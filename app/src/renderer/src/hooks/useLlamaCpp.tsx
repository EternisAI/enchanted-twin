import { useState, useEffect, useCallback } from 'react'

interface LlamaCppStatus {
  isRunning: boolean
  isSetup: boolean
  setupInProgress: boolean
}

interface LlamaCppState {
  status: LlamaCppStatus
  loading: boolean
  error: string | null
}

export function useLlamaCpp() {
  const [state, setState] = useState<LlamaCppState>({
    status: {
      isRunning: false,
      isSetup: false,
      setupInProgress: false
    },
    loading: true,
    error: null
  })

  const fetchStatus = useCallback(async () => {
    try {
      const result = await window.api.llamacpp.getStatus()

      if (result.success) {
        setState((prev) => ({
          ...prev,
          status: {
            isRunning: result.isRunning,
            isSetup: result.isSetup,
            setupInProgress: result.setupInProgress
          },
          loading: false
        }))
      } else {
        setState((prev) => ({
          ...prev,
          error: result.error || 'Failed to get LlamaCpp status',
          loading: false
        }))
      }
    } catch (error) {
      setState((prev) => ({
        ...prev,
        error: error instanceof Error ? error.message : 'Unknown error occurred',
        loading: false
      }))
    }
  }, [])

  const start = useCallback(async () => {
    try {
      setState((prev) => ({ ...prev, loading: true, error: null }))
      const result = await window.api.llamacpp.start()

      if (result.success) {
        await fetchStatus()
      } else {
        setState((prev) => ({
          ...prev,
          error: result.error || 'Failed to start LlamaCpp server',
          loading: false
        }))
      }
    } catch (error) {
      setState((prev) => ({
        ...prev,
        error: error instanceof Error ? error.message : 'Unknown error occurred',
        loading: false
      }))
    }
  }, [fetchStatus])

  const cleanup = useCallback(async () => {
    try {
      setState((prev) => ({ ...prev, loading: true, error: null }))
      const result = await window.api.llamacpp.cleanup()

      if (result.success) {
        await fetchStatus()
      } else {
        setState((prev) => ({
          ...prev,
          error: result.error || 'Failed to cleanup LlamaCpp server',
          loading: false
        }))
      }
    } catch (error) {
      setState((prev) => ({
        ...prev,
        error: error instanceof Error ? error.message : 'Unknown error occurred',
        loading: false
      }))
    }
  }, [fetchStatus])

  useEffect(() => {
    fetchStatus()

    const interval = setInterval(() => {
      fetchStatus()
    }, 10000)

    return () => clearInterval(interval)
  }, [fetchStatus])

  return {
    ...state,
    start,
    cleanup,
    refetch: fetchStatus
  }
}
