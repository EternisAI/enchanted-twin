import { useState, useCallback, useEffect } from 'react'
import { Allotment } from 'allotment'
import 'allotment/dist/style.css'
import { BrowserView } from './BrowserView'
import { BrowserControls } from './BrowserControls'
import { BrowserSidebar } from './BrowserSidebar'
import { useBrowserStore, selectActiveSession } from '../stores/browserStore'
import { checkBrowserEnabled } from '@renderer/lib/utils'
import { useNavigate } from '@tanstack/react-router'
import { AlertCircle } from 'lucide-react'
import { Button } from '@renderer/components/ui/button'

export function BrowserLayout() {
  const navigate = useNavigate()
  const { createSession, setActiveSession } = useBrowserStore()
  const activeSession = useBrowserStore(selectActiveSession)

  console.log('BrowserLayout render - activeSession URL:', activeSession?.url)

  // Check if browser feature is enabled
  const isBrowserEnabled = checkBrowserEnabled()

  // Navigation state
  const [canGoBack, setCanGoBack] = useState(false)
  const [canGoForward, setCanGoForward] = useState(false)
  const [isLoading, setIsLoading] = useState(false) // If needed, but currently not set

  // Initialize browser session
  useEffect(() => {
    if (isBrowserEnabled && !activeSession) {
      const sessionId = createSession('https://www.google.com')
      setActiveSession(sessionId)
    }
  }, [isBrowserEnabled, activeSession, createSession, setActiveSession])

  // Listen for navigation state
  useEffect(() => {
    if (!activeSession) return

    const unsub = window.api.browser.onNavigationState((sid, state) => {
      if (sid === activeSession.id) {
        setCanGoBack(state.canGoBack)
        setCanGoForward(state.canGoForward)
      }
    })

    return unsub
  }, [activeSession])

  const handleNavigate = useCallback(
    async (url: string) => {
      if (activeSession) {
        await window.api.browser.loadUrl(activeSession.id, url)
      }
    },
    [activeSession]
  )

  const handleGoBack = useCallback(async () => {
    if (activeSession && canGoBack) {
      await window.api.browser.goBack(activeSession.id)
    }
  }, [activeSession, canGoBack])

  const handleGoForward = useCallback(async () => {
    if (activeSession && canGoForward) {
      await window.api.browser.goForward(activeSession.id)
    }
  }, [activeSession, canGoForward])

  const handleRefresh = useCallback(async () => {
    if (activeSession) {
      await window.api.browser.reload(activeSession.id)
    }
  }, [activeSession])

  const handleStop = useCallback(async () => {
    if (activeSession) {
      await window.api.browser.stop(activeSession.id)
    }
  }, [activeSession])

  const handleContentUpdate = useCallback(
    (content: any) => {
      if (activeSession) {
        useBrowserStore.getState().updateSession(activeSession.id, { content })
      }
    },
    [activeSession]
  )

  // Show feature disabled message
  if (!isBrowserEnabled) {
    return (
      <div className="flex flex-col items-center justify-center h-full gap-4 p-8 text-center">
        <AlertCircle className="w-12 h-12 text-muted-foreground" />
        <h2 className="text-lg font-semibold">Browser Feature Disabled</h2>
        <p className="text-sm text-muted-foreground max-w-md">
          The browser feature is currently disabled. To enable it, set the VITE_ENABLE_BROWSER
          environment variable to "true" and restart the application.
        </p>
        <Button onClick={() => navigate({ to: '/' })}>Go Back</Button>
      </div>
    )
  }

  if (!activeSession) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary" />
      </div>
    )
  }

  return (
    <div className="flex flex-col h-full w-full pt-5">
      <Allotment>
        {/* Browser pane */}
        <Allotment.Pane minSize={400}>
          <div className="flex flex-col h-full w-full">
            <BrowserControls
              url={activeSession.url}
              canGoBack={canGoBack}
              canGoForward={canGoForward}
              isLoading={isLoading}
              isSecure={activeSession.url.startsWith('https://')}
              onNavigate={handleNavigate}
              onGoBack={handleGoBack}
              onGoForward={handleGoForward}
              onRefresh={handleRefresh}
              onStop={handleStop}
            />
            <BrowserView
              sessionId={activeSession.id}
              url={activeSession.url}
              className="flex-1"
              onNavigate={(url) => {
                useBrowserStore.getState().updateSession(activeSession.id, { url })
              }}
              onContentUpdate={handleContentUpdate}
            />
          </div>
        </Allotment.Pane>

        {/* Chat sidebar */}
        <Allotment.Pane minSize={300} preferredSize={400}>
          <BrowserSidebar sessionId={activeSession.id} />
        </Allotment.Pane>
      </Allotment>
    </div>
  )
}
