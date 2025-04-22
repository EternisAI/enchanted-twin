import { AppNav } from '@renderer/components/AppNav'
import { createRootRoute, Outlet } from '@tanstack/react-router'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { SetupBanner } from '@renderer/components/SetupBanner'

function DevBadge() {
  return <span className="text-xs font-bold text-muted-foreground">⚠️ DEVELOPMENT VERSION</span>
}

function RootComponent() {
  const { isCompleted } = useOnboardingStore()

  if (!isCompleted) {
    return (
      <div className="flex flex-col h-screen w-screen text-foreground">
        <div className="titlebar text-center fixed top-0 left-0 right-0 text-muted-foreground text-xs h-6 z-50 flex items-center justify-center backdrop-blur-sm">
          {process.env.NODE_ENV === 'development' && <DevBadge />}
        </div>
        <div className="flex-1 overflow-auto pt-6">
          <Outlet />
        </div>
      </div>
    )
  }

  return (
    <div className="flex flex-col h-screen w-screen text-foreground">
      <div className="titlebar text-center fixed top-0 left-0 right-0 text-muted-foreground text-xs h-6 z-50 flex items-center justify-center backdrop-blur-sm">
        {process.env.NODE_ENV === 'development' && <DevBadge />}
      </div>
      <div className="flex-1 flex flex-col overflow-hidden pt-6">
        {process.env.NODE_ENV === 'development' && <SetupBanner />}
        <div className="flex-1 flex overflow-hidden">
          <AppNav />
          <div className="flex-1 overflow-auto">
            <Outlet />
          </div>
        </div>
      </div>
    </div>
  )
}

// This is the root layout, components get render in the <Outlet />
export const Route = createRootRoute({
  component: RootComponent
})
