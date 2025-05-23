import { useCallback, useEffect, useState, useRef } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { ChevronRight, Send } from 'lucide-react'
import { cn } from '../lib/utils'
import { useMutation, useQuery } from '@apollo/client'
import {
  CreateChatDocument,
  GetChatsDocument,
  SendMessageDocument
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
  const [selectedIndex, setSelectedIndex] = useState(0)
  const [debouncedQuery, setDebouncedQuery] = useState('')
  const debounceTimeout = useRef<NodeJS.Timeout | null>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const navigate = useNavigate()
  const router = useRouter()
  const { isVoiceMode } = useVoiceStore()
  const [createChat] = useMutation(CreateChatDocument)
  const [sendMessage] = useMutation(SendMessageDocument)
  const { data: chatsData } = useQuery(GetChatsDocument, {
    variables: { first: 20, offset: 0 }
  })

  const { isCompleted } = useOnboardingStore()

  const { isOpen, query, setQuery, closeOmnibar } = useOmnibarStore()

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
        variables: { name: query, voice: isVoiceMode }
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

        sendMessage({
          variables: {
            chatId: newChatId,
            text: query,
            voice: isVoiceMode,
            reasoning: false
          }
        })
      }
    } catch (error) {
      console.error('Failed to create chat:', error)
    } finally {
      closeOmnibar()
    }
  }, [query, navigate, router, createChat, isVoiceMode, sendMessage, closeOmnibar])

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (query.trim()) {
      if (filteredChats.length > 0 && selectedIndex < filteredChats.length) {
        navigate({ to: `/chat/${filteredChats[selectedIndex].id}` })
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
      if (isOpen) {
        if (e.key === 'ArrowDown') {
          e.preventDefault()
          setSelectedIndex((prev) => Math.min(prev + 1, filteredChats.length))
        }
        if (e.key === 'ArrowUp') {
          e.preventDefault()
          setSelectedIndex((prev) => Math.max(prev - 1, 0))
        }
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [isOpen, selectedIndex, filteredChats, navigate, closeOmnibar, isCompleted])

  return (
    <FocusLock disabled={!isOpen} returnFocus>
      <AnimatePresence mode="sync">
        {isOpen && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ type: 'spring', damping: 25, stiffness: 280 }}
            className="fixed inset-0 z-50 flex items-center justify-center bg-card pointer-events-auto"
            onClick={closeOmnibar}
          >
            <motion.div
              initial={{ scale: 0.95, opacity: 0 }}
              animate={{ scale: 1, opacity: 1 }}
              exit={{ scale: 0.95, opacity: 0 }}
              transition={{ type: 'spring', damping: 25, stiffness: 280 }}
              className="w-full max-w-xl px-4"
              onClick={(e) => e.stopPropagation()}
            >
              <motion.form onSubmit={handleSubmit}>
                <motion.div
                  layoutId="message-input-container"
                  className={cn(
                    'flex flex-col gap-3 rounded-lg border border-border bg-card p-4 shadow-2xl',
                    'focus-within:border-primary focus-within:ring-2 focus-within:ring-primary',
                    'max-h-[50vh]'
                  )}
                  transition={{ type: 'spring', stiffness: 300, damping: 30 }}
                >
                  <div className="flex items-start gap-3">
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
                      className="flex-1 !text-base bg-transparent text-foreground placeholder-muted-foreground outline-none resize-none overflow-y-hidden min-h-0 border-0 shadow-none focus-visible:ring-0 focus-visible:ring-offset-0 p-0"
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
                                selectedIndex === index && 'bg-primary/10 text-primary rounded-md'
                              )}
                              layoutId={`chat-${chat.id}`}
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
          </motion.div>
        )}
      </AnimatePresence>
    </FocusLock>
  )
}
