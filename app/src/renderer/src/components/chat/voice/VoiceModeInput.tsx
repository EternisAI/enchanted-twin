import { motion } from 'framer-motion'
import { Mic, MicOff, X } from 'lucide-react'

import useMicrophonePermission from '@renderer/hooks/useMicrophonePermission'
import useVoiceAgent from '@renderer/hooks/useVoiceAgent'
import { Button } from '@renderer/components/ui/button'
import { Tooltip } from '@renderer/components/ui/tooltip'
import { cn } from '@renderer/lib/utils'
import { TooltipContent, TooltipTrigger } from '@radix-ui/react-tooltip'

// STATES
// 1. Initializing voice session
// 2. Allow microphone access
// 3. Listening -> Voice mode input
// 4. Muted
// 5. Text mode input
export function VoiceModeInput({ onStop }: { onStop?: () => void }) {
  const { isMuted, toggleMute, isSessionReady: isLiveKitSessionReady } = useVoiceAgent()
  const { microphoneStatus, isRequestingAccess, requestMicrophoneAccess } =
    useMicrophonePermission()

  // 1. Initializing voice session
  if (!isLiveKitSessionReady) {
    return (
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.3, ease: 'easeOut' }}
        className="flex flex-col gap-4 items-center pb-4"
      >
        <div className="flex flex-col items-center gap-1.5 px-4 py-3">
          <Mic className="w-5 h-5 flex-shrink-0" />
          <span className="text-lg font-medium">Initializing voice session</span>
          <div className="w-32 h-1 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
            <motion.div
              className="h-full bg-gray-500 dark:bg-gray-400"
              initial={{ width: '0%' }}
              animate={{ width: '100%' }}
              transition={{
                duration: 5,
                ease: 'linear',
                repeat: Infinity,
                repeatType: 'loop'
              }}
            />
          </div>
        </div>
        <Button onClick={onStop} variant="outline">
          Exit
        </Button>
      </motion.div>
    )
  }

  // 2. Allow microphone access
  if (microphoneStatus !== 'granted') {
    return (
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.3, ease: 'easeOut' }}
        className="flex flex-col gap-4 items-center pb-4"
      >
        <div className="flex flex-col items-center gap-1 px-4 py-3">
          <Mic className="w-5 h-5 flex-shrink-0" />
          <span className="text-lg font-semibold">Allow Microphone Access</span>
          <span className="text-sm text-gray-500 dark:text-gray-400">
            To talk to Enchanted, you&apos;ll need to allow microphone access.
          </span>
        </div>
        <div className="flex gap-2">
          <Button onClick={requestMicrophoneAccess} disabled={isRequestingAccess}>
            {isRequestingAccess ? 'Requesting...' : 'Allow Access'}
          </Button>
          <Button onClick={onStop} variant="outline">
            Exit
          </Button>
        </div>
      </motion.div>
    )
  }

  return (
    <motion.div
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.3, ease: 'easeOut' }}
      className="flex gap-2 justify-center pb-4"
    >
      <div className="flex p-2 gap-2 max-w-md rounded-full shadow-xl h-14 items-center bg-white">
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              onClick={toggleMute}
              className={cn(
                '!px-4 !py-4 h-10 rounded-full transition-all shadow-none hover:shadow-lg active:shadow-sm border-none !bg-[#E4E4E7] dark:!bg-gray-800 hover:!bg-gray-200 dark:!hover:!bg-gray-700',
                isMuted && '!bg-muted-foreground text-white dark:!bg-gray-700'
              )}
              variant="outline"
            >
              {isMuted ? (
                <>
                  <MicOff className="w-4 h-4" />
                  <span className="text-sm">Muted</span>
                </>
              ) : (
                <>
                  <Mic className="w-4 h-4 font-bold" fontWeight="bold" />
                  <span className="text-sm font-medium">I&apos;m listening...</span>
                </>
              )}
            </Button>
          </TooltipTrigger>
          <TooltipContent className="px-3 py-1 bg-gray-100 rounded-lg">
            {isMuted ? 'Unmute' : 'Mute'}
          </TooltipContent>
        </Tooltip>
        {onStop && (
          <Button
            onClick={onStop}
            className={cn(
              '!px-2.5 rounded-full transition-all shadow-none hover:shadow-lg active:shadow-sm border-none !bg-gray-200 dark:!bg-gray-800 hover:!bg-gray-200 dark:!hover:!bg-gray-700 hover:!text-gray-500 dark:!hover:!text-gray-400'
            )}
            variant="outline"
          >
            <X className="w-4 h-4" />
          </Button>
        )}
      </div>
    </motion.div>
  )
}
