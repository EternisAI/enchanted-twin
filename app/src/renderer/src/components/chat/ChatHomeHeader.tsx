import { motion } from 'framer-motion'
import { TwinNameInput } from './personalize/TwinNameInput'
import { ContextCard } from './personalize/ContextCard'

export function ChatHomeHeader() {
  return (
    <motion.div
      key="header"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      transition={{ type: 'spring', stiffness: 350, damping: 55 }}
      className="flex flex-col items-center py-4 px-4 w-full"
    >
      <TwinNameInput />
      <motion.div layout="position" className="w-full mt-2">
        <ContextCard />
      </motion.div>
    </motion.div>
  )
}
