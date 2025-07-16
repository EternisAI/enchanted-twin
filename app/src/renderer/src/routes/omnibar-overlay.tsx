// <reference path="../../../preload/index.d.ts" />
import { createFileRoute } from '@tanstack/react-router'
import { useEffect, useRef, useState, useCallback } from 'react'
import { motion, AnimatePresence, useMotionValue, animate, LayoutGroup } from 'framer-motion'
import { ChevronRight, Send, Maximize2, ArrowLeft, ArrowDown } from 'lucide-react'
import { cn } from '../lib/utils'
import { useMutation, useQuery } from '@apollo/client'
import {
  ChatCategory,
  CreateChatDocument,
  GetChatsDocument,
  GetChatDocument
} from '@renderer/graphql/generated/graphql'
import { client } from '@renderer/graphql/lib'
import FocusLock from 'react-focus-lock'
import { SendButton } from '../components/chat/MessageInput'
import { useVoiceStore } from '@renderer/lib/stores/voice'
import { SyncedThemeProvider } from '@renderer/components/SyncedThemeProvider'
import { Button } from '../components/ui/button'
import MessageList from '../components/chat/MessageList'
import MessageInput from '../components/chat/MessageInput'
import { ChatProvider } from '@renderer/contexts/ChatContext'
import { useChat } from '@renderer/contexts/ChatContext'
import { Tooltip, TooltipContent, TooltipTrigger, TooltipProvider } from '../components/ui/tooltip'

function OmnibarChatView({ chatId }: { chatId: string }) {
  const containerRef = useRef<HTMLDivElement | null>(null)
  const [isAtBottom, setIsAtBottom] = useState(true)
  const [showScrollToBottom, setShowScrollToBottom] = useState(false)

  const {
    privacyDict,
    messages,
    isWaitingTwinResponse,
    isReasonSelected,
    error,
    sendMessage,
    setIsWaitingTwinResponse,
    setIsReasonSelected
  } = useChat()

  const { data: chatData } = useQuery(GetChatDocument, {
    variables: { id: chatId },
    skip: !chatId
  })

  const chat = chatData?.getChat

  const onScroll = (e: React.UIEvent<HTMLDivElement>) => {
    const el = e.currentTarget
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 20
    setIsAtBottom(atBottom)
    setShowScrollToBottom(!atBottom)
  }

  useEffect(() => {
    const container = containerRef.current
    if (!container) return

    if (isAtBottom) {
      container.scrollTo({
        top: container.scrollHeight,
        behavior: 'smooth'
      })
    }
  }, [messages, isAtBottom])

  const scrollToBottom = () => {
    if (containerRef.current) {
      containerRef.current.scrollTo({
        top: containerRef.current.scrollHeight,
        behavior: 'smooth'
      })
    }
  }

  if (!chat) return null

  return (
    <div className="flex flex-col h-full overflow-hidden">
      <h3 className="text-sm font-medium truncate px-2 pb-3">{chat.name}</h3>

      <div className="flex-1 flex flex-col min-h-0 gap-2">
        <div className="flex-1 relative overflow-hidden">
          <div
            ref={containerRef}
            onScroll={onScroll}
            className="absolute inset-0 overflow-y-auto px-1"
            style={{ WebkitAppRegion: 'no-drag' } as React.CSSProperties}
          >
            <MessageList
              messages={messages}
              isWaitingTwinResponse={isWaitingTwinResponse}
              chatPrivacyDict={privacyDict}
            />
            {error && (
              <div className="py-2 px-4 mt-2 rounded-md border border-red-500 bg-red-500/10 text-red-500">
                Error: {error}
              </div>
            )}
            <div className="h-4" />
          </div>

          {showScrollToBottom && (
            <div className="absolute bottom-2 left-1/2 transform -translate-x-1/2 z-10 pointer-events-none">
              <Button
                onClick={scrollToBottom}
                size="sm"
                className="rounded-full p-2 pointer-events-auto"
                variant="outline"
              >
                <ArrowDown className="w-4 h-4" />
              </Button>
            </div>
          )}
        </div>

        <div className="flex-shrink-0 max-h-[180px] overflow-hidden">
          <MessageInput
            isWaitingTwinResponse={isWaitingTwinResponse}
            onSend={sendMessage}
            onStop={() => setIsWaitingTwinResponse(false)}
            isReasonSelected={isReasonSelected}
            onReasonToggle={setIsReasonSelected}
          />
        </div>
      </div>
    </div>
  )
}

