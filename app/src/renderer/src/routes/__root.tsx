import { useEffect } from 'react'
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

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === ',') {
        e.preventDefault()
        navigate({ to: '/settings' })
      }
      if (e.key === 'Escape' && sidebarOpen && location.pathname !== '/settings') {
        e.preventDefault()
      }
      if ((e.metaKey || e.ctrlKey) && e.key === 'n') {
        e.preventDefault()
        navigate({ to: '/', search: { focusInput: 'true' } })
      }
      if (
        (e.metaKey || e.ctrlKey) &&
        e.key === 's' &&
        isCompleted &&
        !omnibar.isOpen &&
        location.pathname !== '/settings'
      ) {
        e.preventDefault()
        setSidebarOpen(!sidebarOpen)
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    window.api.onOpenSettings(() => navigate({ to: '/settings' }))
    return () => {
      window.removeEventListener('keydown', handleKeyDown)
    }
  }, [sidebarOpen, navigate, isCompleted, omnibar.isOpen, location.pathname, setSidebarOpen])

  if (location.pathname === '/settings') {
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
              {!sidebarOpen && isCompleted && location.pathname !== '/settings' && (
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
                          <kbd className="rounded bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground font-sans">
                            ⌘ S
                          </kbd>
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
                  <Sidebar chats={chats} setSidebarOpen={setSidebarOpen} />
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
