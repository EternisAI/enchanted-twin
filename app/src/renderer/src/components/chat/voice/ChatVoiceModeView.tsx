import { useEffect, useMemo, useRef, useState } from 'react'
import { motion } from 'framer-motion'

import { Chat, Message, Role, ToolCall } from '@renderer/graphql/generated/graphql'
import MessageInput from '../MessageInput'
import { Switch } from '../../ui/switch'
import VoiceVisualizer from './VoiceVisualizer'
import { useMessageSubscription } from '@renderer/hooks/useMessageSubscription'
import { useTTS } from '@renderer/hooks/useTTS'
import { UserMessageBubble } from '../Message'
import { useToolCallUpdate } from '@renderer/hooks/useToolCallUpdate'
import ToolCallCenter from './toolCallCenter/ToolCallCenter'

interface VoiceModeChatViewProps {
  chat: Chat
  messages: Message[]
  onSendMessage: (text: string, reasoning: boolean, voice: boolean) => void
  toggleVoiceMode: () => void
  isWaitingTwinResponse: boolean
}

export default function VoiceModeChatView({
  chat,
  messages,
  onSendMessage,
  toggleVoiceMode
}: VoiceModeChatViewProps) {
  const { isSpeaking, speak, getFreqData, stop, isLoading } = useTTS()
  const triggeredRef = useRef(false)
  const [activeToolCalls, setActiveToolCalls] = useState<ToolCall[]>([])

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

  /* when audio actually starts, drop loading state */
  useEffect(() => {
    if (isSpeaking && triggeredRef.current) {
      triggeredRef.current = false
    }
  }, [isSpeaking])

  const currentToolCall = activeToolCalls.find((tc) => !tc.isCompleted)
  const toolUrl = useMemo(() => {
    if (currentToolCall?.name === 'generate_image') {
      return 'image:https://upload.wikimedia.org/wikipedia/commons/thumb/5/54/NotoSans_-_Frame_With_Picture_-_1F5BC.svg/330px-NotoSans_-_Frame_With_Picture_-_1F5BC.svg.png'
    } else if (currentToolCall?.name === 'perplexity_ask') {
      return 'image:https://upload.wikimedia.org/wikipedia/commons/thumb/5/55/Magnifying_glass_icon.svg/480px-Magnifying_glass_icon.svg.png'
    }
    return undefined
  }, [currentToolCall])

  return (
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
          tool={toolUrl}
        />

        {/* @TODO: Splti active tool calls from last message and historic everything else */}
        <ToolCallCenter activeToolCalls={activeToolCalls} />
      </motion.div>

      <div className="w-full max-w-4xl flex flex-col gap-4 px-2 pb-4">
        {lastUserMessage && <UserMessageBubble message={lastUserMessage} />}
        <VoiceModeSwitch voiceMode setVoiceMode={toggleVoiceMode} />
        <MessageInput
          isWaitingTwinResponse={isLoading || isSpeaking}
          onSend={onSendMessage}
          onStop={stop}
          hasReasoning={false}
          isReasonSelected={true}
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
