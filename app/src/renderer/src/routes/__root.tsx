import { useEffect, useState } from 'react'
import { createRootRoute, Outlet, useNavigate, useRouterState } from '@tanstack/react-router'
import { PanelLeftOpen } from 'lucide-react'
import { useQuery } from '@apollo/client'

import AdminKeyboardShortcuts from '@renderer/components/AdminKeyboardShortcuts'
import { Omnibar } from '@renderer/components/Omnibar'
import { GlobalIndexingStatus } from '@renderer/components/GlobalIndexingStatus'
import { NotificationsProvider } from '@renderer/hooks/NotificationsContextProvider'
import { LayoutGroup, motion, AnimatePresence } from 'framer-motion'
import { Sidebar } from '@renderer/components/chat/Sidebar'
import { Button } from '@renderer/components/ui/button'
import { GetChatsDocument, Chat } from '@renderer/graphql/generated/graphql'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { useOmnibarStore } from '@renderer/lib/stores/omnibar'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger
} from '@renderer/components/ui/tooltip'
import { useSidebarStore } from '@renderer/lib/stores/sidebar'
import { DEFAULT_SETTINGS_ROUTE } from '@renderer/lib/constants/routes'
import { formatShortcutForDisplay } from '@renderer/lib/utils/shortcuts'

