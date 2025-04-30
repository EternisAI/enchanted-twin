import { motion, useAnimation } from 'framer-motion'
import { useEffect } from 'react'
import { cn } from '@renderer/lib/utils'

export type TwinAvatarState = 'idle' | 'thinking' | 'alert' | 'square'

interface TwinAvatarProps {
  state?: TwinAvatarState
  className?: string
  size?: number
  color?: string
}

// Define different blob shapes for morphing
const blobShapes = [
  'M43.9,-51.2C57,-41.3,67.8,-27.6,73.4,-10.8C78.9,6,79.2,26,70,39C60.7,52,42,58,25.6,59.3C9.1,60.6,-5.2,57,-21.4,53.5C-37.7,49.9,-55.9,46.3,-69,34.6C-82.1,22.8,-90.1,2.7,-83.5,-11.2C-76.9,-25.2,-55.6,-33.1,-39.3,-42.4C-23,-51.8,-11.5,-62.5,2,-64.8C15.4,-67.1,30.8,-61.1,43.9,-51.2Z',
  'M45.1,-54.3C58.3,-44.1,68.4,-29.1,72.1,-12.1C75.8,4.9,73.1,23.9,63.7,38.7C54.3,53.5,38.2,64.1,20.7,69.8C3.2,75.5,-15.7,76.3,-32.9,70.3C-50.1,64.3,-65.6,51.5,-73.1,34.7C-80.6,17.9,-80.1,-2.9,-73.1,-20.7C-66.1,-38.5,-52.6,-53.3,-36.2,-62.5C-19.8,-71.7,-0.4,-75.3,17.7,-71.3C35.8,-67.3,52,-55.7,45.1,-54.3Z',
  'M38.2,-46.1C50.3,-36.1,61.3,-23.5,65.9,-8.4C70.5,6.7,68.7,24.2,59.3,37.3C49.9,50.4,32.9,59.1,15.3,63.8C-2.3,68.5,-20.5,69.2,-36.2,63.2C-51.9,57.2,-65.1,44.5,-70.5,28.3C-75.9,12.1,-73.5,-7.6,-65.1,-24.3C-56.7,-41,-42.3,-54.7,-26.1,-63.2C-9.9,-71.7,8.1,-75,24.7,-70.3C41.3,-65.6,56.5,-52.9,38.2,-46.1Z'
]

export const TwinAvatar = ({
  state = 'idle',
  className,
  size = 40,
  color = '#FF0066'
}: TwinAvatarProps) => {
  const controls = useAnimation()

  useEffect(() => {
    switch (state) {
      case 'idle':
        controls.start({
          d: blobShapes,
          transition: {
            duration: 8,
            repeat: Infinity,
            ease: 'easeInOut',
            times: [0, 0.5, 1]
          }
        })
        break
      case 'thinking':
        controls.start({
          scale: [1, 1.05, 1],
          transition: {
            duration: 2,
            repeat: Infinity,
            ease: 'easeInOut'
          }
        })
        break
      case 'alert':
        controls.start({
          scale: [1, 1.1, 1],
          transition: {
            duration: 1,
            repeat: Infinity,
            ease: 'easeInOut'
          }
        })
        break
      case 'square':
        controls.start({
          d: 'M-50,-50 L50,-50 L50,50 L-50,50 Z',
          transition: {
            duration: 0.3,
            ease: 'easeInOut'
          }
        })
        break
    }
  }, [state, controls])

  return (
    <div className={cn('relative', className)}>
      <svg
        width={size}
        height={size}
        viewBox="0 0 200 200"
        xmlns="http://www.w3.org/2000/svg"
        className="text-primary"
        style={{ overflow: 'visible' }}
      >
        <defs>
          <filter id="glow" x="-20%" y="-20%" width="140%" height="140%">
            <feGaussianBlur stdDeviation="2" result="blur" />
            <feComposite in="SourceGraphic" in2="blur" operator="over" />
          </filter>
          <linearGradient id="orbGradient" x1="0%" y1="0%" x2="100%" y2="100%">
            <stop offset="0%" stopColor={color} stopOpacity="0.8" />
            <stop offset="100%" stopColor={color} stopOpacity="1" />
          </linearGradient>
        </defs>

        <motion.g transform="translate(100 100)">
          <motion.path
            fill={state === 'square' ? color : 'url(#orbGradient)'}
            filter="url(#glow)"
            animate={controls}
            d={blobShapes[0]}
          />

          {state === 'thinking' && (
            <>
              <motion.circle
                r="90"
                stroke={color}
                strokeWidth="2"
                fill="none"
                initial={{ scale: 1, opacity: 0 }}
                animate={{
                  scale: [1, 1.1, 1],
                  opacity: [0, 0.3, 0]
                }}
                transition={{
                  duration: 1.5,
                  repeat: Infinity,
                  ease: 'easeInOut'
                }}
              />
              <motion.circle
                r="90"
                stroke={color}
                strokeWidth="2"
                fill="none"
                initial={{ scale: 1, opacity: 0 }}
                animate={{
                  scale: [1, 1.1, 1],
                  opacity: [0, 0.3, 0]
                }}
                transition={{
                  duration: 1.5,
                  repeat: Infinity,
                  ease: 'easeInOut',
                  delay: 0.5
                }}
              />
            </>
          )}
        </motion.g>
      </svg>
    </div>
  )
}
