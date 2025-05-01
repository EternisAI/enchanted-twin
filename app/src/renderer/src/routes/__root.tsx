import { AppNav } from '@renderer/components/AppNav'
import { createRootRoute, Outlet } from '@tanstack/react-router'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import AdminKeyboardShortcuts from '@renderer/components/AdminKeyboardShortcuts'
import { Omnibar } from '@renderer/components/Omnibar'
import { GlobalIndexingStatus } from '@renderer/components/GlobalIndexingStatus'
import { useOsNotifications } from '@renderer/hooks/useNotifications'
import UpdateNotification from '@renderer/components/UpdateNotification'

function DevBadge() {
  return <span className="text-xs font-bold text-muted-foreground">⚠️ DEVELOPMENT VERSION</span>
}

function RootComponent() {
  const { isCompleted } = useOnboardingStore()
  useOsNotifications()

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
          {isCompleted && <AppNav />}
          <Outlet />
        </div>
      </div>
      <UpdateNotification />
    </div>
  )
}

// This is the root layout, components get render in the <Outlet />
export const Route = createRootRoute({
  component: RootComponent
})
