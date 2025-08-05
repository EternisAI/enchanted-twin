import { motion } from 'framer-motion'
import VoiceVisualizer from './voice/VoiceVisualizer'

export function VoiceVisualizerSection() {
  return (
    <motion.div
      key="voice-visualizer"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      transition={{ type: 'spring', stiffness: 350, damping: 55 }}
      className="flex-1 w-full flex items-center justify-center min-h-[300px]"
    >
      <VoiceVisualizer
        visualState={1}
        getFreqData={() => new Uint8Array()}
        className="min-w-60 min-h-40"
      />
    </motion.div>
  )
}
