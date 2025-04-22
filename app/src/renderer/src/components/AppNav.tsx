import { cn } from '@renderer/lib/utils'
import { Link, useRouterState } from '@tanstack/react-router'
import { motion, AnimatePresence } from 'framer-motion'
import { MessageCircle, Settings } from 'lucide-react'

const LINKS = [
  // {
  //   label: 'Feed',
  //   href: '/feed',
  //   icon: HeartPulse
  // },
  {
    label: 'Chats',
    href: '/chat',
    icon: MessageCircle
  },
  {
    label: 'Settings',
    href: '/settings',
    icon: Settings
  }
]

export function AppNav() {
  const { location } = useRouterState()

  return (
    <div>
      <AnimatePresence>
        <motion.aside
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.3 }}
          className="p-1 flex flex-col gap-6 w-18"
        >
          <nav className="flex flex-col gap-2">
            {LINKS.map(({ label, href, icon: Icon }) => {
              const isActive = location.pathname.startsWith(href)
              return (
                <Link
                  key={href}
                  to={href}
                  disabled={isActive}
                  className={cn(
                    'group text-xs font-medium flex flex-col items-center gap-1 px-2 py-1 aspect-square justify-center rounded-md',
                    isActive && 'font-semibold'
                  )}
                >
                  <div
                    className={cn(
                      'w-full h-10 flex items-center justify-center rounded-xl group-hover:bg-accent transition-colors',
                      isActive && 'text-primary bg-accent'
                    )}
                  >
                    <Icon className="w-4 h-4" fill={isActive ? 'currentColor' : 'none'} />
                  </div>
                  {label}
                </Link>
              )
            })}
          </nav>
        </motion.aside>
      </AnimatePresence>
    </div>
  )
}
