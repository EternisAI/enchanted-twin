import { useEffect, useMemo, useRef } from 'react'
import { motion } from 'framer-motion'

import { Chat, Message, Role, ToolCall } from '@renderer/graphql/generated/graphql'
import MessageInput from '../MessageInput'
import { Switch } from '../../ui/switch'
import VoiceVisualizer from './VoiceVisualizer'
import { useMessageSubscription } from '@renderer/hooks/useMessageSubscription'
import { useTTS } from '@renderer/hooks/useTTS'
import { UserMessageBubble } from '../Message'
import ToolCallCenter from './toolCallCenter/ToolCallCenter'
import { extractReasoningAndReply, getToolUrl } from '../config'
import useDependencyStatus from '@renderer/hooks/useDependencyStatus'
import { Tooltip } from '@renderer/components/ui/tooltip'
import { TooltipContent, TooltipTrigger } from '@radix-ui/react-tooltip'
import { Button } from '@renderer/components/ui/button'
import { Mic, Mic2, MicOff, X } from 'lucide-react'
import { cn } from '@renderer/lib/utils'

interface VoiceModeChatViewProps {
  chat: Chat
  messages: Message[]
  activeToolCalls: ToolCall[]
  historicToolCalls: ToolCall[]
  onSendMessage: (text: string, reasoning: boolean, voice: boolean) => void
  toggleVoiceMode: () => void
  error: string
  isWaitingTwinResponse: boolean
}

export default function VoiceModeChatView({
  chat,
  messages,
  activeToolCalls,
  historicToolCalls,
  onSendMessage,
  toggleVoiceMode,
  error
}: VoiceModeChatViewProps) {
  const { isSpeaking, speak, getFreqData, stop, isLoading } = useTTS()

  const triggeredRef = useRef(false)

  const lastUserMessage = useMemo(() => {
    if (!chat || messages.length === 0) return null
    const lastUserMessage = messages.filter((m) => m.role === Role.User).pop()
    return lastUserMessage || null
  }, [chat, messages])

  useMessageSubscription(chat.id, (message) => {
    if (message.role === Role.Assistant) {
      triggeredRef.current = true
      const { replyText } = extractReasoningAndReply(message.text ?? '')
      speak(replyText ?? '')
    }
  })

  const visualState: 0 | 1 | 2 = isSpeaking ? 2 : isLoading ? 1 : 0

  useEffect(() => {
    return () => stop()
  }, [stop])

  useEffect(() => {
    if (isSpeaking && triggeredRef.current) {
      triggeredRef.current = false
    }
  }, [isSpeaking])

  const currentToolCall = activeToolCalls.find((tc) => !tc.isCompleted)
  const toolUrl = getToolUrl(currentToolCall?.name)

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
            getFreqData={getFreqData}
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
              <UserMessageBubble message={lastUserMessage} />
            </motion.div>
          )}
          {error && (
            <div className="py-2 px-4 rounded-md border border-red-500 bg-red-500/10 text-red-500">
              Error: {error}
            </div>
          )}
          {/* <VoiceModeSwitch voiceMode setVoiceMode={toggleVoiceMode} /> */}
          <VoiceModeInput isMuted={false} isAgentSpeaking={isSpeaking} onStop={toggleVoiceMode} />
          {/* <MessageInput
            isWaitingTwinResponse={isLoading || isSpeaking}
            onSend={onSendMessage}
            onStop={stop}
            isReasonSelected={false}
            voiceMode
          /> */}
        </div>
      </div>
    </div>
  )
}

function VoiceModeInput({
  isMuted,
  isAgentSpeaking,
  onStop
}: {
  isMuted: boolean
  isAgentSpeaking: boolean
  onStop: () => void
}) {
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
            // onClick={onClick}
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

export function VoiceModeSwitch({
  voiceMode,
  setVoiceMode
}: {
  voiceMode: boolean
  setVoiceMode: (voiceMode: boolean) => void
}) {
  const { isVoiceReady } = useDependencyStatus()

  return (
    <Tooltip>
      <div className="flex justify-end w-full gap-2">
        <TooltipTrigger asChild>
          <div className="flex items-center gap-2">
            <Switch
              id="voiceMode"
              className="data-[state=unchecked]:bg-foreground/30"
              checked={voiceMode}
              onCheckedChange={() => {
                setVoiceMode(!voiceMode)
              }}
              disabled={!voiceMode && !isVoiceReady}
            />
            <label className="text-sm" htmlFor="voiceMode">
              Voice Mode
            </label>
          </div>
        </TooltipTrigger>
      </div>
      <TooltipContent>{isVoiceReady ? '' : 'Installing dependencies...'}</TooltipContent>
    </Tooltip>
  )
}
