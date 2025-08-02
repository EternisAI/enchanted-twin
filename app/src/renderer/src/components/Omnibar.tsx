import { useCallback, useEffect, useState, useRef } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { ChevronRight, Send } from 'lucide-react'
import { cn } from '../lib/utils'
import { useMutation, useQuery } from '@apollo/client'
import {
  ChatCategory,
  CreateChatDocument,
  GetChatsDocument
} from '@renderer/graphql/generated/graphql'
import { useNavigate } from '@tanstack/react-router'
import { client } from '@renderer/graphql/lib'
import { useRouter } from '@tanstack/react-router'
import { useOmnibarStore } from '@renderer/lib/stores/omnibar'
import FocusLock from 'react-focus-lock'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { Textarea } from '@renderer/components/ui/textarea'
import { SendButton } from './chat/MessageInput'
import { useVoiceStore } from '@renderer/lib/stores/voice'

export const Omnibar = () => {
  const [selectedIndex, setSelectedIndex] = useState(-1)
  const [debouncedQuery, setDebouncedQuery] = useState('')
  const debounceTimeout = useRef<NodeJS.Timeout | null>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const navigate = useNavigate()
  const router = useRouter()
  const { isVoiceMode } = useVoiceStore()
  const [createChat] = useMutation(CreateChatDocument)
  const { data: chatsData } = useQuery(GetChatsDocument, {
    variables: { first: 20, offset: 0 }
  })

  const { isCompleted } = useOnboardingStore()

  const { isOpen, query, setQuery, closeOmnibar, placeholder } = useOmnibarStore()

  const chats = chatsData?.getChats || []
  const filteredChats = chats.filter((chat) =>
    chat.name.toLowerCase().includes(debouncedQuery.toLowerCase())
  )

  useEffect(() => {
    if (debounceTimeout.current) {
      clearTimeout(debounceTimeout.current)
    }

    debounceTimeout.current = setTimeout(() => {
      setDebouncedQuery(query.trim())
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

  useEffect(() => {
    if (isOpen) {
      adjustTextareaHeight()
    }
  }, [query, isOpen])

  useEffect(() => {
    if (isOpen && textareaRef.current) {
      textareaRef.current.focus()
    }
  }, [isOpen])

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
        navigate({
          to: `/chat/${newChatId}`,
          search: { initialMessage: query }
        })

        // Refetch all chats
        await client.cache.evict({ fieldName: 'getChats' })
        await router.invalidate({
          filter: (match) => match.routeId === '/chat/$chatId'
        })
      }
    } catch (error) {
      console.error('Failed to create chat:', error)
    } finally {
      closeOmnibar()
    }
  }, [query, navigate, router, createChat, isVoiceMode, closeOmnibar])

  const handleArrowKeyDown = useCallback(
    (e: KeyboardEvent) => {
      const textarea = textareaRef.current
      if (!textarea) return

      const isAtEnd =
        textarea.selectionStart === textarea.value.length &&
        textarea.selectionEnd === textarea.value.length
      const isAtStart = textarea.selectionStart === 0 && textarea.selectionEnd === 0

      const hasSuggestions = filteredChats.length > 0
      const maxIndex = filteredChats.length

      if (e.key === 'ArrowDown' && isAtEnd && hasSuggestions) {
        e.preventDefault()
        setSelectedIndex((prev) => Math.min(prev + 1, maxIndex))
      }
      if (e.key === 'ArrowUp' && hasSuggestions) {
        if (selectedIndex === 0 && isAtStart) {
          setSelectedIndex(-1)
        } else if (selectedIndex > 0) {
          e.preventDefault()
          setSelectedIndex((prev) => prev - 1)
        }
      }
    },
    [selectedIndex, filteredChats.length]
  )

  useEffect(() => {
    if (!isOpen) return

    const textarea = textareaRef.current
    if (textarea) {
      textarea.addEventListener('keydown', handleArrowKeyDown)
      return () => textarea.removeEventListener('keydown', handleArrowKeyDown)
    }
  }, [isOpen, handleArrowKeyDown])

  useEffect(() => {
    if (!debouncedQuery || filteredChats.length === 0) {
      setSelectedIndex(-1)
    } else {
      setSelectedIndex(0)
    }
  }, [debouncedQuery, filteredChats.length])

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (query.trim()) {
      if (
        filteredChats.length > 0 &&
        selectedIndex >= 1 &&
        selectedIndex < filteredChats.length + 1
      ) {
        navigate({ to: `/chat/${filteredChats[selectedIndex - 1].id}` })
        closeOmnibar()
      } else {
        handleCreateChat()
      }
    }
  }

  useEffect(() => {
    if (!isCompleted) return
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault()
        useOmnibarStore.getState().toggleOmnibar()
      }
      if (e.key === 'Escape') {
        closeOmnibar()
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [closeOmnibar, isCompleted])

  return (
    <FocusLock disabled={!isOpen} returnFocus>
      <AnimatePresence mode="sync">
        {isOpen && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ type: 'spring', damping: 55, stiffness: 350 }}
            className="fixed inset-0 z-50 flex items-center justify-center bg-background/50 backdrop-blur-sm pointer-events-auto"
            onClick={closeOmnibar}
          >
            <motion.div
              initial={{ scale: 0.95, opacity: 0 }}
              animate={{ scale: 1, opacity: 1 }}
              exit={{ scale: 0.95, opacity: 0 }}
              transition={{ type: 'spring', damping: 55, stiffness: 350 }}
              className="w-full max-w-xl px-4"
              onClick={(e) => e.stopPropagation()}
            >
              <motion.form onSubmit={handleSubmit}>
                <motion.div
                  layoutId="message-input-container"
                  className={cn(
                    'flex flex-col gap-3 rounded-lg border border-border bg-card p-4 shadow-xl',
                    'max-h-[50vh]'
                  )}
                  transition={{ type: 'spring', stiffness: 300, damping: 30 }}
                >
                  <motion.div layout="position" className="flex items-start gap-3">
                    <Textarea
                      ref={textareaRef}
                      value={query}
                      onChange={(e) => {
                        setQuery(e.target.value)
                      }}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter' && !e.shiftKey) {
                          e.preventDefault()
                          handleSubmit(e as React.FormEvent)
                        }
                      }}
                      placeholder={placeholder}
                      className="flex-1 !rounded-none !bg-transparent text-foreground placeholder-muted-foreground outline-none resize-none overflow-y-hidden min-h-0 border-0 shadow-none focus-visible:ring-0 focus-visible:ring-offset-0 p-0"
                      rows={1}
                    />
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
                          isVoiceReady={false}
                          type="button"
                        />
                      </motion.div>
                    )}
                  </motion.div>

                  <AnimatePresence mode="wait">
                    {debouncedQuery && filteredChats.length > 0 && (
                      <motion.div
                        key="results"
                        initial={{ height: 0, opacity: 0 }}
                        animate={{ height: 'auto', opacity: 1 }}
                        exit={{ height: 0, opacity: 0 }}
                        transition={{
                          duration: 0.2,
                          ease: 'easeInOut',
                          layout: { type: 'spring', damping: 20, stiffness: 300 },
                          staggerChildren: 0.1
                        }}
                        className="rounded-lg overflow-hidden"
                      >
                        <div className="py-1">
                          <button
                            type="button"
                            onClick={handleCreateChat}
                            className={cn(
                              'flex w-full items-center justify-between px-3 py-2 text-left text-sm',
                              'hover:bg-muted/80',
                              selectedIndex === 0 && 'bg-primary/10 text-primary rounded-md'
                            )}
                          >
                            <span>New chat: &quot;{debouncedQuery}&quot;</span>
                            <Send className="h-4 w-4 text-muted-foreground" />
                          </button>
                          {filteredChats.map((chat, index) => (
                            <motion.button
                              key={chat.id}
                              type="button"
                              onClick={() => {
                                navigate({ to: `/chat/${chat.id}` })
                                closeOmnibar()
                              }}
                              className={cn(
                                'flex w-full items-center justify-between px-3 py-2 text-left text-sm',
                                'hover:bg-muted/80',
                                selectedIndex === index + 1 &&
                                  'bg-primary/10 text-primary rounded-md'
                              )}
                              layoutId={`chat-${chat.id}`}
                            >
                              <span className="truncate">{chat.name}</span>
                              <ChevronRight className="h-4 w-4 text-muted-foreground" />
                            </motion.button>
                          ))}
                        </div>
                      </motion.div>
                    )}
                  </AnimatePresence>
                </motion.div>
              </motion.form>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </FocusLock>
  )
}
