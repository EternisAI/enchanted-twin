import { createRootRoute, Outlet } from '@tanstack/react-router'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import AdminKeyboardShortcuts from '@renderer/components/AdminKeyboardShortcuts'
import { Omnibar } from '@renderer/components/Omnibar'
import { GlobalIndexingStatus } from '@renderer/components/GlobalIndexingStatus'
import { useOsNotifications } from '@renderer/hooks/useNotifications'
import UpdateNotification from '@renderer/components/UpdateNotification'
import { useSettingsStore } from '@renderer/lib/stores/settings'
import { useEffect } from 'react'
import { SettingsDialog } from '@renderer/components/settings/SettingsDialog'

function DevBadge() {
  return <span className="text-xs font-bold text-muted-foreground">⚠️ DEVELOPMENT VERSION</span>
}

function RootComponent() {
  const { isCompleted } = useOnboardingStore()
  useOsNotifications()
  const { open } = useSettingsStore()

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === ',') {
        e.preventDefault()
        open()
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    window.api.onOpenSettings(open)

    return () => {
      window.removeEventListener('keydown', handleKeyDown)
    }
  }, [open])

  return (
    <div className="flex flex-col h-screen w-screen text-foreground pt-8">
      <AdminKeyboardShortcuts />
      <div className="titlebar text-center fixed top-0 left-0 right-0 text-muted-foreground text-xs h-8 z-20 flex items-center justify-center backdrop-blur-sm">
        {process.env.NODE_ENV === 'development' ? <DevBadge /> : ' '}
      </div>
      <Omnibar />
      {isCompleted && (
        <div className="fixed top-0 right-0 z-50 h-8 no-drag">
          <GlobalIndexingStatus />
        </div>
      )}
      <div className="flex-1 flex flex-col overflow-hidden">
        <div className="flex-1 flex overflow-hidden">
          <Outlet />
        </div>
      </div>
      <UpdateNotification />
      <SettingsDialog />
    </div>
  )
}

// This is the root layout, components get render in the <Outlet />
export const Route = createRootRoute({
  component: RootComponent
})
