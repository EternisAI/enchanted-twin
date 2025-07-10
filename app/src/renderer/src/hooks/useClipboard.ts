import { useState, useCallback } from 'react'

interface UseClipboardOptions {
  /**
   * Duration in milliseconds to show the "copied" state
   * @default 2000
   */
  successDuration?: number
  /**
   * Callback when text is successfully copied
   */
  onSuccess?: () => void
  /**
   * Callback when copy fails
   */
  onError?: (error: string) => void
}

interface UseClipboardReturn {
  /**
   * Copy text to clipboard
   */
  copy: (text: string) => Promise<void>
  /**
   * Read text from clipboard
   */
  read: () => Promise<string | undefined>
  /**
   * Whether text was recently copied
   */
  copied: boolean
  /**
   * Error message if copy/read failed
   */
  error: string | null
  /**
   * Loading state for async operations
   */
  isLoading: boolean
}

/**
 * Hook for clipboard operations with Electron IPC
 *
 * @example
 * ```tsx
 * const { copy, copied } = useClipboard()
 *
 * return (
 *   <button onClick={() => copy('Hello World')}>
 *     {copied ? 'Copied!' : 'Copy'}
 *   </button>
 * )
 * ```
 */
export function useClipboard(options: UseClipboardOptions = {}): UseClipboardReturn {
  const { successDuration = 2000, onSuccess, onError } = options

  const [copied, setCopied] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(false)

  const copy = useCallback(
    async (text: string) => {
      if (!text) {
        setError('No text to copy')
        return
      }

      setIsLoading(true)
      setError(null)

      try {
        // Try Electron IPC first
        if (window.api?.clipboard?.writeText) {
          const result = await window.api.clipboard.writeText(text)

          if (result.success) {
            setCopied(true)
            onSuccess?.()

            // Reset copied state after duration
            setTimeout(() => {
              setCopied(false)
            }, successDuration)
          } else {
            const errorMsg = result.error || 'Failed to copy text'
            setError(errorMsg)
            onError?.(errorMsg)
          }
        } else if (navigator.clipboard?.writeText) {
          // Fallback to browser clipboard API
          await navigator.clipboard.writeText(text)
          setCopied(true)
          onSuccess?.()

          // Reset copied state after duration
          setTimeout(() => {
            setCopied(false)
          }, successDuration)
        } else {
          // Final fallback: document.execCommand
          const textarea = document.createElement('textarea')
          textarea.value = text
          textarea.style.position = 'fixed'
          textarea.style.opacity = '0'
          document.body.appendChild(textarea)
          textarea.select()
          const success = document.execCommand('copy')
          document.body.removeChild(textarea)

          if (success) {
            setCopied(true)
            onSuccess?.()
            setTimeout(() => {
              setCopied(false)
            }, successDuration)
          } else {
            throw new Error('Failed to copy text using fallback method')
          }
        }
      } catch (err) {
        const errorMsg = err instanceof Error ? err.message : 'Failed to copy text'
        setError(errorMsg)
        onError?.(errorMsg)
      } finally {
        setIsLoading(false)
      }
    },
    [successDuration, onSuccess, onError]
  )

  const read = useCallback(async (): Promise<string | undefined> => {
    setIsLoading(true)
    setError(null)

    try {
      // Try Electron IPC first
      if (window.api?.clipboard?.readText) {
        const result = await window.api.clipboard.readText()

        if (result.success) {
          return result.text
        } else {
          const errorMsg = result.error || 'Failed to read clipboard'
          setError(errorMsg)
          onError?.(errorMsg)
          return undefined
        }
      } else if (navigator.clipboard?.readText) {
        // Fallback to browser clipboard API
        const text = await navigator.clipboard.readText()
        return text
      } else {
        throw new Error('Clipboard read not supported in this context')
      }
    } catch (err) {
      const errorMsg = err instanceof Error ? err.message : 'Failed to read clipboard'
      setError(errorMsg)
      onError?.(errorMsg)
      return undefined
    } finally {
      setIsLoading(false)
    }
  }, [onError])

  return {
    copy,
    read,
    copied,
    error,
    isLoading
  }
}
