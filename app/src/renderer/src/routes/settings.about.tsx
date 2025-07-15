import { createFileRoute } from '@tanstack/react-router'
import Versions from '@renderer/components/Versions'
import { SettingsContent } from '@renderer/components/settings/SettingsContent'
import { Brain } from '@renderer/components/graphics/brain'
import enchantedIcon from '@resources/icon.png'
import { motion } from 'framer-motion'
import { Button } from '@renderer/components/ui/button'
import { useAuth } from '@renderer/contexts/AuthContext'
import { LogOutIcon } from 'lucide-react'
import { ErrorBoundary } from '@renderer/components/ui/error-boundary'

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
      ease: 'easeOut'
    }
  }
}

function AboutSettings() {
  const { signOut } = useAuth()
  return (
    <div className="relative h-full">
      <motion.div
        initial={{ opacity: 0 }}
        animate={{ opacity: 0.15 }}
        transition={{ duration: 2, ease: 'easeOut', delay: 1 }}
        className="absolute inset-0 z-0 opacity-15 h-screen isolate bg-radial from-[#667eea] to-[#764ba2]"
      >
        <Brain />
      </motion.div>
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
          <motion.h1 variants={itemVariants} className="text-4xl font-semibold">
            Enchanted
          </motion.h1>
          <motion.div variants={itemVariants}>
            <ErrorBoundary
              componentName="Versions"
              fallback={
                <div className="w-full border-none flex flex-col gap-2 items-center text-center">
                  <h2 className="text-2xl font-semibold">Version Information</h2>
                  <p className="text-sm text-muted-foreground">Unable to load version details</p>
                </div>
              }
            >
              <Versions />
            </ErrorBoundary>
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
