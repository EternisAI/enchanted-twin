import { useEffect, useMemo, useState } from 'react'
import { motion } from 'framer-motion'
import { TooltipContent, TooltipTrigger } from '@radix-ui/react-tooltip'
import { Mic, MicOff, X } from 'lucide-react'

import { Chat, Message, Role, ToolCall } from '@renderer/graphql/generated/graphql'
import VoiceVisualizer from './VoiceVisualizer'
import { UserMessageBubble } from '../Message'
import ToolCallCenter from './toolCallCenter/ToolCallCenter'
import { getToolConfig } from '../config'
import { Tooltip } from '@renderer/components/ui/tooltip'
import { Button } from '@renderer/components/ui/button'
import { cn, getMockFrequencyData } from '@renderer/lib/utils'
import useVoiceAgent from '@renderer/hooks/useVoiceAgent'
import useDependencyStatus from '@renderer/hooks/useDependencyStatus'

interface VoiceModeChatViewProps {
  chat: Chat
  messages: Message[]
  activeToolCalls: ToolCall[]
  historicToolCalls: ToolCall[]
  onSendMessage: (text: string, reasoning: boolean, voice: boolean) => void
  stopVoiceMode: () => void
  error: string
  isWaitingTwinResponse: boolean
  chatPrivacyDict: string | null
}

export default function VoiceModeChatView({
  chat,
  messages,
  activeToolCalls,
  historicToolCalls,
  stopVoiceMode,
  error,
  chatPrivacyDict
}: VoiceModeChatViewProps) {
  const { isAgentSpeaking } = useVoiceAgent()

  const lastUserMessage = useMemo(() => {
    if (!chat || messages.length === 0) return null
    const lastUserMessage = messages.filter((m) => m.role === Role.User).pop()
    return lastUserMessage || null
  }, [chat, messages])

  const visualState: 0 | 1 | 2 = isAgentSpeaking ? 2 : isAgentSpeaking ? 2 : 0

  const currentToolCall = activeToolCalls.find((tc) => !tc.isCompleted)
  const { toolUrl } = getToolConfig(currentToolCall?.name || '')

  return (
    <div className="flex h-full w-full items-center ">
      <div className="flex flex-col h-full w-full items-center relative">
        <motion.div
          className="relative flex-1 w-full"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ duration: 1 }}
        >
          <VoiceVisualizer
            className="absolute inset-0"
            visualState={visualState}
            getFreqData={getMockFrequencyData}
            toolUrl={toolUrl}
          />
          <ToolCallCenter activeToolCalls={activeToolCalls} historicToolCalls={historicToolCalls} />
        </motion.div>

        <div className="w-full max-w-4xl flex flex-col gap-4 px-2 pb-4">
          {lastUserMessage && (
            <motion.div
              key={lastUserMessage.id}
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.3, ease: 'easeOut' }}
            >
              <UserMessageBubble message={lastUserMessage} chatPrivacyDict={chatPrivacyDict} />
            </motion.div>
          )}
          {error && (
            <div className="py-2 px-4 rounded-md border border-red-500 bg-red-500/10 text-red-500">
              Error: {error}
            </div>
          )}
          <VoiceModeInput onStop={stopVoiceMode} />
        </div>
      </div>
    </div>
  )
}

export function VoiceModeInput({ onStop }: { onStop: () => void }) {
  const { isLiveKitSessionReady } = useDependencyStatus()
  const { isMuted, toggleMute } = useVoiceAgent()
  const [microphoneStatus, setMicrophoneStatus] = useState<
    'granted' | 'denied' | 'not-determined' | 'loading'
  >('loading')
  const [isRequestingAccess, setIsRequestingAccess] = useState(false)

  const queryMicrophoneStatus = async () => {
    try {
      const status = await window.api.queryMediaStatus('microphone')
      setMicrophoneStatus(status as 'granted' | 'denied' | 'not-determined')
    } catch (error) {
      console.error('Error querying microphone status:', error)
      setMicrophoneStatus('denied')
    }
  }

  const requestMicrophoneAccess = async () => {
    try {
      setIsRequestingAccess(true)
      await window.api.requestMediaAccess('microphone')
      await queryMicrophoneStatus()

      window.api.analytics.capture('permission_asked', {
        name: 'microphone'
      })
    } catch (error) {
      console.error('Error requesting microphone access:', error)
    } finally {
      setIsRequestingAccess(false)
    }
  }

  useEffect(() => {
    queryMicrophoneStatus()
    const interval = setInterval(queryMicrophoneStatus, 5000)
    return () => clearInterval(interval)
  }, [])

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
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            onClick={toggleMute}
            className={cn(
              '!px-2.5 rounded-full transition-all shadow-none hover:shadow-lg active:shadow-sm border-none !bg-gray-100 dark:!bg-gray-800 hover:!bg-gray-200 dark:!hover:!bg-gray-700'
            )}
            variant="outline"
          >
            {isMuted ? <MicOff className="w-4 h-4" /> : <Mic className="w-4 h-4" />}
          </Button>
        </TooltipTrigger>
        <TooltipContent className="px-3 py-1 bg-gray-100 rounded-lg">
          {isMuted ? 'Unmute' : 'Mute'}
        </TooltipContent>
      </Tooltip>
      <Button
        onClick={onStop}
        className={cn(
          '!px-2.5 rounded-full transition-all shadow-none hover:shadow-lg active:shadow-sm border-none !bg-gray-100 dark:!bg-gray-800 hover:!bg-red-200 dark:!hover:!bg-red-700 hover:!text-red-500 dark:!hover:!text-red-400'
        )}
        variant="outline"
      >
        <X className="w-4 h-4" />
      </Button>
    </motion.div>
  )
}
