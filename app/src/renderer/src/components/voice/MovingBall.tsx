import { motion } from 'framer-motion'

export default function MovingBall() {
  return (
    <div className="flex items-center justify-center w-full h-full">
      <motion.div
        className="w-24 h-24 bg-green-500 rounded-full"
        animate={{ y: [0, -20, 0] }}
        transition={{ repeat: Infinity, duration: 1.5, ease: 'easeInOut' }}
      />
    </div>
  )
}