function OmnibarResults({
  debouncedQuery,
  filteredChats,
  selectedIndex,
  handleOpenChat,
  handleCreateChat
}: {
  debouncedQuery: string
  filteredChats: { id: string; name: string }[]
  selectedIndex: number
  handleOpenChat: (chatId: string) => void
  handleCreateChat: () => void
}) {
  return (
    <motion.div
      key="results"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      transition={{ duration: 0.2 }}
      className="rounded-lg overflow-auto max-h-[280px] relative"
      style={{ WebkitAppRegion: 'no-drag' } as React.CSSProperties}
      role="listbox"
      aria-label="Chat search results"
    >
      <LayoutGroup>
        <div className="py-1">
          {filteredChats.map((chat, index) => (
            <motion.button
              key={chat.id}
              type="button"
              style={{ WebkitAppRegion: 'no-drag' } as React.CSSProperties}
              onClick={() => handleOpenChat(chat.id)}
              role="option"
              aria-selected={selectedIndex === index}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  e.preventDefault()
                  handleOpenChat(chat.id)
                }
              }}
              className={cn(
                'flex w-full items-center justify-between px-3 py-2 text-left text-sm text-muted-foreground transition-colors rounded-md duration-100',
                'hover:bg-sidebar-accent',
                selectedIndex === index && 'bg-sidebar-accent text-sidebar-primary rounded-md'
              )}
            >
              <span className="truncate">{chat.name}</span>
              <ChevronRight className="h-4 w-4 text-muted-foreground" />
            </motion.button>
          ))}
          {debouncedQuery.trim() && (
            <button
              type="button"
              onClick={handleCreateChat}
              role="option"
              aria-selected={selectedIndex === filteredChats.length}
              className={cn(
                'flex text-muted-foreground w-full items-center justify-between px-3 py-2 text-left text-sm',
                'hover:bg-sidebar-accent',
                selectedIndex === filteredChats.length && 'bg-sidebar-accent rounded-md font-medium'
              )}
            >
              <span>New chat: &quot;{debouncedQuery}&quot;</span>
              <Send className="h-4 w-4 text-muted-foreground" />
            </button>
          )}
        </div>
      </LayoutGroup>
    </motion.div>
  )
}

