import { AnimatePresence, motion } from 'framer-motion'
import { MessageSquareIcon, Mic, MicOff } from 'lucide-react'

import useMicrophonePermission from '@renderer/hooks/useMicrophonePermission'
import useVoiceAgent from '@renderer/hooks/useVoiceAgent'
import { Button } from '@renderer/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@renderer/components/ui/tooltip'
import { cn } from '@renderer/lib/utils'

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
        key="initializing-voice-session"
        initial={{ opacity: 0, y: 20 }}
        exit={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ type: 'spring', stiffness: 100, damping: 10, mass: 0.5 }}
        className="flex flex-col gap-4 items-center pb-4"
      >
        <div className="flex flex-col items-center gap-1.5 px-4 py-3">
          <Mic className="w-5 h-5 flex-shrink-0" />
          <span className="text-lg font-medium">Starting voice conversation</span>
          <div className="w-32 h-1 bg-neutral-200 dark:bg-neutral-700 rounded-full overflow-hidden">
            <motion.div
              className="h-full bg-neutral-500 dark:bg-neutral-400"
              initial={{ width: '0%' }}
              animate={{ width: '100%' }}
              transition={{
                duration: 10,
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
        key="allow-microphone-access"
        initial={{ opacity: 0, y: 20 }}
        exit={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ type: 'spring', stiffness: 100, damping: 10, mass: 0.5 }}
        className="flex flex-col gap-4 items-center pb-4"
      >
        <div className="flex flex-col items-center gap-1 px-4 py-3">
          <Mic className="w-5 h-5 flex-shrink-0" />
          <span className="text-lg font-semibold">Allow Microphone Access</span>
          <span className="text-sm text-muted-foreground">
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
      key="message-input-container"
      initial={{ opacity: 0, y: 20 }}
      exit={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ type: 'spring', stiffness: 100, damping: 10, mass: 0.5 }}
      className="flex gap-2 mx-auto justify-center p-2 w-fit rounded-full shadow-xl items-center bg-card"
      layout
    >
      <Tooltip>
        <TooltipTrigger asChild>
          <motion.button
            key="mute-button"
            layout
            transition={{ type: 'spring', stiffness: 100, damping: 10, mass: 0.5 }}
            onClick={toggleMute}
            className={cn(
              'cursor-pointer active:opacity-90 px-4 h-10 rounded-full transition-colors shadow-none hover:shadow-none active:shadow-none border-none !bg-accent hover:!bg-accent/70 dark:!hover:!bg-accent/70',
              isMuted &&
                'text-red-500 dark:text-red-400 !bg-red-100 hover:!bg-red-200/70 dark:!bg-red-600/20 dark:!hover:!bg-red-600/70'
            )}
          >
            <motion.div layout className="flex items-center gap-2">
              <AnimatePresence mode="wait">
                {isMuted ? (
                  <motion.div
                    key="muted"
                    layout
                    exit={{ opacity: 0, scale: 0.9 }}
                    initial={{ opacity: 0, scale: 0.9 }}
                    animate={{ opacity: 1, scale: 1 }}
                    transition={{ duration: 0.2, ease: 'easeInOut' }}
                    className="flex items-center gap-2"
                  >
                    <MicOff className="w-4 h-4" />
                    <span className="text-sm">Muted</span>
                  </motion.div>
                ) : (
                  <motion.div
                    key="listening"
                    layout
                    exit={{ opacity: 0, scale: 0.9 }}
                    initial={{ opacity: 0, scale: 0.9 }}
                    animate={{ opacity: 1, scale: 1 }}
                    transition={{ duration: 0.2, ease: 'easeInOut' }}
                    className="flex items-center gap-2"
                  >
                    <Mic className="w-4 h-4 font-bold" fontWeight="bold" />
                    <span className="text-sm font-medium">I&apos;m listening...</span>
                  </motion.div>
                )}
              </AnimatePresence>
            </motion.div>
          </motion.button>
        </TooltipTrigger>
        <TooltipContent className="px-3 py-1 rounded-lg">
          {isMuted ? 'Unmute' : 'Mute'}
        </TooltipContent>
      </Tooltip>
      {onStop && (
        <motion.div
          layout
          key="stop-button"
          transition={{ type: 'spring', stiffness: 100, damping: 10, mass: 0.5 }}
        >
          <Button
            onClick={onStop}
            size="icon"
            className={cn(
              '!px-2.5 active:scale-95 !bg-accent !hover:bg-accent/50 rounded-full shadow-none hover:shadow-none active:shadow-none border-none'
            )}
            variant="outline"
          >
            <MessageSquareIcon className="w-4 h-4" />
          </Button>
        </motion.div>
      )}
    </motion.div>
  )
}
