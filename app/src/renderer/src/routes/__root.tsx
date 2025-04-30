import { AppNav } from '@renderer/components/AppNav'
import { createRootRoute, Outlet } from '@tanstack/react-router'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { ContinueSetupButton } from '@renderer/components/ContinueSetupButton'
import AdminKeyboardShortcuts from '@renderer/components/AdminKeyboardShortcuts'
import { AnimatePresence } from 'framer-motion'

function DevBadge() {
  return <span className="text-xs font-bold text-muted-foreground">⚠️ DEVELOPMENT VERSION</span>
}

function RootComponent() {
  const { isCompleted } = useOnboardingStore()

  return (
    <div className="flex flex-col h-screen w-screen text-foreground pt-8">
      <AdminKeyboardShortcuts />
      <div className="titlebar text-center fixed top-0 left-0 right-0 text-muted-foreground text-xs h-8 z-20 flex items-center justify-center backdrop-blur-sm">
        {process.env.NODE_ENV === 'development' ? <DevBadge /> : ' '}
      </div>
      {isCompleted && (
        <div className="fixed top-2 right-2 z-50 h-8 no-drag">
          <ContinueSetupButton />
        </div>
      )}
      <div className="flex-1 flex flex-col overflow-hidden">
        <div className="flex-1 flex overflow-hidden">
          <AnimatePresence>
            {isCompleted && <AppNav />}
            <Outlet />
          </AnimatePresence>
        </div>
      </div>
    </div>
  )
}

// This is the root layout, components get render in the <Outlet />
export const Route = createRootRoute({
  component: RootComponent
})
