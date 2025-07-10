import { createFileRoute } from '@tanstack/react-router'
import Versions from '@renderer/components/Versions'
import { SettingsContent } from '@renderer/components/settings/SettingsContent'
import { Brain } from '@renderer/components/graphics/brain'
import enchantedIcon from '@resources/icon.png'
import { motion } from 'framer-motion'

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
            <Versions />
          </motion.div>
          <motion.p className="text-sm text-muted-foreground" variants={itemVariants}>
            Made with 💚 by{' '}
            <a href="https://www.freysa.ai" target="_blank" rel="noopener noreferrer">
              Freysa
            </a>
          </motion.p>
        </motion.div>
      </SettingsContent>
    </div>
  )
}
