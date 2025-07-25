import { useMemo, useState, useEffect } from 'react'
import { useDebounce } from '@renderer/hooks/useDebounce'
import { AnimatePresence, motion } from 'framer-motion'

import { Chat, Message, Role, ToolCall } from '@renderer/graphql/generated/graphql'
import VoiceVisualizer from './VoiceVisualizer'
import { UserMessageBubble } from '../messages/Message'
import ToolCallCenter from './toolCallCenter/ToolCallCenter'
import { getToolConfig } from '../config'
import { getMockFrequencyData } from '@renderer/lib/utils'
import useVoiceAgent from '@renderer/hooks/useVoiceAgent'
import { VoiceModeInput } from './VoiceModeInput'
import { AnonToggleButton } from '../AnonToggleButton'
import { TypingIndicator } from '../TypingIndicator'
import Markdown from '../messages/Markdown'

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
  isAnonymized: boolean
  setIsAnonymized: (isAnonymized: boolean) => void
}

export default function VoiceModeChatView({
  chat,
  messages,
  activeToolCalls,
  historicToolCalls,
  stopVoiceMode,
  error,
  chatPrivacyDict,
  isAnonymized,
  setIsAnonymized
}: VoiceModeChatViewProps) {
  const { isAgentSpeaking, agentState } = useVoiceAgent()

  const [assistantMessageStack, setAssistantMessageStack] = useState<Set<string>>(new Set([]))
  const [lastAgentMessage, setLastAgentMessage] = useState<Message | null>(null)

  const debouncedMessages = useDebounce(messages, 250)

  useEffect(() => {
    // This is needed because for voice mode it does reprocess entire chat history
    if (!chat || debouncedMessages.length === 0) return

    const lastAgentMessage = debouncedMessages
      .filter((m) => m.role === Role.Assistant)
      .sort((a, b) => a.createdAt - b.createdAt)
      .pop()

    if (
      lastAgentMessage &&
      lastAgentMessage.text &&
      !assistantMessageStack.has(lastAgentMessage.text)
    ) {
      setAssistantMessageStack((prev) => new Set([...prev, lastAgentMessage.text || '']))
      setLastAgentMessage(lastAgentMessage)
    }
  }, [chat, debouncedMessages, assistantMessageStack])

  const lastUserMessage = useMemo(() => {
    if (!chat || debouncedMessages.length === 0) return null
    const lastUserMessage = debouncedMessages.filter((m) => m.role === Role.User).pop()
    return lastUserMessage || null
  }, [chat, debouncedMessages])

  const visualState: 0 | 1 | 2 = isAgentSpeaking ? 2 : isAgentSpeaking ? 2 : 0

  const currentToolCall = activeToolCalls.find((tc) => !tc.isCompleted)
  const { toolUrl } = getToolConfig(currentToolCall?.name || '')

  const hasUserMessages = messages.some((msg) => msg.role === Role.User)
  const showAnonymizationToggle = !!(hasUserMessages && chatPrivacyDict)

  return (
    <div className="flex h-full w-full items-start">
      <div className="flex flex-col h-full w-full items-center relative justify-between">
        {showAnonymizationToggle && (
          <AnonToggleButton isAnonymized={isAnonymized} setIsAnonymized={setIsAnonymized} />
        )}

        <motion.div
          className="relative w-full h-120"
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

          <div className="flex-1 w-full flex flex-col items-center gap-6 z-10 min-h-[70%] max-h-[70%] overflow-y-auto voice-chat-scrollbar">
            {agentState === 'thinking' && <TypingIndicator />}
            {lastAgentMessage && agentState !== 'thinking' && (
              <motion.div
                key={lastAgentMessage.text}
                className="text-black dark:text-white text-lg text-center max-w-xl break-words"
                initial={{ opacity: 0, y: 20 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.3, ease: 'easeOut' }}
              >
                <Markdown>{lastAgentMessage.text || ''}</Markdown>
              </motion.div>
            )}
          </div>
        </motion.div>

        <div className="w-full max-w-4xl flex flex-col gap-4 px-2 pb-4 z-10">
          {lastUserMessage && (
            <motion.div
              key={lastUserMessage.id}
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.3, ease: 'easeOut' }}
              className="flex flex-row items-center justify-center"
            >
              <UserMessageBubble
                showTimestamp={false}
                message={lastUserMessage}
                chatPrivacyDict={chatPrivacyDict}
                isAnonymized={isAnonymized}
              />
            </motion.div>
          )}
          {error && (
            <div className="py-2 px-4 rounded-md border border-red-500 bg-red-500/10 text-red-500">
              Error: {error}
            </div>
          )}
          <AnimatePresence mode="wait">
            <VoiceModeInput onStop={stopVoiceMode} />
          </AnimatePresence>
        </div>
      </div>
      <style>{`
        .voice-chat-scrollbar::-webkit-scrollbar {
          width: 8px;
        }
        .voice-chat-scrollbar::-webkit-scrollbar-track {
          background: #f1f1f1;
          border-radius: 4px;
        }
        .voice-chat-scrollbar::-webkit-scrollbar-thumb {
          background: #888;
          border-radius: 4px;
        }
        .voice-chat-scrollbar::-webkit-scrollbar-thumb:hover {
          background: #555;
        }
      `}</style>
    </div>
  )
}
