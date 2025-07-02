// <reference path="../../../preload/index.d.ts" />
import { createFileRoute } from '@tanstack/react-router'
import { useEffect, useRef, useState, useCallback } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
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
import { Textarea } from '@renderer/components/ui/textarea'
import { SendButton } from '../components/chat/MessageInput'
import { useVoiceStore } from '@renderer/lib/stores/voice'

function OmnibarOverlay() {
  const [query, setQuery] = useState('')
  const [selectedIndex, setSelectedIndex] = useState(0)
  const [debouncedQuery, setDebouncedQuery] = useState('')
  const debounceTimeout = useRef<NodeJS.Timeout | null>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const { isVoiceMode } = useVoiceStore()
  const [createChat] = useMutation(CreateChatDocument)
  const { data: chatsData } = useQuery(GetChatsDocument, {
    variables: { first: 20, offset: 0 },
    errorPolicy: 'ignore', // Don't throw on backend errors
    fetchPolicy: 'cache-first' // Use cache if available
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

  const adjustTextareaHeight = () => {
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto'
      const scrollHeight = textareaRef.current.scrollHeight
      const maxHeight = 240
      textareaRef.current.style.height = `${Math.min(scrollHeight, maxHeight)}px`
    }
  }

  const resizeWindowToContent = useCallback(() => {
    // Get the content container dimensions
    const contentContainer = document.querySelector('[data-omnibar-content]') as HTMLElement
    if (contentContainer && window.api?.resizeOmnibarWindow) {
      const rect = contentContainer.getBoundingClientRect()
      const padding = 16 // 8px padding on each side from the wrapper

      // Only resize height, keep width fixed to prevent expansion
      const validHeight = rect.height > 0 ? rect.height : 80

      const windowWidth = 500 // Fixed width
      const windowHeight = Math.max(80, Math.min(500, validHeight + padding))

      if (
        window.api?.resizeOmnibarWindow &&
        typeof windowWidth === 'number' &&
        typeof windowHeight === 'number' &&
        !isNaN(windowWidth) &&
        !isNaN(windowHeight) &&
        isFinite(windowWidth) &&
        isFinite(windowHeight)
      ) {
        ;(
          window.api.resizeOmnibarWindow as unknown as (
            width: number,
            height: number
          ) => Promise<{ success: boolean; error?: string }>
        )(windowWidth, windowHeight).catch(() => {
          // Silently handle resize errors (window might not be resizable)
        })
      }
    }
  }, [])

  useEffect(() => {
    adjustTextareaHeight()
    // Resize window after a short delay to allow for layout changes
    const timeoutId = setTimeout(resizeWindowToContent, 100)
    return () => clearTimeout(timeoutId)
  }, [query, resizeWindowToContent])

  useEffect(() => {
    // Resize window when search results change
    const timeoutId = setTimeout(resizeWindowToContent, 100)
    return () => clearTimeout(timeoutId)
  }, [filteredChats.length, debouncedQuery, resizeWindowToContent])

  useEffect(() => {
    if (textareaRef.current) {
      textareaRef.current.focus()
    }
    // Initial resize after component mounts
    setTimeout(resizeWindowToContent, 200)

    // Set up ResizeObserver for responsive resizing
    const contentContainer = document.querySelector('[data-omnibar-content]') as HTMLElement
    if (contentContainer && typeof ResizeObserver !== 'undefined') {
      const resizeObserver = new ResizeObserver(() => {
        resizeWindowToContent()
      })
      resizeObserver.observe(contentContainer)

      return () => {
        resizeObserver.disconnect()
      }
    }
    return undefined
  }, [resizeWindowToContent])

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

  // This is the overlay window - just the omnibar component without any chrome
  return (
    <FocusLock returnFocus>
      <motion.div
        initial={{ scale: 0.95, opacity: 0, y: -5 }}
        animate={{ scale: 1, opacity: 1, y: 0 }}
        transition={{ type: 'spring', damping: 25, stiffness: 280 }}
        className="w-full h-full"
        style={{ WebkitAppRegion: 'drag' } as React.CSSProperties}
      >
        <motion.form onSubmit={handleSubmit}>
          <motion.div data-omnibar-content className={cn('flex flex-col gap-3 p-4 min-w-[500px]')}>
            <div className="flex items-center gap-3">
              <Textarea
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
                className="flex-1 flex justify-center items-center h-full min-h-10 !text-base !rounded-none transparent text-foreground placeholder-muted-foreground outline-none resize-none overflow-y-hidden  border-0 shadow-none focus-visible:ring-0 focus-visible:ring-offset-0 p-0"
                style={{ WebkitAppRegion: 'no-drag' } as React.CSSProperties}
                rows={1}
              />
              <AnimatePresence mode="wait">
                {debouncedQuery.trim() && filteredChats.length === 0 && (
                  <motion.div
                    layout="position"
                    className="self-center"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    style={{ WebkitAppRegion: 'no-drag' } as React.CSSProperties}
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

            <AnimatePresence mode="wait" onExitComplete={resizeWindowToContent}>
              {debouncedQuery && filteredChats.length > 0 && (
                <motion.div
                  key="results"
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  exit={{ opacity: 0 }}
                  transition={{
                    duration: 0.2,
                    ease: 'easeInOut'
                  }}
                  onAnimationComplete={resizeWindowToContent}
                  className="rounded-lg overflow-hidden"
                  style={{ WebkitAppRegion: 'no-drag' } as React.CSSProperties}
                >
                  <div className="py-1">
                    {filteredChats.map((chat, index) => (
                      <motion.button
                        key={chat.id}
                        type="button"
                        onClick={() => handleOpenChat(chat.id)}
                        className={cn(
                          'flex w-full items-center justify-between px-3 py-2 text-left text-sm',
                          'hover:bg-muted/80',
                          selectedIndex === index && 'bg-primary/10 text-primary rounded-md'
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
                          'flex w-full items-center justify-between px-3 py-2 text-left text-sm',
                          'hover:bg-muted/80',
                          selectedIndex === filteredChats.length &&
                            'bg-primary/10 text-primary rounded-md'
                        )}
                      >
                        <span>New chat: &quot;{debouncedQuery}&quot;</span>
                        <Send className="h-4 w-4 text-muted-foreground" />
                      </button>
                    )}
                  </div>
                </motion.div>
              )}
            </AnimatePresence>
          </motion.div>
        </motion.form>
      </motion.div>
    </FocusLock>
  )
}

export const Route = createFileRoute('/omnibar-overlay')({
  component: OmnibarOverlay
})
