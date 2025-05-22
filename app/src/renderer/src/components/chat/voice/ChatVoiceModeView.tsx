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
import { getToolUrl } from '../config'

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

  const lastAssistantMessage = useMemo(() => {
    if (!chat || messages.length === 0) return null
    const lastAssistantMessage = messages.filter((m) => m.role === Role.Assistant).pop()
    return lastAssistantMessage || null
  }, [chat, messages])

  const lastUserMessage = useMemo(() => {
    if (!chat || messages.length === 0) return null
    const lastUserMessage = messages.filter((m) => m.role === Role.User).pop()
    return lastUserMessage || null
  }, [chat, messages])

  useMessageSubscription(chat.id, (message) => {
    if (message.role === Role.Assistant) {
      triggeredRef.current = true
      speak(message.text ?? '')
    }
  })

  const visualState: 0 | 1 | 2 = isSpeaking ? 2 : isLoading ? 1 : 0

  useEffect(() => {
    console.log({ isSpeaking, isLoading })
  }, [isSpeaking, isLoading])

  /* when audio actually starts, drop loading state */
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
            assistantTextMessage={lastAssistantMessage?.text ?? undefined}
            toolUrl={toolUrl}
          />
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
          <VoiceModeSwitch voiceMode setVoiceMode={toggleVoiceMode} />
          <MessageInput
            isWaitingTwinResponse={isLoading || isSpeaking}
            onSend={onSendMessage}
            onStop={stop}
            isReasonSelected={true}
            voiceMode
          />
        </div>
      </div>
      <ToolCallCenter activeToolCalls={activeToolCalls} historicToolCalls={historicToolCalls} />
    </div>
  )
}

export function VoiceModeSwitch({
  voiceMode,
  setVoiceMode
}: {
  voiceMode: boolean
  setVoiceMode: (voiceMode: boolean) => void
}) {
  return (
    <div className="flex justify-end w-full gap-2">
      <Switch
        id="voiceMode"
        className="data-[state=unchecked]:bg-foreground/30"
        checked={voiceMode}
        onCheckedChange={() => {
          setVoiceMode(!voiceMode)
        }}
      >
        Voice Mode
      </Switch>
      <label className="text-sm" htmlFor="voiceMode">
        Voice Mode
      </label>
    </div>
  )
}
