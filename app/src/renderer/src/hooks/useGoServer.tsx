import { useState, useCallback, useEffect } from 'react'

export interface GoServerStatus {
  success: boolean
  isRunning: boolean
  isInitializing: boolean
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
      isInitializing: false,
      message: error instanceof Error ? error.message : 'Unknown error'
    }
  }
}

export function useGoServer() {
  const [status, setStatus] = useState<GoServerStatus>({
    success: false,
    isRunning: false,
    isInitializing: false,
    message: 'Loading...'
  })

  const checkStatus = useCallback(async () => {
    try {
      const currentStatus = await getGoServerStatus()
      setStatus(currentStatus)
      return currentStatus
    } catch (error) {
      const errorStatus: GoServerStatus = {
        success: false,
        isRunning: false,
        isInitializing: false,
        message: error instanceof Error ? error.message : 'Unknown error'
      }
      setStatus(errorStatus)
      throw error
    }
  }, [])

  const start = useCallback(async () => {
    try {
      // First check current status
      const currentStatus = await getGoServerStatus()

      if (currentStatus.isRunning) {
        setStatus(currentStatus)
        return { success: true }
      }

      if (currentStatus.isInitializing) {
        setStatus(currentStatus)
        return { success: true }
      }

      // Initialize the server
      const result = await initializeGoServer()

      if (result.success) {
        // Refresh status after successful initialization
        await checkStatus()
      } else {
        // Update status with error
        setStatus((prev) => ({
          ...prev,
          success: false,
          message: result.error || 'Failed to initialize Go server'
        }))
      }

      return result
    } catch (error) {
      const errorStatus: GoServerStatus = {
        success: false,
        isRunning: false,
        isInitializing: false,
        message: error instanceof Error ? error.message : 'Unknown error'
      }
      setStatus(errorStatus)
      throw error
    }
  }, [checkStatus])

  const stop = useCallback(async () => {
    try {
      const result = await cleanupGoServer()

      if (result.success) {
        // Refresh status after successful cleanup
        await checkStatus()
      } else {
        // Update status with error
        setStatus((prev) => ({
          ...prev,
          success: false,
          message: result.error || 'Failed to cleanup Go server'
        }))
      }

      return result
    } catch (error) {
      const errorStatus: GoServerStatus = {
        success: false,
        isRunning: false,
        isInitializing: false,
        message: error instanceof Error ? error.message : 'Unknown error'
      }
      setStatus(errorStatus)
      throw error
    }
  }, [checkStatus])

  const initializeIfNeeded = useCallback(async () => {
    try {
      // Check current status first
      const currentStatus = await getGoServerStatus()

      if (currentStatus.isRunning || currentStatus.isInitializing) {
        setStatus(currentStatus)
        return { success: true }
      }

      // Initialize if not running and not initializing
      return await start()
    } catch (error) {
      const errorStatus: GoServerStatus = {
        success: false,
        isRunning: false,
        isInitializing: false,
        message: error instanceof Error ? error.message : 'Unknown error'
      }
      setStatus(errorStatus)
      throw error
    }
  }, [start])

  const retry = useCallback(async () => {
    return await initializeIfNeeded()
  }, [initializeIfNeeded])

  useEffect(() => {
    checkStatus()
  }, [checkStatus])

  return {
    state: {
      initializing: status.isInitializing,
      isRunning: status.isRunning,
      error: status.success ? undefined : status.message
    },
    checkStatus,
    start,
    stop,
    initializeIfNeeded,
    retry
  }
}
