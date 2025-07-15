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
import { SyncedThemeProvider } from '@renderer/components/SyncedThemeProvider'

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

  useEffect(() => {
    if (textareaRef.current) {
      textareaRef.current.focus()
    }
  }, [])

  // Auto-resize textarea fallback for browsers without field-sizing support
  useEffect(() => {
    if (!textareaRef.current) return

    const textarea = textareaRef.current

    // Check if field-sizing is supported
    const supportsFieldSizing = CSS.supports('field-sizing', 'content')

    if (!supportsFieldSizing) {
      // Manual auto-resize for browsers without field-sizing
      const adjustHeight = () => {
        textarea.style.height = 'auto'
        textarea.style.height = `${textarea.scrollHeight}px`
      }

      // Initial adjustment
      adjustHeight()

      // Adjust on input
      textarea.addEventListener('input', adjustHeight)

      return () => {
        textarea.removeEventListener('input', adjustHeight)
      }
    }
  }, [])

  // Use ResizeObserver to detect textarea size changes and resize window
  useEffect(() => {
    if (!textareaRef.current) return

    const resizeObserver = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const height = entry.borderBoxSize?.[0]?.blockSize || entry.contentRect.height

        // Calculate window height based on textarea + padding + results
        const padding = 36 // 20px top (pt-5) + 16px bottom (pb-4)
        const itemHeight = 36
        const gapHeight = 12

        const resultCount =
          debouncedQuery.trim() && filteredChats.length > 0
            ? Math.min(filteredChats.length + 1, 8)
            : 0

        const resultsHeight = resultCount > 0 ? gapHeight + resultCount * itemHeight : 0
        const targetHeight = padding + height + resultsHeight

        // Animate window height for smooth resize
        animate(windowHeight, targetHeight, {
          type: 'spring',
          stiffness: 500,
          damping: 35,
          mass: 0.5,
          onUpdate: (latest) => {
            // Resize the actual window
            if (window.api?.resizeOmnibarWindow) {
              ;(
                window.api.resizeOmnibarWindow as unknown as (
                  width: number,
                  height: number
                ) => Promise<{ success: boolean; error?: string }>
              )(500, Math.ceil(latest)).catch(() => {})
            }
          },
          onComplete: () => {
            // Ensure final values match
            windowHeight.set(targetHeight)
          }
        })
      }
    })

    resizeObserver.observe(textareaRef.current)

    return () => {
      resizeObserver.disconnect()
    }
  }, [debouncedQuery, filteredChats.length, windowHeight])

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
        window.api.openMainWindowWithChat?.(newChatId, query, true)

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
        const showNewChat = debouncedQuery.trim() !== ''
        const maxIndex = Math.max(0, showNewChat ? filteredChats.length : filteredChats.length - 1)
        setSelectedIndex((prev) => Math.min(prev + 1, maxIndex))
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault()
        setSelectedIndex((prev) => Math.max(prev - 1, 0))
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [filteredChats, debouncedQuery])

  // This is the overlay window - just the omnibar component without any chrome
  return (
    <>
      <style>{`
        .auto-sizing-textarea {
          field-sizing: content;
          min-height: 1.5rem;
          max-height: 15rem;
          display: block;
        }
        
        @supports not (field-sizing: content) {
          /* Fallback for browsers that don't support field-sizing */
          .auto-sizing-textarea {
            min-height: 1.5rem;
            max-height: 15rem;
          }
        }
        
        textarea::-webkit-scrollbar {
          width: 4px;
        }
        textarea::-webkit-scrollbar-track {
          background: transparent;
        }
        textarea::-webkit-scrollbar-thumb {
          background-color: rgba(155, 155, 155, 0.5);
          border-radius: 2px;
        }
        textarea::-webkit-scrollbar-thumb:hover {
          background-color: rgba(155, 155, 155, 0.7);
        }
      `}</style>
      <SyncedThemeProvider>
        <FocusLock returnFocus>
          <motion.div
            initial={{ scale: 0.95, opacity: 0, y: -5 }}
            animate={{ scale: 1, opacity: 1, y: 0 }}
            transition={{ type: 'spring', damping: 55, stiffness: 350 }}
            className="w-full h-full !bg-transparent border-0 shadow-none overflow-visible"
            style={{ WebkitAppRegion: 'drag' } as React.CSSProperties}
          >
            <motion.form onSubmit={handleSubmit} className="w-full h-full overflow-visible">
              <motion.div
                ref={contentRef}
                data-omnibar-content
                className={cn('flex flex-col gap-3 px-4 pt-3.5 pb-4 w-full mx-auto')}
                transition={{ type: 'spring', damping: 55, stiffness: 350 }}
              >
                <div className="flex items-center gap-3">
                  <div className="flex-1 flex items-center">
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
                      className="w-full !bg-transparent overflow-y-auto !text-base !rounded-none text-foreground placeholder-muted-foreground outline-none resize-none border-0 shadow-none focus-visible:ring-0 focus-visible:ring-offset-0 p-0 leading-normal auto-sizing-textarea"
                      style={
                        {
                          lineHeight: '1.5rem',
                          // @ts-ignore - WebkitAppRegion is a valid CSS property for Electron
                          WebkitAppRegion: 'no-drag'
                        } as React.CSSProperties
                      }
                      rows={1}
                    />
                  </div>

                  <motion.div
                    layout="position"
                    className="flex self-end pb-0.5"
                    initial={{ opacity: 0 }}
                    animate={{
                      opacity: debouncedQuery.trim() && filteredChats.length === 0 ? 1 : 0,
                      scale: debouncedQuery.trim() && filteredChats.length === 0 ? 1 : 0.8
                    }}
                    exit={{ opacity: 0 }}
                    transition={{
                      type: 'spring',
                      stiffness: 400,
                      damping: 30
                    }}
                  >
                    <SendButton
                      onSend={handleCreateChat}
                      isWaitingTwinResponse={false}
                      text={query}
                    />
                  </motion.div>
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
          </motion.div>
        </FocusLock>
      </SyncedThemeProvider>
    </>
  )
}

export const Route = createFileRoute('/omnibar-overlay')({
  component: OmnibarOverlay
})
