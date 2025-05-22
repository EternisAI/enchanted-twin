import { motion } from 'framer-motion'

type AudioIndicatorProps = {
  speaking: boolean
}

export default function AudioIndicator({ speaking }: AudioIndicatorProps) {
  return (
    <div className="flex items-center justify-center w-full h-full relative">
      {speaking ? (
        <>
          <div className="w-24 h-24 bg-green-500 rounded-full z-10" />
          {[0, 1, 2].map((i) => (
            <motion.div
              key={i}
              className="absolute w-48 h-48 border border-green-400 rounded-full"
              initial={{ scale: 0.5, opacity: 0.6 }}
              animate={{ scale: 1.5, opacity: 0 }}
              transition={{
                duration: 1.5,
                ease: 'easeOut',
                repeat: Infinity,
                delay: i * 0.5
              }}
            />
          ))}
        </>
      ) : (
        <motion.div
          className="w-24 h-24 bg-green-500 rounded-full"
          animate={{ y: [0, -20, 0] }}
          transition={{ repeat: Infinity, duration: 1.5, ease: 'easeInOut' }}
        />
      )}
    </div>
  )
}
