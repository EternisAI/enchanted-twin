import { useRef, useEffect, useState } from 'react'

import MessageList from './MessageList'
import MessageInput from './MessageInput'
import { Chat, ChatCategory } from '@renderer/graphql/generated/graphql'
import { useToolCallUpdate } from '@renderer/hooks/useToolCallUpdate'
import { useMessageStreamSubscription } from '@renderer/hooks/useMessageStreamSubscription'
import { useMessageSubscription } from '@renderer/hooks/useMessageSubscription'
import VoiceModeChatView from './voice/ChatVoiceModeView'
import { useVoiceStore } from '@renderer/lib/stores/voice'
import { useChat } from '@renderer/contexts/ChatContext'
import { Role } from '@renderer/graphql/generated/graphql'
import HolonThreadContext from '../holon/HolonThreadContext'
import useDependencyStatus from '@renderer/hooks/useDependencyStatus'
import { Tooltip, TooltipContent, TooltipTrigger } from '../ui/tooltip'
import { Switch } from '../ui/switch'

interface ChatViewProps {
  chat: Chat
}

export default function ChatView({ chat }: ChatViewProps) {
  const bottomRef = useRef<HTMLDivElement | null>(null)
  const { isVoiceMode, stopVoiceMode, startVoiceMode } = useVoiceStore()
  const [mounted, setMounted] = useState(false)

  const {
    messages,
    isWaitingTwinResponse,
    isReasonSelected,
    error,
    activeToolCalls,
    historicToolCalls,
    sendMessage,
    upsertMessage,
    updateToolCallInMessage,
    setIsWaitingTwinResponse,
    setIsReasonSelected,
    setActiveToolCalls
  } = useChat()

  useMessageSubscription(chat.id, (message) => {
    if (message.role !== Role.User) {
      upsertMessage(message)
      window.api.analytics.capture('message_received', {
        tools: message.toolCalls.map((tool) => tool.name)
      })
    }

    // Messages on voice mode are sent by python code via livekit
    if (message.role === Role.User && isVoiceMode) {
      upsertMessage(message)
      window.api.analytics.capture('voice_message_sent', {
        tools: message.toolCalls.map((tool) => tool.name)
      })
    }
  })

  useMessageStreamSubscription(chat.id, (messageId, chunk, isComplete, imageUrls) => {
    const existingMessage = messages.find((m) => m.id === messageId)
    if (!existingMessage) {
      upsertMessage({
        id: messageId,
        text: chunk ?? '',
        role: Role.Assistant,
        createdAt: new Date().toISOString(),
        imageUrls: imageUrls ?? [],
        toolCalls: [],
        toolResults: []
      })
    } else {
      const allImageUrls = existingMessage.imageUrls.concat(imageUrls ?? [])
      const updatedMessage = {
        ...existingMessage,
        text: (existingMessage.text ?? '') + (chunk ?? ''),
        imageUrls: allImageUrls
      }
      upsertMessage(updatedMessage)
    }

    setIsWaitingTwinResponse(false)
  })

  useToolCallUpdate(chat.id, (toolCall) => {
    updateToolCallInMessage(toolCall)

    // Update active tool calls
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

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: mounted ? 'smooth' : 'instant' })
    if (!mounted) {
      setMounted(true)
    }
  }, [messages, mounted, isVoiceMode])

  if (isVoiceMode) {
    return (
      <VoiceModeChatView
        chat={chat}
        stopVoiceMode={stopVoiceMode}
        messages={messages}
        activeToolCalls={activeToolCalls}
        historicToolCalls={historicToolCalls}
        onSendMessage={sendMessage}
        isWaitingTwinResponse={isWaitingTwinResponse}
        error={error}
      />
    )
  }

  return (
    <div className="flex flex-col h-full w-full items-center">
      <div className="flex flex-1 flex-col w-full overflow-y-auto ">
        <div className="flex w-full justify-center">
          <div className="flex flex-col max-w-4xl items-center p-4 w-full">
            <div className="w-full flex flex-col gap-2">
              {chat.category === ChatCategory.Holon && chat.holonThreadId && (
                <HolonThreadContext threadId={chat.holonThreadId} />
              )}
              <MessageList
                messages={messages}
                isWaitingTwinResponse={isWaitingTwinResponse}
                chatPrivacyDict={chat.privacyDictJson}
              />
              {error && (
                <div className="py-2 px-4 mt-2 rounded-md border border-red-500 bg-red-500/10 text-red-500">
                  Error: {error}
                </div>
              )}
              <div ref={bottomRef} />
            </div>
          </div>
        </div>
      </div>

      <div className="flex flex-col w-full items-center justify-center px-2">
        <div className="pb-4 w-full max-w-4xl flex flex-col gap-4 justify-center items-center ">
          <VoiceModeToggle
            voiceMode={isVoiceMode}
            setVoiceMode={() => {
              if (isVoiceMode) {
                stopVoiceMode()
              } else {
                startVoiceMode(chat.id)
              }
            }}
          />
          <MessageInput
            isWaitingTwinResponse={isWaitingTwinResponse}
            onSend={sendMessage}
            onStop={() => {
              setIsWaitingTwinResponse(false)
            }}
            isReasonSelected={isReasonSelected}
            onReasonToggle={setIsReasonSelected}
          />
        </div>
      </div>
    </div>
  )
}

function VoiceModeToggle({
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
              className="data-[state=unchecked]:bg-foreground/30 cursor-pointer"
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
      {!isVoiceReady && <TooltipContent>Installing dependencies...</TooltipContent>}
    </Tooltip>
  )
}
