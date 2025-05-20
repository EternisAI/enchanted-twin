import { useEffect, useRef, useState } from 'react'

import { Chat, Message, Role, ToolCall } from '@renderer/graphql/generated/graphql'
import MessageInput from '../MessageInput'
import { Switch } from '../../ui/switch'
import VoiceVisualizer from './VoiceVisualizer'
import { useSendMessage } from '@renderer/hooks/useChat'
import { useMessageSubscription } from '@renderer/hooks/useMessageSubscription'
import { useTTS } from '@renderer/hooks/useTTS'
import { UserMessageBubble } from '../Message'
import { motion, AnimatePresence } from 'framer-motion'
import { Badge } from '../../ui/badge'
import { CheckCircle, LoaderIcon } from 'lucide-react'
import { cn } from '@renderer/lib/utils'
import { formatToolName } from '../config'
import { useToolCallUpdate } from '@renderer/hooks/useToolCallUpdate'

interface VoiceModeChatViewProps {
  chat: Chat
  messages: Message[]
  initialMessage?: string
  toggleVoiceMode: () => void
}

export default function VoiceModeChatView({
  chat,
  initialMessage,
  messages,
  toggleVoiceMode
}: VoiceModeChatViewProps) {
  const { isSpeaking, speak, getFreqData, isLoading } = useTTS()
  const triggeredRef = useRef(false)
  const [activeToolCalls, setActiveToolCalls] = useState<ToolCall[]>([])

  const [lastUserMessage, setLastUserMessage] = useState<Message | null>(() => {
    if (!chat) return null
    if (messages.length > 0) {
      const lastUserMessage = messages.filter((m) => m.role === Role.User).pop()
      if (lastUserMessage) return lastUserMessage
    }

    if (initialMessage && messages.length === 0) {
      return {
        id: `temp-${Date.now()}`,
        text: initialMessage,
        imageUrls: [],
        role: Role.User,
        toolCalls: [],
        toolResults: [],
        createdAt: new Date().toISOString()
      }
    }
    return [...messages].reverse().find((m) => m.role === Role.User) ?? null
  })

  const { sendMessage: sendMessageRaw } = useSendMessage(chat.id, setLastUserMessage, () => {})

  const sendMessage = (text: string, reasoning: boolean) => {
    sendMessageRaw(text, reasoning, true)
  }

  useMessageSubscription(chat.id, (message) => {
    if (message.role === Role.Assistant) {
      triggeredRef.current = true
      speak(message.text ?? '')

      message.toolCalls.forEach((toolCall) => {
        if (toolCall.name === 'image') {
          // Handle image tool calls if needed
        }
      })
    }
  })

  useToolCallUpdate(chat.id, (toolCall) => {
    setActiveToolCalls((prev) => {
      const existingIndex = prev.findIndex((tc) => tc.id === toolCall.id)
      if (existingIndex !== -1) {
        const updated = [...prev]
        updated[existingIndex] = { ...updated[existingIndex], ...toolCall }
        return updated
      }
      return [...prev, toolCall]
    })
  })

  const visualState: 0 | 1 | 2 = isSpeaking ? 2 : isLoading ? 1 : 0

  useEffect(() => {
    console.log({ isSpeaking, isLoading })
  }, [isSpeaking, isLoading])

  useEffect(() => {
    console.log('isLoading', isLoading)
  }, [isLoading])

  /* when audio actually starts, drop loading state */
  useEffect(() => {
    if (isSpeaking && triggeredRef.current) {
      triggeredRef.current = false
    }
  }, [isSpeaking])

  useEffect(() => {
    console.log('visualState', visualState)
  }, [visualState])

  return (
    <div className="flex flex-col h-full w-full items-center">
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
        />
        <AnimatePresence>
          {activeToolCalls.length > 0 && (
            <motion.div
              className="absolute top-4 left-1/2 -translate-x-1/2 flex flex-col gap-4 items-center"
              initial={{ opacity: 0, y: -20 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -20 }}
            >
              {activeToolCalls.map((toolCall) => {
                const { toolNameInProgress, toolNameCompleted } = formatToolName(toolCall.name)
                return (
                  <motion.div
                    key={toolCall.id}
                    initial={{ opacity: 0, scale: 0.8 }}
                    animate={{ opacity: 1, scale: 1 }}
                    exit={{ opacity: 0, scale: 0.8 }}
                    className={cn(
                      'flex items-center gap-3',
                      toolCall.isCompleted ? 'text-green-600' : 'text-muted-foreground'
                    )}
                  >
                    {toolCall.isCompleted ? (
                      <Badge
                        className="text-green-600 border-green-500 px-4 py-2 text-base"
                        variant="outline"
                      >
                        <CheckCircle className="h-14 w-14 mr-2" />
                        <span>{toolNameCompleted}</span>
                      </Badge>
                    ) : (
                      <Badge variant="outline" className="border-gray-300 px-4 py-2 text-base">
                        <LoaderIcon className="h-10 w-10 mr-2 animate-spin" />
                        <span>{toolNameInProgress}...</span>
                      </Badge>
                    )}
                  </motion.div>
                )
              })}
            </motion.div>
          )}
        </AnimatePresence>
      </motion.div>

      <div className="w-full max-w-4xl flex flex-col gap-4 px-2 pb-4">
        {lastUserMessage && <UserMessageBubble message={lastUserMessage} />}
        <VoiceModeSwitch voiceMode setVoiceMode={toggleVoiceMode} />
        <MessageInput
          isWaitingTwinResponse={false}
          onSend={sendMessage}
          hasReasoning={false}
          voice
        />
      </div>
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
