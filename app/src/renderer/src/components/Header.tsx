'use client'

import { cn } from '@renderer/lib/utils'
import { Link, useRouterState } from '@tanstack/react-router'
import { useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { Home, MenuIcon, MessageCircle } from 'lucide-react'

const LINKS = [
  {
    label: 'Home',
    to: '/',
    icon: Home
  },
  {
    label: 'Chat',
    to: '/chat',
    icon: MessageCircle
  }
]

export default function SidebarDock() {
  const { location } = useRouterState()
  const [hovering, setHovering] = useState(false)

  return (
    <div
      className="fixed top-1/2 -translate-y-1/2 left-0 z-50"
      onMouseEnter={() => setHovering(true)}
      onMouseLeave={() => setHovering(false)}
    >
      {!hovering && (
        <AnimatePresence>
          <motion.div
            initial={{ x: -50, opacity: 0 }}
            animate={{ x: 0, opacity: 1 }}
            exit={{ x: -50, opacity: 0 }}
            transition={{ type: 'spring', stiffness: 300, damping: 30 }}
            className="bg-purple-600 text-white rounded-r-full text-lg px-3 py-3 cursor-pointer shadow-lg"
          >
            <MenuIcon className="w-5 h-5" />
          </motion.div>
        </AnimatePresence>
      )}

      {/* Sidebar */}
      {hovering && (
        <AnimatePresence>
          <motion.aside
            initial={{ x: -150, opacity: 0 }}
            animate={{ x: 0, opacity: 1 }}
            exit={{ x: -150, opacity: 0 }}
            transition={{ type: 'spring', stiffness: 200, damping: 24 }}
            className="bg-white border-r shadow-lg p-4 flex flex-col gap-6 rounded-r-xl w-48"
          >
            <div className="text-xl font-bold text-purple-700 px-2">Enchanted Twin</div>
            <nav className="flex flex-col gap-4 mt-2">
              {LINKS.map(({ label, to, icon: Icon }) => {
                const isActive = location.href === to
                return (
                  <Link
                    key={to}
                    to={to}
                    disabled={isActive}
                    className={cn(
                      'flex items-center gap-2 px-3 py-2 rounded-md hover:bg-purple-100 transition-colors',
                      isActive ? 'bg-purple-200 text-purple-800 font-semibold' : 'text-gray-700'
                    )}
                  >
                    <Icon className="w-5 h-5" />
                    {label}
                  </Link>
                )
              })}
            </nav>
          </motion.aside>
        </AnimatePresence>
      )}
    </div>
  )
}
