import { createFileRoute } from '@tanstack/react-router'
import Versions from '@renderer/components/Versions'
import { SettingsContent } from '@renderer/components/settings/SettingsContent'
import enchantedIcon from '@resources/icon.png'
import { motion } from 'framer-motion'
import { Button } from '@renderer/components/ui/button'
import { useAuth } from '@renderer/contexts/AuthContext'
import { LogOutIcon } from 'lucide-react'
import { useAppName } from '@renderer/hooks/useAppName'

export const Route = createFileRoute('/settings/about')({
  component: AboutSettings
})

const containerVariants = {
  hidden: { opacity: 0, filter: 'blur(10px)' },
  visible: {
    opacity: 1,
    filter: 'blur(0px)',
    transition: {
      delayChildren: 0.2,
      staggerChildren: 0.15
    }
  }
}

const itemVariants = {
  hidden: { opacity: 0, y: 20, filter: 'blur(10px)' },
  visible: {
    opacity: 1,
    y: 0,
    filter: 'blur(0px)',
    transition: {
      duration: 1,
      ease: 'easeOut' as const
    }
  }
} as const

function AboutSettings() {
  const { signOut } = useAuth()
  const { buildChannel } = useAppName()
  return (
    <div className="relative h-full">
      <SettingsContent className="p-0 gap-5 relative z-10 flex flex-col items-center justify-center">
        <motion.div
          variants={containerVariants}
          initial="hidden"
          animate="visible"
          className="flex flex-col items-center gap-5"
        >
          <motion.img
            variants={itemVariants}
            src={enchantedIcon}
            alt="Enchanted"
            className="w-24 h-24"
          />
          <motion.h1 variants={itemVariants} className="text-4xl font-semibold capitalize">
            {buildChannel === 'latest' ? 'Enchanted' : 'Enchanted Dev'}
          </motion.h1>
          <motion.div variants={itemVariants}>
            <Versions />
          </motion.div>
          <motion.p className="text-sm text-muted-foreground" variants={itemVariants}>
            Made with 💚 by{' '}
            <a href="https://www.freysa.ai" target="_blank" rel="noopener noreferrer">
              Freysa
            </a>
          </motion.p>
          <motion.div variants={itemVariants}>
            <Button variant="ghost" className="flex items-center justify-start" onClick={signOut}>
              <LogOutIcon className="mr-2" />
              Sign Out
            </Button>
          </motion.div>
        </motion.div>
      </SettingsContent>
    </div>
  )
}
