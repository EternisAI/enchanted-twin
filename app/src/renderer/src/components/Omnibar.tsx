import { useCallback, useEffect, useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { Search, ChevronRight } from 'lucide-react'
import { cn } from '../lib/utils'
import { useMutation, useQuery } from '@apollo/client'
import { CreateChatDocument, GetChatsDocument } from '@renderer/graphql/generated/graphql'
import { useNavigate } from '@tanstack/react-router'
import { client } from '@renderer/graphql/lib'
import { useRouter } from '@tanstack/react-router'

export const Omnibar = () => {
  const [isOpen, setIsOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [selectedIndex, setSelectedIndex] = useState(0)
  const navigate = useNavigate()
  const router = useRouter()
  const [createChat] = useMutation(CreateChatDocument)
  const { data: chatsData } = useQuery(GetChatsDocument, {
    variables: { first: 20, offset: 0 }
  })

  const chats = chatsData?.getChats || []
  const filteredChats = chats.filter((chat) =>
    chat.name.toLowerCase().includes(query.toLowerCase())
  )

  const handleCreateChat = useCallback(async () => {
    if (!query.trim()) return

    try {
      const { data: createData } = await createChat({
        variables: { name: query }
      })
      const newChatId = createData?.createChat?.id

      if (newChatId) {
        // Invalidate chats cache
        await client.cache.evict({ fieldName: 'getChats' })
        await router.invalidate({
          filter: (match) => match.routeId === '/chat'
        })

        // Navigate to the new chat with initial message
        navigate({
          to: `/chat/${newChatId}`,
          search: { initialMessage: query }
        })
      }
    } catch (error) {
      console.error('Failed to create chat:', error)
    } finally {
      setIsOpen(false)
      setQuery('')
    }
  }, [query, navigate, router, createChat])

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (query.trim()) {
      if (filteredChats.length > 0 && selectedIndex < filteredChats.length) {
        // Navigate to selected chat
        navigate({ to: `/chat/${filteredChats[selectedIndex].id}` })
        setIsOpen(false)
        setQuery('')
      } else {
        // Create new chat
        handleCreateChat()
      }
    }
  }

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Check for CMD+K (Mac) or CTRL+K (Windows/Linux)
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault()
        setIsOpen(true)
      }
      // Close on Escape
      if (e.key === 'Escape') {
        setIsOpen(false)
      }
      // Handle arrow keys for selection
      if (isOpen) {
        if (e.key === 'ArrowDown') {
          e.preventDefault()
          setSelectedIndex((prev) => Math.min(prev + 1, filteredChats.length))
        }
        if (e.key === 'ArrowUp') {
          e.preventDefault()
          setSelectedIndex((prev) => Math.max(prev - 1, 0))
        }
        if (e.key === 'Enter') {
          e.preventDefault()
          if (filteredChats.length > 0 && selectedIndex < filteredChats.length) {
            // Navigate to selected chat
            navigate({ to: `/chat/${filteredChats[selectedIndex].id}` })
            setIsOpen(false)
            setQuery('')
          } else if (query.trim()) {
            // Create new chat
            handleCreateChat()
          }
        }
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [isOpen, query, selectedIndex, filteredChats, navigate, handleCreateChat])

  return (
    <AnimatePresence>
      {isOpen && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          className="fixed inset-0 z-50 flex items-center justify-center bg-card/50 backdrop-blur-sm"
          onClick={() => setIsOpen(false)}
        >
          <motion.div
            initial={{ scale: 0.95, opacity: 0 }}
            animate={{ scale: 1, opacity: 1 }}
            exit={{ scale: 0.95, opacity: 0 }}
            transition={{ type: 'spring', damping: 20, stiffness: 300 }}
            className="w-full max-w-2xl px-4"
            onClick={(e) => e.stopPropagation()}
          >
            <form onSubmit={handleSubmit}>
              <div className="relative">
                <motion.div
                  initial={{ y: 20, opacity: 0 }}
                  animate={{ y: 0, opacity: 1 }}
                  transition={{ delay: 0.1 }}
                  className={cn(
                    'flex items-center gap-2 rounded-lg border border-border bg-background/90 p-4 shadow-xl',
                    'focus-within:border-primary focus-within:ring-2 focus-within:ring-primary'
                  )}
                >
                  <Search className="h-5 w-5 text-muted-foreground" />
                  <input
                    type="text"
                    value={query}
                    onChange={(e) => {
                      setQuery(e.target.value)
                      setSelectedIndex(0)
                    }}
                    placeholder="Start a new chat or search existing chats..."
                    className="w-full bg-transparent text-foreground placeholder-muted-foreground outline-none"
                    autoFocus
                  />
                  <div className="flex items-center gap-1 text-xs text-muted-foreground">
                    <kbd className="rounded bg-muted px-2 py-1">Esc</kbd>
                    <span>to close</span>
                  </div>
                </motion.div>

                {/* Search Results */}
                {query && (
                  <motion.div
                    initial={{ opacity: 0, y: 10 }}
                    animate={{ opacity: 1, y: 0 }}
                    className="absolute mt-2 w-full rounded-lg border border-border bg-background/90 shadow-xl"
                  >
                    {filteredChats.map((chat, index) => (
                      <button
                        key={chat.id}
                        onClick={() => {
                          navigate({ to: `/chat/${chat.id}` })
                          setIsOpen(false)
                          setQuery('')
                        }}
                        className={cn(
                          'flex w-full items-center justify-between px-4 py-2 text-left text-sm',
                          'hover:bg-muted',
                          selectedIndex === index && 'bg-muted'
                        )}
                      >
                        <span className="truncate">{chat.name}</span>
                        <ChevronRight className="h-4 w-4 text-muted-foreground" />
                      </button>
                    ))}
                    {query.trim() && (
                      <button
                        onClick={handleCreateChat}
                        className={cn(
                          'flex w-full items-center justify-between px-4 py-2 text-left text-sm',
                          'hover:bg-muted',
                          selectedIndex === filteredChats.length && 'bg-muted'
                        )}
                      >
                        <span>Create new chat: &quot;{query}&quot;</span>
                        <ChevronRight className="h-4 w-4 text-muted-foreground" />
                      </button>
                    )}
                  </motion.div>
                )}
              </div>
            </form>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  )
}
