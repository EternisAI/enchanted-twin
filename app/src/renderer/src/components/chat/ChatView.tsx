import { useRef, useEffect, useState } from 'react'

import MessageList from './MessageList'
import MessageInput from './MessageInput'
import { Chat, ChatCategory } from '@renderer/graphql/generated/graphql'
import VoiceModeChatView from './voice/ChatVoiceModeView'
import { useVoiceStore } from '@renderer/lib/stores/voice'
import { useChat } from '@renderer/contexts/ChatContext'
import HolonThreadContext from '../holon/HolonThreadContext'
import useDependencyStatus from '@renderer/hooks/useDependencyStatus'
import { Tooltip, TooltipContent, TooltipTrigger } from '../ui/tooltip'
import { Switch } from '../ui/switch'
import { Button } from '../ui/button'
import { ArrowDown, Eye, EyeClosed } from 'lucide-react'

interface ChatViewProps {
  chat: Chat
}

export default function ChatView({ chat }: ChatViewProps) {
  const containerRef = useRef<HTMLDivElement | null>(null)
  const bottomRef = useRef<HTMLDivElement | null>(null)
  const { isVoiceMode, stopVoiceMode, startVoiceMode } = useVoiceStore()
  const [mounted, setMounted] = useState(false)
  const [isAtBottom, setIsAtBottom] = useState(true)
  const [showScrollToBottom, setShowScrollToBottom] = useState(false)
  const [isAnonymized, setIsAnonymized] = useState(false)

  const {
    privacyDict,
    messages,
    isWaitingTwinResponse,
    isReasonSelected,
    error,
    activeToolCalls,
    historicToolCalls,
    sendMessage,
    setIsWaitingTwinResponse,
    setIsReasonSelected
  } = useChat()

  const onScroll = (e: React.UIEvent<HTMLDivElement>) => {
    const el = e.currentTarget
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 20
    setIsAtBottom(atBottom)
    setShowScrollToBottom(!atBottom)
  }

  useEffect(() => {
    const container = containerRef.current
    if (!container) return

    const scrollOptions = { top: container.scrollHeight }
    if (!mounted) {
      container.scrollTo({ ...scrollOptions, behavior: 'instant' })
      setMounted(true)
    } else if (isAtBottom) {
      container.scrollTo({ ...scrollOptions, behavior: 'smooth' })
    }
  }, [messages, mounted, isAtBottom])

  const scrollToBottom = () => {
    if (containerRef.current) {
      containerRef.current.scrollTo({
        top: containerRef.current.scrollHeight,
        behavior: 'smooth'
      })
    }
  }

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
        chatPrivacyDict={privacyDict}
        isAnonymized={isAnonymized}
        setIsAnonymized={setIsAnonymized}
      />
    )
  }

  const hasUserMessages = messages.some((msg) => msg.role === 'USER')
  const showAnonymizationToggle = hasUserMessages && privacyDict

  return (
    <div className="flex flex-col h-full w-full items-center relative">
      <div
        ref={containerRef}
        onScroll={onScroll}
        className="flex flex-1 flex-col w-full overflow-y-auto"
      >
        <div className="flex w-full justify-center">
          <div className="flex flex-col max-w-4xl items-center p-4 w-full">
            <div className="w-full flex flex-col gap-2">
              {chat.category === ChatCategory.Holon && chat.holonThreadId && (
                <HolonThreadContext threadId={chat.holonThreadId} />
              )}
              {showAnonymizationToggle && (
                <div className="absolute top-0 right-12 flex justify-end mb-2">
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button
                        onClick={() => setIsAnonymized(!isAnonymized)}
                        className="p-2 rounded-md bg-accent cursor-pointer hover:bg-accent/50"
                        variant="ghost"
                        size="sm"
                      >
                        {isAnonymized ? (
                          <EyeClosed className="h-4 w-4 text-primary" />
                        ) : (
                          <Eye className="h-4 w-4 text-primary" />
                        )}
                        <span className="ml-2 text-sm">
                          {isAnonymized ? 'Show original' : 'Show anonymized'}
                        </span>
                      </Button>
                    </TooltipTrigger>
                    <TooltipContent>
                      <p>{isAnonymized ? 'Show original messages' : 'Anonymize messages'}</p>
                    </TooltipContent>
                  </Tooltip>
                </div>
              )}
              <MessageList
                messages={messages}
                isWaitingTwinResponse={isWaitingTwinResponse}
                chatPrivacyDict={privacyDict}
                isAnonymized={isAnonymized}
              />
              {error && (
                <div className="py-2 px-4 mt-2 rounded-md border border-red-500 bg-red-500/10 text-red-500">
                  Error: {error}
                </div>
              )}
              <div ref={bottomRef} className="h-8" />
            </div>
          </div>
        </div>
      </div>

      {/* Scroll to bottom button */}
      {showScrollToBottom && (
        <div className="absolute bottom-30 left-1/2 transform -translate-x-1/2 z-10">
          <Button onClick={scrollToBottom} size="sm" className="rounded-full p-2" variant="outline">
            <ArrowDown className="w-4 h-4" />
          </Button>
        </div>
      )}

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
