import { useState, useCallback } from 'react'

interface GoServerState {
  initializing: boolean
  isRunning: boolean
  error?: string
}

export interface GoServerStatus {
  success: boolean
  isRunning: boolean
  message: string
}

export interface GoServerResponse {
  success: boolean
  error?: string
}

export async function initializeGoServer(): Promise<GoServerResponse> {
  try {
    return await window.api.goServer.initialize()
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : 'Unknown error'
    }
  }
}

export async function cleanupGoServer(): Promise<GoServerResponse> {
  try {
    return await window.api.goServer.cleanup()
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error.message : 'Unknown error'
    }
  }
}

export async function getGoServerStatus(): Promise<GoServerStatus> {
  try {
    return await window.api.goServer.getStatus()
  } catch (error) {
    return {
      success: false,
      isRunning: false,
      message: error instanceof Error ? error.message : 'Unknown error'
    }
  }
}

export function useGoServer() {
  const [state, setState] = useState<GoServerState>({
    initializing: false,
    isRunning: false
  })

  const checkStatus = useCallback(async () => {
    try {
      const status = await getGoServerStatus()
      setState((prev) => ({
        ...prev,
        isRunning: status.isRunning,
        error: status.success ? undefined : status.message
      }))
      return status
    } catch (error) {
      setState((prev) => ({
        ...prev,
        isRunning: false,
        error: error instanceof Error ? error.message : 'Unknown error'
      }))
      throw error
    }
  }, [])

  const start = useCallback(async () => {
    try {
      const status = await getGoServerStatus()

      if (status.isRunning) {
        setState({ initializing: false, isRunning: true })
        return { success: true }
      }

      setState((prev) => ({ ...prev, initializing: true, error: undefined }))

      const result = await initializeGoServer()

      if (result.success) {
        setState({ initializing: false, isRunning: true })
      } else {
        setState({
          initializing: false,
          isRunning: false,
          error: result.error
        })
      }
      return result
    } catch (error) {
      setState({
        initializing: false,
        isRunning: false,
        error: error instanceof Error ? error.message : 'Unknown error'
      })
      throw error
    }
  }, [])

  const stop = useCallback(async () => {
    try {
      setState((prev) => ({ ...prev, initializing: true, error: undefined }))

      const result = await cleanupGoServer()
      if (result.success) {
        setState({ initializing: false, isRunning: false })
      } else {
        setState((prev) => ({
          ...prev,
          initializing: false,
          error: result.error
        }))
      }
      return result
    } catch (error) {
      setState((prev) => ({
        ...prev,
        initializing: false,
        error: error instanceof Error ? error.message : 'Unknown error'
      }))
      throw error
    }
  }, [])

  const initializeIfNeeded = useCallback(async () => {
    if (state.initializing) {
      return { success: true }
    }

    try {
      setState((prev) => ({ ...prev, initializing: true, error: undefined }))
      console.log('[useGoServer] Initializing Go Server')
      const status = await getGoServerStatus()
      if (status.isRunning) {
        setState({ initializing: false, isRunning: true })
        return { success: true }
      }

      return await start()
    } catch (error) {
      setState({
        initializing: false,
        isRunning: false,
        error: error instanceof Error ? error.message : 'Unknown error'
      })
      throw error
    }
  }, [start, state.initializing])

  const retry = useCallback(async () => {
    setState((prev) => ({ ...prev, error: undefined }))
    return await initializeIfNeeded()
  }, [initializeIfNeeded])

  return {
    state,
    checkStatus,
    start,
    stop,
    initializeIfNeeded,
    retry
  }
}
