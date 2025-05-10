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

export const Omnibar = () => {
  const [selectedIndex, setSelectedIndex] = useState(0)
  const [debouncedQuery, setDebouncedQuery] = useState('')
  const debounceTimeout = useRef<NodeJS.Timeout | null>(null)
  const navigate = useNavigate()
  const router = useRouter()
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

  const handleCreateChat = useCallback(async () => {
    if (!query.trim()) return

    try {
      const { data: createData } = await createChat({
        variables: { name: query }
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

        sendMessage({ variables: { chatId: newChatId, text: query } })
      }
    } catch (error) {
      console.error('Failed to create chat:', error)
    } finally {
      closeOmnibar()
    }
  }, [query, navigate, router, createChat, sendMessage, closeOmnibar])

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
            className="fixed inset-0 z-50 flex items-center justify-center bg-card/50 backdrop-blur-sm pointer-events-auto"
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
                    'focus-within:border-primary focus-within:ring-2 focus-within:ring-primary'
                  )}
                  transition={{
                    layout: { type: 'spring', damping: 25, stiffness: 280 }
                  }}
                >
                  <div className="flex items-center gap-3">
                    <input
                      type="text"
                      value={query}
                      onChange={(e) => {
                        setQuery(e.target.value)
                        setSelectedIndex(0)
                      }}
                      placeholder="What would you like to discuss?"
                      className="flex-1 bg-transparent text-foreground placeholder-muted-foreground outline-none"
                      autoFocus
                    />
                    {debouncedQuery.trim() && filteredChats.length === 0 && (
                      <button
                        type="button"
                        onClick={handleCreateChat}
                        className="rounded-full p-1 text-primary hover:bg-muted"
                      >
                        <Send className="h-5 w-5" />
                      </button>
                    )}
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
