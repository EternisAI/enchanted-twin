import { createFileRoute, Outlet, Navigate } from '@tanstack/react-router'
import { Button } from '@renderer/components/ui/button'
import { ArrowLeft, Info } from 'lucide-react'
import { Link, useRouterState, useRouter } from '@tanstack/react-router'
import { Database, Settings2, Shield } from 'lucide-react'
import { ScrollArea } from '@renderer/components/ui/scroll-area'
import { cn } from '@renderer/lib/utils'
import { DEFAULT_SETTINGS_ROUTE } from '@renderer/lib/constants/routes'
import { ErrorBoundary } from '@renderer/components/ui/error-boundary'
import { motion } from 'framer-motion'

export const Route = createFileRoute('/settings')({
  component: SettingsLayout
})

const settingsTabs = [
  {
    value: 'data-sources',
    label: 'Data Sources',
    icon: Database,
    path: '/settings/data-sources'
  },
  {
    value: 'permissions',
    label: 'Permissions',
    icon: Shield,
    path: '/settings/permissions'
  },
  {
    value: 'customize',
    label: 'Customize',
    icon: Settings2,
    path: '/settings/customize'
  },
  {
    value: 'about',
    label: 'About',
    icon: Info,
    path: '/settings/about'
  },
  ...(process.env.NODE_ENV === 'development'
    ? [
        {
          value: 'advanced',
          label: 'Advanced',
          icon: Settings2,
          path: '/settings/advanced'
        }
      ]
    : [])
]

function SettingsLayout() {
  const { location } = useRouterState()
  const router = useRouter()

  // Default to appearance tab if on base settings route
  const isBaseSettingsRoute = location.pathname === '/settings'

  if (isBaseSettingsRoute) {
    return <Navigate to={DEFAULT_SETTINGS_ROUTE} replace />
  }

  const handleBackClick = () => {
    router.history.back()
  }

  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      transition={{ duration: 0.5, ease: 'easeInOut' }}
      className="flex flex-col h-screen w-screen text-foreground pt-8 relative bg-background"
    >
      <div className="titlebar text-center fixed top-0 left-0 right-0 text-muted-foreground text-xs h-8 z-20 flex items-center justify-center backdrop-blur-sm drag">
        Settings
      </div>
      <div className="flex-1 flex flex-col overflow-hidden">
        <div className="p-4 border-b no-drag border-border">
          <Button variant="ghost" className="h-9 px-2 absolute left-4" onClick={handleBackClick}>
            <ArrowLeft className="w-4 h-4 mr-2" />
            Back
          </Button>
          <div className="no-drag flex items-center justify-center">
            <div className="flex items-center gap-6">
              {settingsTabs.map((tab) => {
                const isActive = location.pathname === tab.path
                return (
                  <Link
                    key={tab.value}
                    to={tab.path}
                    replace={true}
                    className={cn(
                      'flex flex-col items-center gap-1 p-3 rounded-lg transition-colors min-w-[80px]',
                      'hover:bg-accent',
                      isActive && 'bg-muted font-bold'
                    )}
                  >
                    <tab.icon className="h-5 w-5" />
                    <span className="text-xs">{tab.label}</span>
                  </Link>
                )
              })}
            </div>
          </div>
        </div>
        <ScrollArea className="h-full w-full">
          <ErrorBoundary>
            <Outlet />
          </ErrorBoundary>
        </ScrollArea>
      </div>
    </motion.div>
  )
}
