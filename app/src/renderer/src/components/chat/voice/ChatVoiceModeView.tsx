import { useMemo } from 'react'
import { motion } from 'framer-motion'

import { Chat, Message, Role, ToolCall } from '@renderer/graphql/generated/graphql'
import VoiceVisualizer from './VoiceVisualizer'
import { UserMessageBubble } from '../messages/Message'
import ToolCallCenter from './toolCallCenter/ToolCallCenter'
import { getToolConfig } from '../config'
import { getMockFrequencyData } from '@renderer/lib/utils'
import useVoiceAgent from '@renderer/hooks/useVoiceAgent'
import { VoiceModeInput } from './VoiceModeInput'

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
