import { createFileRoute, Outlet, Navigate } from '@tanstack/react-router'
import { Button } from '@renderer/components/ui/button'
import { ArrowLeft } from 'lucide-react'
import { Link, useRouterState } from '@tanstack/react-router'
import { Monitor, Database, Settings2, Plug, Shield, RefreshCcw, Keyboard } from 'lucide-react'
import { ScrollArea } from '@renderer/components/ui/scroll-area'
import { cn } from '@renderer/lib/utils'
import { DEFAULT_SETTINGS_ROUTE } from '@renderer/lib/constants/routes'
import { ErrorBoundary } from '@renderer/components/ui/error-boundary'

export const Route = createFileRoute('/settings')({
  component: SettingsLayout
})

const settingsTabs = [
  {
    value: 'connections',
    label: 'Connections',
    icon: Plug,
    path: '/settings/connections'
  },
  {
    value: 'import-data',
    label: 'Import Data',
    icon: Database,
    path: '/settings/import-data'
  },
  {
    value: 'permissions',
    label: 'Permissions',
    icon: Shield,
    path: '/settings/permissions'
  },
  {
    value: 'updates',
    label: 'Updates',
    icon: RefreshCcw,
    path: '/settings/updates'
  },
  {
    value: 'appearance',
    label: 'Appearance',
    icon: Monitor,
    path: '/settings/appearance'
  },
  {
    value: 'keyboard',
    label: 'Keyboard',
    icon: Keyboard,
    path: '/settings/keyboard'
  },
  {
    value: 'advanced',
    label: 'Advanced',
    icon: Settings2,
    path: '/settings/advanced'
  }
]

function SettingsLayout() {
  const { location } = useRouterState()

  // Default to appearance tab if on base settings route
  if (location.pathname === '/settings') {
    return <Navigate to={DEFAULT_SETTINGS_ROUTE} />
  }

  return (
    <div className="flex flex-col h-screen w-screen text-foreground pt-8 relative bg-background">
      <div className="titlebar text-center fixed top-0 left-0 right-0 text-muted-foreground text-xs h-8 z-20 flex items-center justify-center backdrop-blur-sm drag">
        Settings
      </div>
      <div className="flex-1 flex flex-col mt-8 overflow-hidden">
        <div className="p-4 border-b no-drag">
          <Link to="/">
            <Button variant="ghost" className="h-9 px-2">
              <ArrowLeft className="w-4 h-4 mr-2" />
              Back
            </Button>
          </Link>
        </div>
        <div className="flex-1 flex overflow-y-auto">
          <div className="w-[240px] bg-muted/50 p-4 flex flex-col gap-1 border-r">
            {settingsTabs.map((tab) => {
              const isActive = location.pathname === tab.path
              return (
                <Link
                  key={tab.value}
                  to={tab.path}
                  className={cn(
                    'flex items-center gap-2 w-full p-2 rounded-md justify-start text-sm transition-colors',
                    'hover:bg-accent',
                    isActive && 'bg-accent'
                  )}
                >
                  <tab.icon className="h-4 w-4" />
                  {tab.label}
                </Link>
              )
            })}
          </div>
          <div className="flex-1 relative w-full">
            <ScrollArea className="h-full">
              <ErrorBoundary>
                <Outlet />
              </ErrorBoundary>
            </ScrollArea>
          </div>
        </div>
      </div>
    </div>
  )
}
