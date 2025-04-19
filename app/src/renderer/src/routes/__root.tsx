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
        <div className="titlebar text-center fixed top-0 left-0 right-0 text-muted-foreground text-xs h-6 z-50 flex items-center justify-center">
          {process.env.NODE_ENV === 'development' && <DevBadge />}
        </div>
        <Outlet />
      </div>
    )
  }

  return (
    <div className="flex flex-col text-foreground">
      <div className="titlebar text-center fixed top-0 left-0 right-0 text-muted-foreground text-xs h-6 z-50 flex items-center justify-center">
        {process.env.NODE_ENV === 'development' && <DevBadge />}
      </div>
      <div className="flex flex-col flex-1 pt-6 gap-2">
        {process.env.NODE_ENV === 'development' && <SetupBanner />}
        <div className="flex flex-1">
          <AppNav />
          <div className="flex-1 overflow-auto rounded-lg h-full">
            <Outlet />
            {/* <TanStackRouterDevtools /> */}
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
