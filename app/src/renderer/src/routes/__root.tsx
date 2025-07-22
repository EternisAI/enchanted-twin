import { useEffect, useState } from 'react'
import { createRootRoute, Outlet, useNavigate, useRouterState } from '@tanstack/react-router'
import { useQuery } from '@apollo/client'

import AdminKeyboardShortcuts from '@renderer/components/AdminKeyboardShortcuts'
import { Omnibar } from '@renderer/components/Omnibar'
import { GlobalIndexingStatus } from '@renderer/components/GlobalIndexingStatus'
import { NotificationsProvider } from '@renderer/hooks/NotificationsContextProvider'
import { LayoutGroup, motion, AnimatePresence } from 'framer-motion'
import { Sidebar } from '@renderer/components/chat/Sidebar'
import { GetChatsDocument, Chat } from '@renderer/graphql/generated/graphql'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { useOmnibarStore } from '@renderer/lib/stores/omnibar'
import { useSidebarStore } from '@renderer/lib/stores/sidebar'
import { DEFAULT_SETTINGS_ROUTE } from '@renderer/lib/constants/routes'
import { PrivacyButton } from '@renderer/components/chat/privacy/PrivacyButton'

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

  if (location.pathname.startsWith('/settings')) {
    return <Outlet />
  }

  return (
    <NotificationsProvider>
      <LayoutGroup>
        <motion.div
          className="flex flex-col h-screen w-screen text-foreground relative bg-background"
          layout="position"
        >
          <AdminKeyboardShortcuts />
          <motion.div className="titlebar text-center fixed top-0 left-0 right-0 text-muted-foreground text-xs h-8 z-20 flex items-center justify-center">
            {/* {process.env.NODE_ENV === 'development' ? <DevBadge /> : ' '} */}
          </motion.div>
          <div className="fixed top-0 right-0 z-50 h-8 no-drag flex items-center gap-2">
            {location.pathname !== '/onboarding' && <PrivacyButton />}
            <GlobalIndexingStatus />
          </div>

          <div className="flex flex-1 overflow-hidden">
            <AnimatePresence>
              {isCompleted && !location.pathname.startsWith('/settings') && (
                <motion.div
                  key="sidebar"
                  initial={{ width: sidebarOpen ? 256 : 64 }}
                  animate={{ width: sidebarOpen ? 256 : 64 }}
                  exit={{ width: 0, opacity: 0 }}
                  transition={{ type: 'spring', stiffness: 350, damping: 55 }}
                  className="h-full overflow-y-auto"
                >
                  <Sidebar
                    chats={chats}
                    setSidebarOpen={setSidebarOpen}
                    shortcuts={shortcuts}
                    collapsed={!sidebarOpen}
                  />
                </motion.div>
              )}
            </AnimatePresence>
            <motion.div
              className="flex-1 flex flex-col overflow-hidden"
              layout
              transition={{ type: 'spring', stiffness: 350, damping: 55 }}
            >
              <motion.div className="flex-1 flex overflow-hidden relative ">
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
