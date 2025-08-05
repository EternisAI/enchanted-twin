import { useEffect, useRef, useState } from 'react'
import { useBrowserStore } from '../stores/browserStore'
import { BrowserSecuritySettings } from '../types/browser.types'
import { AlertCircle } from 'lucide-react'
import { cn } from '@renderer/lib/utils'

interface BrowserViewProps {
  sessionId: string
  url: string
  className?: string
  securitySettings?: BrowserSecuritySettings
  onNavigate?: (url: string) => void
  onContentUpdate?: (content: { text: string; html: string; metadata: any }) => void
}

export function BrowserView({
  sessionId,
  url,
  className,
  securitySettings = {
    enableJavaScript: true,
    enablePlugins: false,
    enableWebSecurity: true,
    requireUserApproval: true
  },
  onNavigate,
  onContentUpdate // Note: This may not be used, but kept for compatibility
}: BrowserViewProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const { updateSession } = useBrowserStore()

  useEffect(() => {
    // Create session
    window.api.browser.createSession(sessionId, url, `browser-${sessionId}`, securitySettings)

    // Set up event listeners
    const unsubStart = window.api.browser.onDidStartLoading((sid) => {
      if (sid === sessionId) {
        setIsLoading(true)
        setError(null)
      }
    })

    const unsubStop = window.api.browser.onDidStopLoading((sid) => {
      if (sid === sessionId) {
        setIsLoading(false)
      }
    })

    const unsubFail = window.api.browser.onDidFailLoad((sid, details) => {
      if (sid === sessionId) {
        setError(`Failed to load: ${details.errorDescription}`)
        setIsLoading(false)
      }
    })

    const unsubNavigate = window.api.browser.onDidNavigate((sid, newUrl) => {
      if (sid === sessionId) {
        console.log('BrowserView received navigation event:', { sessionId, newUrl })
        updateSession(sessionId, { url: newUrl })
        onNavigate?.(newUrl)
      }
    })

    const unsubTitle = window.api.browser.onPageTitleUpdated((sid, title) => {
      if (sid === sessionId) {
        updateSession(sessionId, { title })
      }
    })

    const unsubSession = window.api.browser.onSessionUpdated((sid, content) => {
      if (sid === sessionId) {
        updateSession(sessionId, {
          content: { text: content.text, html: content.html },
          title: content.metadata.title
        })
      }
    })

    const unsubNavOccurred = window.api.browser.onNavigationOccurred((sid, newUrl) => {
      if (sid === sessionId) {
        console.log('BrowserView received navigation-occurred event:', { sessionId, newUrl })
        updateSession(sessionId, { url: newUrl })
        onNavigate?.(newUrl)
      }
    })

    // Set up resize observer for bounds
    const resizeObserver = new ResizeObserver(() => {
      if (containerRef.current) {
        const bounds = containerRef.current.getBoundingClientRect()
        const rect = {
          x: Math.floor(bounds.left),
          y: Math.floor(bounds.top),
          width: Math.ceil(bounds.width),
          height: Math.ceil(bounds.height)
        }
        window.api.browser.setBounds(sessionId, rect)
      }
    })

    if (containerRef.current) {
      resizeObserver.observe(containerRef.current)
    }

    // Initial bounds
    if (containerRef.current) {
      const bounds = containerRef.current.getBoundingClientRect()
      const rect = {
        x: Math.floor(bounds.left),
        y: Math.floor(bounds.top),
        width: Math.ceil(bounds.width),
        height: Math.ceil(bounds.height)
      }
      window.api.browser.setBounds(sessionId, rect)
    }

    // Cleanup
    return () => {
      unsubStart()
      unsubStop()
      unsubFail()
      unsubNavigate()
      unsubTitle()
      unsubSession()
      unsubNavOccurred()
      window.api.browser.destroySession(sessionId)
    }
  }, [sessionId, url, securitySettings, updateSession, onNavigate])

  // Handle url changes
  useEffect(() => {
    window.api.browser.loadUrl(sessionId, url)
  }, [url, sessionId])

  return (
    <div className={cn('relative flex flex-col h-full', className)}>
      {error && (
        <div className="absolute top-0 left-0 right-0 bg-red-50 dark:bg-red-950 p-2 flex items-center gap-2 text-sm text-red-600 dark:text-red-400 z-10">
          <AlertCircle className="w-4 h-4" />
          {error}
        </div>
      )}

      {isLoading && (
        <div className="absolute top-0 left-0 right-0 h-1 bg-blue-200 dark:bg-blue-800 z-10">
          <div className="h-full bg-blue-500 animate-pulse" />
        </div>
      )}

      <div
        ref={containerRef}
        className="flex-1 w-full h-full"
        style={{ backgroundColor: 'white' }}
      />
    </div>
  )
}