function RootComponent() {
  const omnibar = useOmnibarStore()
  const { isOpen: sidebarOpen, setOpen: setSidebarOpen } = useSidebarStore()
  const navigate = useNavigate()
  const { location } = useRouterState()

  const { isCompleted } = useOnboardingStore()
  const { data: chatsData } = useQuery(GetChatsDocument, {
    variables: { first: 20, offset: 0 }
  })
  const chats: Chat[] = chatsData?.getChats || []

  // Get keyboard shortcuts from store
  const [shortcuts, setShortcuts] = useState<
    Record<string, { keys: string; default: string; global?: boolean }>
  >({})

  useEffect(() => {
    // Load shortcuts on mount
    window.api.keyboardShortcuts.get().then(setShortcuts)
  }, [])

  useEffect(() => {
    // Parse shortcut keys to check for matches
    const parseShortcut = (keys: string) => {
      if (!keys) return null
      const parts = keys.split('+')
      const hasCmd = parts.includes('CommandOrControl')
      const hasAlt = parts.includes('Alt')
      const hasShift = parts.includes('Shift')
      const key = parts[parts.length - 1].toLowerCase()
      return { hasCmd, hasAlt, hasShift, key }
    }

    const handleKeyDown = (e: KeyboardEvent) => {
      // Don't process if shortcuts haven't loaded yet
      if (Object.keys(shortcuts).length === 0) return

      // Don't process if user is typing in an input/textarea
      const target = e.target as HTMLElement
      if (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable) {
        return
      }

      const isCmd = e.metaKey || e.ctrlKey
      const isAlt = e.altKey
      const isShift = e.shiftKey
      const key = e.key.toLowerCase()

      // Check each shortcut
      Object.entries(shortcuts).forEach(([action, shortcut]) => {
        // Skip global shortcuts - they're handled in the main process
        if (shortcut.global) return

        const parsed = parseShortcut(shortcut.keys)
        if (!parsed) return

        if (
          parsed.hasCmd === isCmd &&
          parsed.hasAlt === isAlt &&
          parsed.hasShift === isShift &&
          parsed.key === key
        ) {
          e.preventDefault()
          e.stopPropagation()

          console.log(`Shortcut triggered: ${action}`)

          switch (action) {
            case 'openSettings':
              navigate({ to: DEFAULT_SETTINGS_ROUTE })
              break
            case 'newChat':
              navigate({ to: '/', search: { focusInput: 'true' } })
              break
            case 'toggleSidebar':
              // Only prevent toggling on settings pages where sidebar doesn't exist
              if (!location.pathname.startsWith('/settings')) {
                console.log('Toggling sidebar from keyboard shortcut')
                setSidebarOpen(!sidebarOpen)
              }
              break
          }
        }
      })
    }

    window.addEventListener('keydown', handleKeyDown)

    // IPC event listeners for menu items and global shortcuts
    const removeOpenSettingsListener = window.api.onOpenSettings(() =>
      navigate({ to: DEFAULT_SETTINGS_ROUTE })
    )
    const removeNewChatListener = window.api.onNewChat(() =>
      navigate({ to: '/', search: { focusInput: 'true' } })
    )
    const removeToggleSidebarListener = window.api.onToggleSidebar(() => {
      if (isCompleted && !omnibar.isOpen && !location.pathname.startsWith('/settings')) {
        setSidebarOpen(!sidebarOpen)
      }
    })
    const removeNavigateToListener = window.api.onNavigateTo?.((url: string) => {
      navigate({ to: url })
    })

    // Signal to main process that renderer is ready for navigation (after listener is set up)
    window.api?.rendererReady?.()
    return () => {
      window.removeEventListener('keydown', handleKeyDown)
      removeNavigateToListener?.()
      if (removeOpenSettingsListener) {
        removeOpenSettingsListener()
      }
      if (removeNewChatListener) {
        removeNewChatListener()
      }
      if (removeToggleSidebarListener) {
        removeToggleSidebarListener()
      }
    }
  }, [
    shortcuts,
    navigate,
    isCompleted,
    omnibar.isOpen,
    location.pathname,
    setSidebarOpen,
    sidebarOpen
  ])

  if (location.pathname.startsWith('/settings') || location.pathname === '/omnibar-overlay') {
    return <Outlet />
  }

  return (
    <NotificationsProvider>
      <LayoutGroup>
        <motion.div
          className="flex flex-col h-screen w-screen text-foreground pt-8 relative"
          layout="position"
        >
          <AdminKeyboardShortcuts />
          <motion.div className="titlebar text-center fixed top-0 left-0 right-0 text-muted-foreground text-xs h-8 z-20 flex items-center justify-center backdrop-blur-sm">
            {/* {process.env.NODE_ENV === 'development' ? <DevBadge /> : ' '} */}
          </motion.div>
          <div className="fixed top-0 right-0 z-50 h-8 no-drag">
            <GlobalIndexingStatus />
          </div>

          <div className="flex flex-1 overflow-hidden mt-0">
            <AnimatePresence>
              {!sidebarOpen && isCompleted && !location.pathname.startsWith('/settings') && (
                <motion.div
                  className="absolute top-11 left-3 z-[60]"
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  transition={{ duration: 0.5 }}
                >
                  <TooltipProvider>
                    <Tooltip delayDuration={500}>
                      <TooltipTrigger asChild>
                        <Button
                          onClick={() => setSidebarOpen(true)}
                          variant="ghost"
                          size="icon"
                          className="text-muted-foreground hover:text-foreground"
                        >
                          <PanelLeftOpen className="w-5 h-5" />
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent side="bottom" align="center">
                        <div className="flex items-center gap-2">
                          <span>Open sidebar</span>
                          {shortcuts.toggleSidebar?.keys && (
                            <kbd className="rounded bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground font-sans">
                              {formatShortcutForDisplay(shortcuts.toggleSidebar.keys)}
                            </kbd>
                          )}
                        </div>
                      </TooltipContent>
                    </Tooltip>
                  </TooltipProvider>
                </motion.div>
              )}
              {sidebarOpen && isCompleted && location.pathname !== '/settings' && (
                <motion.div
                  key="sidebar"
                  initial={{ width: 0, opacity: 0, marginRight: 0 }}
                  animate={{ width: 256, opacity: 1, marginRight: 16 }}
                  exit={{ width: 0, opacity: 0, marginRight: 0 }}
                  transition={{ type: 'spring', stiffness: 300, damping: 30, duration: 0.2 }}
                  className="h-full overflow-y-auto"
                >
                  <Sidebar chats={chats} setSidebarOpen={setSidebarOpen} shortcuts={shortcuts} />
                </motion.div>
              )}
            </AnimatePresence>
            <motion.div
              className="flex-1 flex flex-col overflow-hidden"
              layout
              transition={{ type: 'spring', stiffness: 300, damping: 30, duration: 0.2 }}
            >
              <motion.div className="flex-1 flex overflow-hidden relative justify-center">
                <Outlet />
              </motion.div>
            </motion.div>
          </div>
          <Omnibar />
        </motion.div>
      </LayoutGroup>
    </NotificationsProvider>
  )
}

export const Route = createRootRoute({
  component: RootComponent
})
