// <reference path="../../../preload/index.d.ts" />
import { createFileRoute } from '@tanstack/react-router'
import { useEffect, useRef, useState, useCallback } from 'react'
import { motion, AnimatePresence, useMotionValue, animate, LayoutGroup } from 'framer-motion'
import { ChevronRight, Send } from 'lucide-react'
import { cn } from '../lib/utils'
import { useMutation, useQuery } from '@apollo/client'
import {
  ChatCategory,
  CreateChatDocument,
  GetChatsDocument
} from '@renderer/graphql/generated/graphql'
import { client } from '@renderer/graphql/lib'
import FocusLock from 'react-focus-lock'
import { SendButton } from '../components/chat/MessageInput'
import { useVoiceStore } from '@renderer/lib/stores/voice'
import { ThemeProvider } from '@renderer/lib/theme'

function OmnibarOverlay() {
  const [query, setQuery] = useState('')
  const [selectedIndex, setSelectedIndex] = useState(0)
  const [debouncedQuery, setDebouncedQuery] = useState('')
  const debounceTimeout = useRef<NodeJS.Timeout | null>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const contentRef = useRef<HTMLDivElement>(null)
  const { isVoiceMode } = useVoiceStore()
  const [createChat] = useMutation(CreateChatDocument)
  const { data: chatsData } = useQuery(GetChatsDocument, {
    variables: { first: 20, offset: 0 },
    errorPolicy: 'ignore', // Don't throw on backend errors
    fetchPolicy: 'cache-first', // Use cache if available
    skip: false // Always try to fetch, even if not authenticated
  })

  console.log('chatsData', chatsData)

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

  const windowHeight = useMotionValue(64)
  const containerHeight = useMotionValue(64)
  const currentHeight = useRef(64)
  const previousResultCount = useRef(0)

  useEffect(() => {
    if (textareaRef.current) {
      textareaRef.current.focus()
    }
  }, [])

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
        // Open the main window with the new chat
        window.api.openMainWindowWithChat?.(newChatId, query)

        // Refetch all chats
        client.cache.evict({ fieldName: 'getChats' })
      }
    } catch (error) {
      console.error('Failed to create chat:', error)
    } finally {
      // Clear query and close overlay
      setQuery('')
      window.api.hideOmnibarWindow?.()
    }
  }, [query, createChat, isVoiceMode])

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (query.trim()) {
      if (filteredChats.length > 0 && selectedIndex < filteredChats.length) {
        const chatId = filteredChats[selectedIndex].id
        window.api.openMainWindowWithChat?.(chatId, query)
        // Clear query and close overlay
        setQuery('')
        window.api.hideOmnibarWindow?.()
      } else {
        handleCreateChat()
      }
    }
  }

  const handleOpenChat = (chatId: string) => {
    window.api.openMainWindowWithChat?.(chatId, query)
    // Clear query and close overlay
    setQuery('')
    window.api.hideOmnibarWindow?.()
  }

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        window.api.hideOmnibarWindow?.()
      }
      if (e.key === 'ArrowDown') {
        e.preventDefault()
        setSelectedIndex((prev) => Math.min(prev + 1, filteredChats.length))
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault()
        setSelectedIndex((prev) => Math.max(prev - 1, 0))
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [selectedIndex, filteredChats])

  // Calculate and animate window height based on state
  useEffect(() => {
    const windowWidth = 500
    const baseHeight = 64
    const itemHeight = 48 // Height per result item

    // Calculate current result count
    const resultCount =
      debouncedQuery.trim() && filteredChats.length > 0
        ? Math.min(filteredChats.length + 1, 8) // +1 for "new chat", cap at 8
        : 0

    // Only animate if result count changed
    if (resultCount === previousResultCount.current) return
    previousResultCount.current = resultCount

    const targetHeight = baseHeight + resultCount * itemHeight

    if (Math.abs(targetHeight - currentHeight.current) < 1) return

    // Animate both window and container height together
    animate(windowHeight, targetHeight, {
      type: 'spring',
      stiffness: 350,
      damping: 55,
      onUpdate: (latest) => {
        containerHeight.set(latest)
        if (window.api?.resizeOmnibarWindow) {
          ;(
            window.api.resizeOmnibarWindow as unknown as (
              width: number,
              height: number
            ) => Promise<{ success: boolean; error?: string }>
          )(windowWidth, Math.ceil(latest)).catch(() => {})
        }
      }
    })

    currentHeight.current = targetHeight
  }, [debouncedQuery, filteredChats.length, windowHeight, containerHeight])

  // This is the overlay window - just the omnibar component without any chrome
  return (
    <ThemeProvider>
      <FocusLock returnFocus>
        <motion.div
          initial={{ scale: 0.95, opacity: 0, y: -5 }}
          animate={{ scale: 1, opacity: 1, y: 0 }}
          transition={{ type: 'spring', damping: 55, stiffness: 350 }}
          className="w-full h-full !bg-transparent border-0 shadow-none"
          style={{ WebkitAppRegion: 'drag' } as React.CSSProperties}
        >
          <motion.form onSubmit={handleSubmit} className="w-full">
            <motion.div
              ref={contentRef}
              data-omnibar-content
              className={cn('flex flex-col gap-3 p-4 justify-center w-full  mx-auto')}
              transition={{ type: 'spring', damping: 55, stiffness: 350 }}
              style={{ maxHeight: containerHeight }}
            >
              <div className="flex justify-center items-center gap-3 h-full min-h-10">
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
                <AnimatePresence>
                  {debouncedQuery.trim() && filteredChats.length === 0 && (
                    <motion.div
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
                  <motion.div
                    key="results"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.2 }}
                    className="rounded-lg overflow-hidden"
                    style={{ WebkitAppRegion: 'no-drag' } as React.CSSProperties}
                  >
                    <LayoutGroup>
                      <div className="py-1">
                        {filteredChats.map((chat, index) => (
                          <motion.button
                            key={chat.id}
                            type="button"
                            style={{ WebkitAppRegion: 'no-drag' } as React.CSSProperties}
                            onClick={() => handleOpenChat(chat.id)}
                            className={cn(
                              'flex w-full items-center justify-between px-3 py-2 text-left text-sm text-muted-foreground transition-colors rounded-md duration-100',
                              'hover:bg-sidebar-accent',
                              selectedIndex === index &&
                                'bg-sidebar-accent text-sidebar-primary rounded-md'
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
                            className={cn(
                              'flex text-muted-foreground w-full items-center justify-between px-3 py-2 text-left text-sm',
                              'hover:bg-sidebar-accent',
                              selectedIndex === filteredChats.length &&
                                'bg-sidebar-accent text-sidebar-primary-foreground rounded-md'
                            )}
                          >
                            <span>New chat: &quot;{debouncedQuery}&quot;</span>
                            <Send className="h-4 w-4 text-muted-foreground" />
                          </button>
                        )}
                      </div>
                    </LayoutGroup>
                  </motion.div>
                )}
              </AnimatePresence>
            </motion.div>
          </motion.form>
        </motion.div>
      </FocusLock>
    </ThemeProvider>
  )
}

export const Route = createFileRoute('/omnibar-overlay')({
  component: OmnibarOverlay
})