function OmnibarOverlay() {
  const [query, setQuery] = useState('')
  const [selectedIndex, setSelectedIndex] = useState(0)
  const [debouncedQuery, setDebouncedQuery] = useState('')
  const [activeChatId, setActiveChatId] = useState<string | null>(null)
  const debounceTimeout = useRef<NodeJS.Timeout | null>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const contentRef = useRef<HTMLDivElement>(null)
  const { isVoiceMode } = useVoiceStore()
  const [createChat] = useMutation(CreateChatDocument)
  const { data: chatsData } = useQuery(GetChatsDocument, {
    variables: { first: 20, offset: 0 },
    errorPolicy: 'ignore',
    fetchPolicy: 'cache-first',
    skip: false
  })

  const chats = chatsData?.getChats || []
  const filteredChats = chats.filter((chat) =>
    chat.name.toLowerCase().includes(debouncedQuery.toLowerCase())
  )

  useEffect(() => {
    if (debounceTimeout.current) {
      clearTimeout(debounceTimeout.current)
    }

    debounceTimeout.current = setTimeout(() => {
      setDebouncedQuery(query)
    }, 150)
    return () => {
      if (debounceTimeout.current) {
        clearTimeout(debounceTimeout.current)
      }
    }
  }, [query])

  useEffect(() => {
    const showNewChat = debouncedQuery.trim() !== ''
    const maxIndex = Math.max(0, showNewChat ? filteredChats.length : filteredChats.length - 1)
    setSelectedIndex((prev) => Math.max(0, Math.min(prev, maxIndex)))
  }, [debouncedQuery, filteredChats.length])

  const windowHeight = useMotionValue(68)
  const windowWidth = useMotionValue(500)
  const containerHeight = useMotionValue(68)
  const currentHeight = useRef(68)
  const currentWidth = useRef(500)
  const previousResultCount = useRef(0)

  // Fetch the active chat data
  const { data: activeChatData } = useQuery(GetChatDocument, {
    variables: { id: activeChatId || '' },
    skip: !activeChatId
  })

  const activeChat = activeChatData?.getChat

  useEffect(() => {
    if (textareaRef.current && !activeChatId) {
      textareaRef.current.focus()
    }
  }, [activeChatId])

  // Calculate and animate window dimensions based on state
  useEffect(() => {
    const baseWidth = 500
    const chatWidth = 700
    const baseHeight = 68
    const itemHeight = 48
    const chatHeight = 500

    let targetWidth = baseWidth
    let targetHeight = baseHeight

    if (activeChatId) {
      targetWidth = chatWidth
      targetHeight = chatHeight
    } else {
      // Calculate current result count
      const resultCount =
        debouncedQuery.trim() && filteredChats.length > 0
          ? Math.min(filteredChats.length + 1, 8)
          : 0

      // Only animate if result count changed
      if (resultCount !== previousResultCount.current) {
        previousResultCount.current = resultCount
        targetHeight = baseHeight + resultCount * itemHeight
      } else {
        return
      }
    }

    if (
      Math.abs(targetHeight - currentHeight.current) < 1 &&
      Math.abs(targetWidth - currentWidth.current) < 1
    )
      return

    // Animate both dimensions together
    Promise.all([
      animate(windowHeight, targetHeight, {
        type: 'spring',
        stiffness: 350,
        damping: 55,
        onUpdate: (latest) => {
          containerHeight.set(latest)
        }
      }),
      animate(windowWidth, targetWidth, {
        type: 'spring',
        stiffness: 350,
        damping: 55
      })
    ]).then(() => {
      if (window.api?.resizeOmnibarWindow) {
        ;(
          window.api.resizeOmnibarWindow as unknown as (
            width: number,
            height: number
          ) => Promise<{ success: boolean; error?: string }>
        )(Math.ceil(currentWidth.current), Math.ceil(currentHeight.current)).catch(() => {})
      }
    })

    // Update window dimensions during animation
    const unsubscribeHeight = windowHeight.on('change', (latest) => {
      currentHeight.current = latest
      if (window.api?.resizeOmnibarWindow) {
        ;(
          window.api.resizeOmnibarWindow as unknown as (
            width: number,
            height: number
          ) => Promise<{ success: boolean; error?: string }>
        )(Math.ceil(currentWidth.current), Math.ceil(latest)).catch(() => {})
      }
    })

    const unsubscribeWidth = windowWidth.on('change', (latest) => {
      currentWidth.current = latest
      if (window.api?.resizeOmnibarWindow) {
        ;(
          window.api.resizeOmnibarWindow as unknown as (
            width: number,
            height: number
          ) => Promise<{ success: boolean; error?: string }>
        )(Math.ceil(latest), Math.ceil(currentHeight.current)).catch(() => {})
      }
    })

    currentHeight.current = targetHeight
    currentWidth.current = targetWidth

    return () => {
      unsubscribeHeight()
      unsubscribeWidth()
    }
  }, [
    debouncedQuery,
    filteredChats.length,
    windowHeight,
    windowWidth,
    containerHeight,
    activeChatId
  ])

  const handleCreateChat = useCallback(async () => {
    if (!query.trim()) return

    try {
      const { data: createData } = await createChat({
        variables: {
          name: query,
          category: isVoiceMode ? ChatCategory.Voice : ChatCategory.Text,
          initialMessage: query
        }
      })
      const newChatId = createData?.createChat?.id

      if (newChatId) {
        // Show chat inline
        setActiveChatId(newChatId)
        setQuery('')
        setDebouncedQuery('')

        // Refetch all chats
        client.cache.evict({ fieldName: 'getChats' })
      }
    } catch (error) {
      console.error('Failed to create chat:', error)
    }
  }, [query, createChat, isVoiceMode])

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (query.trim() && !activeChatId) {
      if (filteredChats.length > 0 && selectedIndex < filteredChats.length) {
        const chatId = filteredChats[selectedIndex].id
        // Show chat inline
        setActiveChatId(chatId)
        setQuery('')
        setDebouncedQuery('')
      } else {
        handleCreateChat()
      }
    }
  }

  const handleOpenChat = (chatId: string) => {
    // Show chat inline
    setActiveChatId(chatId)
    setQuery('')
    setDebouncedQuery('')
  }

  const handleExpandToMainWindow = () => {
    if (activeChatId) {
      window.api.openMainWindowWithChat?.(activeChatId, '')
      // Reset state and close overlay
      setActiveChatId(null)
      setQuery('')
      window.api.hideOmnibarWindow?.()
    }
  }

  const handleCloseChat = () => {
    setActiveChatId(null)
    setQuery('')
    // Focus back on input
    setTimeout(() => {
      textareaRef.current?.focus()
    }, 100)
  }

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        if (activeChatId) {
          handleCloseChat()
        } else {
          window.api.hideOmnibarWindow?.()
        }
      }
      if (e.key === 'ArrowDown' && !activeChatId) {
        e.preventDefault()
        const showNewChat = debouncedQuery.trim() !== ''
        const maxIndex = Math.max(0, showNewChat ? filteredChats.length : filteredChats.length - 1)
        setSelectedIndex((prev) => Math.min(prev + 1, maxIndex))
      }
      if (e.key === 'ArrowUp' && !activeChatId) {
        e.preventDefault()
        setSelectedIndex((prev) => Math.max(prev - 1, 0))
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [selectedIndex, filteredChats, activeChatId])

  // Prevent blur when showing chat
  useEffect(() => {
    if (activeChatId) {
      const handleBlur = (e: FocusEvent) => {
        e.preventDefault()
        e.stopPropagation()
      }
      window.addEventListener('blur', handleBlur, true)
      return () => window.removeEventListener('blur', handleBlur, true)
    }
  }, [activeChatId])

  // This is the overlay window - just the omnibar component without any chrome
  return (
    <SyncedThemeProvider>
      <TooltipProvider>
        <FocusLock returnFocus>
          <motion.div
            initial={{ scale: 0.95, opacity: 0, y: -5 }}
            animate={{ scale: 1, opacity: 1, y: 0 }}
            transition={{ type: 'spring', damping: 55, stiffness: 350 }}
            className="w-full h-full !bg-transparent border-0 shadow-none overflow-hidden"
            style={{ WebkitAppRegion: 'drag' } as React.CSSProperties}
          >
            {activeChatId && activeChat ? (
              <ChatProvider chat={activeChat}>
                <motion.div
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  className="flex flex-col p-4 overflow-hidden"
                  style={{ height: containerHeight }}
                >
                  <div className="flex items-center justify-between mb-3 flex-shrink-0">
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={handleCloseChat}
                          className="p-1"
                          style={{ WebkitAppRegion: 'no-drag' } as React.CSSProperties}
                        >
                          <ArrowLeft className="h-4 w-4" />
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent>
                        <p>ESC to go back</p>
                      </TooltipContent>
                    </Tooltip>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={handleExpandToMainWindow}
                      className="gap-2"
                      style={{ WebkitAppRegion: 'no-drag' } as React.CSSProperties}
                    >
                      <Maximize2 className="h-4 w-4" />
                      Expand
                    </Button>
                  </div>
                  <div className="flex-1 min-h-0 overflow-hidden">
                    <OmnibarChatView chatId={activeChatId} />
                  </div>
                </motion.div>
              </ChatProvider>
            ) : (
              <motion.form onSubmit={handleSubmit} className="w-full h-full">
                <motion.div
                  ref={contentRef}
                  data-omnibar-content
                  className={cn('flex flex-col gap-3 p-4 w-full mx-auto h-full justify-center')}
                  transition={{ type: 'spring', damping: 55, stiffness: 350 }}
                  style={{ height: containerHeight }}
                >
                  <div className={cn('flex items-center gap-3 h-full min-h-10')}>
                    <textarea
                      ref={textareaRef}
                      value={query}
                      onChange={(e) => {
                        setQuery(e.target.value)
                        setSelectedIndex(0)
                      }}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter' && !e.shiftKey) {
                          e.preventDefault()
                          handleSubmit(e as React.FormEvent)
                        }
                      }}
                      placeholder="What would you like to discuss?"
                      className="flex-1 !bg-transparent !h-full flex justify-center items-center !text-base !rounded-none transparent text-foreground placeholder-muted-foreground outline-none resize-none border-0 shadow-none focus-visible:ring-0 focus-visible:ring-offset-0 p-0"
                      style={{ WebkitAppRegion: 'no-drag' } as React.CSSProperties}
                      rows={1}
                    />
                    <AnimatePresence mode="wait">
                      {debouncedQuery.trim() && filteredChats.length === 0 && (
                        <motion.div
                          key="create-chat"
                          layout="position"
                          className="self-center"
                          initial={{ opacity: 0 }}
                          animate={{ opacity: 1 }}
                          exit={{ opacity: 0 }}
                        >
                          <SendButton
                            onSend={handleCreateChat}
                            isWaitingTwinResponse={false}
                            text={query}
                          />
                        </motion.div>
                      )}
                    </AnimatePresence>
                  </div>

                  <AnimatePresence>
                    {debouncedQuery && filteredChats.length > 0 && (
                      <OmnibarResults
                        debouncedQuery={debouncedQuery}
                        filteredChats={filteredChats}
                        selectedIndex={selectedIndex}
                        handleOpenChat={handleOpenChat}
                        handleCreateChat={handleCreateChat}
                      />
                    )}
                  </AnimatePresence>
                </motion.div>
              </motion.form>
            )}
          </motion.div>
        </FocusLock>
      </TooltipProvider>
    </SyncedThemeProvider>
  )
}

export const Route = createFileRoute('/omnibar-overlay')({
  component: OmnibarOverlay
})
