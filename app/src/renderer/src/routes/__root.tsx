import { createRootRoute, Outlet, useNavigate } from '@tanstack/react-router'
import AdminKeyboardShortcuts from '@renderer/components/AdminKeyboardShortcuts'
import { Omnibar } from '@renderer/components/Omnibar'
import { GlobalIndexingStatus } from '@renderer/components/GlobalIndexingStatus'
import { useOsNotifications } from '@renderer/hooks/useNotifications'
import UpdateNotification from '@renderer/components/UpdateNotification'
import { useSettingsStore } from '@renderer/lib/stores/settings'
import { useEffect, useState } from 'react'
import { SettingsDialog } from '@renderer/components/settings/SettingsDialog'
import { LayoutGroup, motion, AnimatePresence } from 'framer-motion'
import { Sidebar } from '@renderer/components/chat/Sidebar'
import { Button } from '@renderer/components/ui/button'
import { PanelLeftOpen } from 'lucide-react'
import { GetChatsDocument, Chat } from '@renderer/graphql/generated/graphql'
import { useQuery } from '@apollo/client'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'

function DevBadge() {
  return <span className="text-xs font-bold text-muted-foreground">⚠️ DEVELOPMENT VERSION</span>
}

function RootComponent() {
  useOsNotifications()
  const { open } = useSettingsStore()
  const [sidebarOpen, setSidebarOpen] = useState(true)
  const navigate = useNavigate()

  const { isCompleted } = useOnboardingStore()
  const { data: chatsData } = useQuery(GetChatsDocument, {
    variables: { first: 20, offset: 0 }
  })
  const chats: Chat[] = chatsData?.getChats || []

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === ',') {
        e.preventDefault()
        open()
      }
      if (e.key === 'Escape' && sidebarOpen) {
        e.preventDefault()
      }
      if ((e.metaKey || e.ctrlKey) && e.key === 'n') {
        e.preventDefault()
        navigate({ to: '/', search: { focusInput: 'true' } })
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    window.api.onOpenSettings(open)
    return () => {
      window.removeEventListener('keydown', handleKeyDown)
    }
  }, [open, sidebarOpen, navigate])

  return (
    <LayoutGroup>
      <motion.div
        className="flex flex-col h-screen w-screen text-foreground pt-8 relative"
        layout="position"
      >
        <AdminKeyboardShortcuts />
        <motion.div className="titlebar text-center fixed top-0 left-0 right-0 text-muted-foreground text-xs h-8 z-20 flex items-center justify-center backdrop-blur-sm">
          {process.env.NODE_ENV === 'development' ? <DevBadge /> : ' '}
        </motion.div>
        <div className="fixed top-0 right-0 z-50 h-8 no-drag">
          <GlobalIndexingStatus />
        </div>

        <div className="flex flex-1 overflow-hidden mt-0">
          <AnimatePresence>
            {!sidebarOpen && isCompleted && (
              <motion.div
                className="absolute top-11 left-3 z-[60]"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                transition={{ duration: 0.5 }}
              >
                <Button
                  onClick={() => setSidebarOpen(true)}
                  variant="ghost"
                  size="icon"
                  className="text-muted-foreground hover:text-foreground"
                >
                  <PanelLeftOpen className="w-5 h-5" />
                </Button>
              </motion.div>
            )}
            {sidebarOpen && isCompleted && (
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
            layout="position"
            transition={{ type: 'spring', stiffness: 300, damping: 30, duration: 0.2 }}
          >
            <motion.div layout="position" className="flex-1 flex overflow-hidden relative">
              <Outlet />
              <Omnibar />
            </motion.div>
          </motion.div>
        </div>
        <UpdateNotification />
        <SettingsDialog />
      </motion.div>
    </LayoutGroup>
  )
}

export const Route = createRootRoute({
  component: RootComponent
})
