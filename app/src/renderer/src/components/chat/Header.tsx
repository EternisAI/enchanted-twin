import { useState, useCallback, useEffect, useRef } from 'react'
import { motion, AnimatePresence, LayoutGroup } from 'framer-motion'
import { Input } from '@renderer/components/ui/input'
import { Textarea } from '@renderer/components/ui/textarea'
import {
  Send,
  AudioLines,
  Calendar,
  Search,
  GraduationCap,
  PenTool,
  Brain,
  MessageCircle,
  Telescope
} from 'lucide-react'
import { useNavigate, useRouter, useSearch } from '@tanstack/react-router'
import { useQuery, useMutation, gql } from '@apollo/client'
import {
  GetProfileDocument,
  GetChatsDocument,
  CreateChatDocument,
  SendMessageDocument
} from '@renderer/graphql/generated/graphql'
import { toast } from 'sonner'
import { client } from '@renderer/graphql/lib'
import { ContextCard } from './ContextCard'
import { cn } from '@renderer/lib/utils'
import { Button } from '../ui/button'
import { TooltipContent, TooltipTrigger, Tooltip, TooltipProvider } from '../ui/tooltip'
import { useDebounce } from '@renderer/hooks/useDebounce'
import { ScrollArea } from '../ui/scroll-area'

// Define expected search params type that matches routes/index.tsx
interface IndexRouteSearch {
  focusInput?: string
}

const UPDATE_PROFILE = gql`
  mutation UpdateProfile($input: UpdateProfileInput!) {
    updateProfile(input: $input)
  }
`

