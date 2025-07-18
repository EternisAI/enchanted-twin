import { useRef, useEffect, useState } from 'react'
import { AnimatePresence, motion } from 'framer-motion'

import MessageList from './MessageList'
import MessageInput from './MessageInput'
import { Chat, ChatCategory } from '@renderer/graphql/generated/graphql'
import VoiceModeChatView from './voice/ChatVoiceModeView'
import { useVoiceStore } from '@renderer/lib/stores/voice'
import { useChat } from '@renderer/contexts/ChatContext'
import HolonThreadContext from '../holon/HolonThreadContext'
import { Button } from '../ui/button'
import { ArrowDown } from 'lucide-react'
import { Fade } from '../ui/blur-fade'
import Error from './Error'
import { AnonToggleButton } from './AnonToggleButton'

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

  // TODO: replace with intersection observer instead of scroll event listener - performance improvement
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
    <div className="flex flex-col h-full w-full items-center relative ">
      <Fade
        background="var(--color-background)"
        className="w-full h-[100px] absolute top-0 left-0 z-20 pointer-events-none"
        side="top"
        blur="24px"
        stop="10%"
      />
      <div
        ref={containerRef}
        onScroll={onScroll}
        className="flex flex-1 flex-col w-full overflow-y-auto pt-[72px] px-4"
      >
        <div className="flex w-full justify-center">
          <div className="flex flex-col max-w-3xl items-center p-4 w-full">
            <div className="w-full flex flex-col gap-2">
              {chat.category === ChatCategory.Holon && chat.holonThreadId && (
                <HolonThreadContext threadId={chat.holonThreadId} />
              )}
              {showAnonymizationToggle && (
                <AnonToggleButton isAnonymized={isAnonymized} setIsAnonymized={setIsAnonymized} />
              )}
              <MessageList
                messages={messages}
                isWaitingTwinResponse={isWaitingTwinResponse}
                chatPrivacyDict={privacyDict}
                isAnonymized={isAnonymized}
              />
              {error && <Error error={error} />}
              <div ref={bottomRef} className="h-8" />
            </div>
          </div>
        </div>
      </div>

      {/* Scroll to bottom button */}
      <AnimatePresence>
        {showScrollToBottom && (
          <motion.div
            initial={{ opacity: 0, scale: 0.8, y: 20 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.8, y: 0 }}
            transition={{
              type: 'spring',
              stiffness: 350,
              damping: 20,
              opacity: {
                duration: 0.2,
                ease: 'easeInOut'
              }
            }}
            className="absolute bottom-30 left-1/2 transform -translate-x-1/2 z-10"
          >
            <Button
              onClick={scrollToBottom}
              className="backdrop-blur-sm !bg-white shadow-sm dark:shadow-none dark:border dark:border-border dark:!bg-black/50 rounded-full p-2"
              variant="ghost"
            >
              <ArrowDown className="w-4 h-4" />
            </Button>
          </motion.div>
        )}
      </AnimatePresence>

      <div className="flex flex-col items-center justify-center px-2 absolute bottom-0 inset-x-4">
        <Fade
          background="var(--color-background)"
          className="w-full h-[180px] absolute bottom-0 left-0 z-0 pointer-events-none"
          side="bottom"
          blur="12px"
          stop="30%"
        />
        <div className="pb-4 w-full max-w-3xl flex flex-col gap-4 justify-center items-center relative z-10">
          <MessageInput
            isWaitingTwinResponse={isWaitingTwinResponse}
            onSend={sendMessage}
            onStop={() => {
              setIsWaitingTwinResponse(false)
            }}
            voiceMode={isVoiceMode}
            onVoiceModeChange={() => {
              if (isVoiceMode) {
                stopVoiceMode()
              } else {
                startVoiceMode(chat.id)
              }
            }}
            isReasonSelected={isReasonSelected}
            onReasonToggle={setIsReasonSelected}
          />
        </div>
      </div>
    </div>
  )
}