export function Header() {
  const { data: profile, refetch: refetchProfile } = useQuery(GetProfileDocument)
  const { data: chatsData } = useQuery(GetChatsDocument, {
    variables: { first: 20, offset: 0 }
  })
  const navigate = useNavigate()
  const router = useRouter()
  const searchParams = useSearch({ from: '/' }) as IndexRouteSearch
  const [query, setQuery] = useState('')
  const [editedName, setEditedName] = useState('')
  const [isEditingName, setIsEditingName] = useState(false)
  const [selectedIndex, setSelectedIndex] = useState(-1)
  const debouncedQuery = useDebounce(query, 300)
  const [createChat] = useMutation(CreateChatDocument)
  const [sendMessage] = useMutation(SendMessageDocument)
  const [updateProfile] = useMutation(UPDATE_PROFILE)
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  const chats = chatsData?.getChats || []
  const filteredChats = chats.filter((chat) =>
    chat.name.toLowerCase().includes(debouncedQuery.toLowerCase())
  )

  const dummySuggestions = [
    {
      id: 'dummy0',
      name: 'I notice you seem stressed. Would you like to talk about it?',
      icon: MessageCircle,
      emphasized: true
    },
    { id: 'dummya', name: "Let's get to know each other", icon: Telescope },
    { id: 'dummy1', name: 'Help me plan my day and set priorities', icon: Calendar },
    { id: 'dummy2', name: 'Research and summarize a topic for me', icon: Search },
    { id: 'dummy3', name: 'Help me learn a new skill or concept', icon: GraduationCap },
    { id: 'dummy4', name: 'Review and improve my writing', icon: PenTool },
    { id: 'dummy5', name: 'Help me make a decision', icon: Brain }
  ]

  const suggestions = debouncedQuery ? filteredChats : dummySuggestions

  useEffect(() => {
    if (!debouncedQuery) {
      setSelectedIndex(-1)
    } else {
      setSelectedIndex(0)
    }
  }, [debouncedQuery])

  useEffect(() => {
    if (searchParams && searchParams.focusInput === 'true') {
      textareaRef.current?.focus()
      const currentPath = router.state.location.pathname
      const existingSearchParams = { ...router.state.location.search }
      delete (existingSearchParams as IndexRouteSearch).focusInput

      navigate({
        to: currentPath,
        search: existingSearchParams,
        replace: true,
        resetScroll: false
      })
    }
  }, [searchParams, navigate, router.state.location.pathname, router.state.location.search])

  const adjustTextareaHeight = () => {
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto'
      const scrollHeight = textareaRef.current.scrollHeight
      const maxHeight = 240
      textareaRef.current.style.height = `${Math.min(scrollHeight, maxHeight)}px`
    }
  }

  useEffect(() => {
    adjustTextareaHeight()
  }, [query])

  const handleNameUpdate = async () => {
    if (!editedName.trim()) {
      toast.error('Name cannot be empty')
      return
    }

    try {
      await updateProfile({
        variables: {
          input: {
            name: editedName.trim()
          }
        }
      })
      await refetchProfile()
      setIsEditingName(false)
      toast.success('Name updated successfully')
    } catch (error) {
      console.error('Failed to update name:', error)
      toast.error('Failed to update name')
    }
  }

  const handleNameEditKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      handleNameUpdate()
    } else if (e.key === 'Escape') {
      setIsEditingName(false)
      setEditedName('')
    }
  }

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

        await client.cache.evict({ fieldName: 'getChats' })
        await router.invalidate({
          filter: (match) => match.routeId === '/chat/$chatId'
        })

        sendMessage({ variables: { chatId: newChatId, text: query } })
        setQuery('')
      }
    } catch (error) {
      console.error('Failed to create chat:', error)
    }
  }, [query, navigate, createChat, sendMessage, router])

  const handleSubmit = (e: React.FormEvent | React.KeyboardEvent<HTMLTextAreaElement>) => {
    e.preventDefault()
    if (query.trim()) {
      if (filteredChats.length > 0 && selectedIndex < filteredChats.length) {
        navigate({ to: `/chat/${filteredChats[selectedIndex].id}` })
        setQuery('')
      } else {
        handleCreateChat()
      }
    }
  }

  useEffect(() => {
    const handleArrowKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'ArrowDown') {
        e.preventDefault()
        setSelectedIndex((prev) => Math.min(prev + 1, suggestions.length - 1))
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault()
        setSelectedIndex((prev) => Math.max(prev - 1, 0))
      }
    }

    window.addEventListener('keydown', handleArrowKeyDown)
    return () => window.removeEventListener('keydown', handleArrowKeyDown)
  }, [selectedIndex, suggestions])

  const handleSuggestionClick = async (suggestion: (typeof dummySuggestions)[0]) => {
    try {
      const { data: createData } = await createChat({
        variables: { name: suggestion.name }
      })
      const newChatId = createData?.createChat?.id

      if (newChatId) {
        navigate({
          to: `/chat/${newChatId}`,
          search: { initialMessage: suggestion.name }
        })

        await client.cache.evict({ fieldName: 'getChats' })
        await router.invalidate({
          filter: (match) => match.routeId === '/chat/$chatId'
        })

        sendMessage({ variables: { chatId: newChatId, text: suggestion.name } })
        setQuery('')
      }
    } catch (error) {
      console.error('Failed to create chat:', error)
    }
  }

  const twinName = profile?.profile?.name || 'Your Twin'

  return (
    <motion.div
      initial={{ opacity: 0, scale: 0.98 }}
      animate={{ opacity: 1, scale: 1 }}
      transition={{ type: 'spring', stiffness: 120, damping: 20 }}
      className="flex flex-col items-center justify-center gap-6 w-full max-w-2xl mx-auto px-4"
    >
      <div className="flex flex-col items-center gap-4 w-full">
        {isEditingName ? (
          <motion.div
            layout
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.5 }}
            className="w-full"
          >
            <Input
              value={editedName}
              onChange={(e) => setEditedName(e.target.value)}
              onKeyDown={handleNameEditKeyDown}
              onBlur={handleNameUpdate}
              autoFocus
              className="!text-2xl font-bold text-center"
            />
          </motion.div>
        ) : (
          <motion.h1
            layout
            className="text-2xl font-bold cursor-pointer hover:text-gray-600 transition-all text-center"
            onClick={() => {
              setEditedName(twinName)
              setIsEditingName(true)
            }}
          >
            {twinName}
          </motion.h1>
        )}
        <motion.div layout>
          <ContextCard />
        </motion.div>
      </div>

      <div className="relative w-full">
        <form onSubmit={handleSubmit} className="relative w-full">
          <div className="flex items-center gap-6 p-1">
            <div className="rounded-xl transition-all duration-300 focus-within:shadow-xl hover:shadow-xl relative z-10 flex items-center gap-2 flex-1 bg-card hover:bg-card/80 border px-2">
              <Textarea
                ref={textareaRef}
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                onKeyDown={(e: React.KeyboardEvent<HTMLTextAreaElement>) => {
                  if (e.key === 'Enter' && !e.shiftKey) {
                    e.preventDefault()
                    handleSubmit(e)
                  }
                }}
                placeholder="Start a new chat..."
                className="!text-base flex-1 border-0 shadow-none focus-visible:ring-0 focus-visible:ring-offset-0 py-4 pl-2 pr-1 resize-none overflow-y-hidden min-h-[58px] bg-transparent"
                rows={1}
              />
              <div className="flex items-center self-end gap-1 pb-2">
                <LayoutGroup id="chat-input-buttons">
                  <motion.div layout>
                    <TooltipProvider>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Button variant="ghost" type="button" size="icon" className="h-10 w-10">
                            <AudioLines className="h-5 w-5" />
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>
                          <p>Start voice chat</p>
                        </TooltipContent>
                      </Tooltip>
                    </TooltipProvider>
                  </motion.div>
                  <AnimatePresence mode="wait">
                    {debouncedQuery.trim() && (
                      <motion.div
                        layout
                        key="send-button"
                        initial={{ opacity: 0, scale: 0.7 }}
                        animate={{ opacity: 1, scale: 1 }}
                        exit={{ opacity: 0, scale: 0.7, transition: { duration: 0.1 } }}
                        transition={{ type: 'spring', stiffness: 400, damping: 25 }}
                      >
                        <Button variant="ghost" type="submit" size="icon" className="h-10 w-10">
                          <Send className="h-5 w-5" />
                        </Button>
                      </motion.div>
                    )}
                  </AnimatePresence>
                </LayoutGroup>
              </div>
            </div>
          </div>
        </form>
        <AnimatePresence mode="wait">
          <motion.div
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: 296 }}
            exit={{ opacity: 0, height: 0 }}
            transition={{
              opacity: { duration: 0.15 },
              height: { duration: 0.2, ease: 'easeOut' }
            }}
            className="relative w-full overflow-hidden"
          >
            <div className="h-4" />
            <div className="bg-background/90 backdrop-blur-sm">
              <ScrollArea className="h-[280px]">
                {debouncedQuery ? (
                  <>
                    {filteredChats.map((chat, index) => (
                      <motion.button
                        key={chat.id}
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        transition={{ duration: 0.15, delay: index * 0.03 }}
                        type="button"
                        onClick={() => {
                          navigate({ to: `/chat/${chat.id}` })
                          setQuery('')
                        }}
                        className={cn(
                          'flex w-full items-center gap-2 px-3 py-2 text-left text-sm rounded-md',
                          'hover:bg-muted/80',
                          selectedIndex === index && 'bg-primary/10 text-primary'
                        )}
                      >
                        <span className="truncate">{chat.name}</span>
                      </motion.button>
                    ))}
                  </>
                ) : (
                  <>
                    {suggestions.map((chat, index) => {
                      const Icon = 'icon' in chat ? chat.icon : Brain
                      const isEmphasized = 'emphasized' in chat && chat.emphasized
                      return (
                        <motion.button
                          key={chat.id}
                          initial={{ opacity: 0 }}
                          animate={{ opacity: 1 }}
                          transition={{ duration: 0.15, delay: index * 0.03 }}
                          type="button"
                          onClick={() => {
                            if (chat.id.startsWith('dummy')) {
                              handleSuggestionClick(chat)
                            } else {
                              navigate({ to: `/chat/${chat.id}` })
                              setQuery('')
                            }
                          }}
                          className={cn(
                            'flex w-full items-center gap-2 px-3 py-2 text-left text-sm rounded-md',
                            'hover:bg-muted/80',
                            selectedIndex === index && 'bg-primary/10 text-primary',
                            isEmphasized &&
                              'relative before:absolute before:inset-0 before:rounded-md'
                          )}
                        >
                          <Icon
                            className={cn(
                              'h-4 w-4 relative z-10',
                              isEmphasized ? 'text-primary' : 'text-muted-foreground'
                            )}
                          />
                          <span
                            className={cn('truncate relative z-10', isEmphasized && 'font-medium')}
                          >
                            {chat.name}
                          </span>
                        </motion.button>
                      )
                    })}
                  </>
                )}
              </ScrollArea>
            </div>
          </motion.div>
        </AnimatePresence>
      </div>
    </motion.div>
  )
}
