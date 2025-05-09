import { useState, useCallback, useEffect } from 'react'
import { motion } from 'framer-motion'
import { Input } from '@renderer/components/ui/input'
import { Send, AudioLines, Calendar, Search, GraduationCap, PenTool, Brain } from 'lucide-react'
import { useNavigate } from '@tanstack/react-router'
import { useQuery, useMutation, gql } from '@apollo/client'
import { GetProfileDocument, GetChatsDocument } from '@renderer/graphql/generated/graphql'
import { CreateChatDocument, SendMessageDocument } from '@renderer/graphql/generated/graphql'
import { toast } from 'sonner'
import { client } from '@renderer/graphql/lib'
import { useRouter } from '@tanstack/react-router'
import { ContextCard } from './ContextCard'
import { cn } from '@renderer/lib/utils'
import { Button } from '../ui/button'
import { TooltipContent, TooltipTrigger } from '../ui/tooltip'
import { Tooltip } from '../ui/tooltip'
import { TooltipProvider } from '../ui/tooltip'
import { useDebounce } from '@renderer/hooks/useDebounce'
import { ScrollArea } from '../ui/scroll-area'

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
  const [updateProfile] = useMutation(UPDATE_PROFILE)
  const [isEditingName, setIsEditingName] = useState(false)
  const [editedName, setEditedName] = useState('')
  const [query, setQuery] = useState('')
  const debouncedQuery = useDebounce(query, 300)
  const [selectedIndex, setSelectedIndex] = useState(-1)
  const navigate = useNavigate()
  const router = useRouter()
  const [createChat] = useMutation(CreateChatDocument)
  const [sendMessage] = useMutation(SendMessageDocument)

  const chats = chatsData?.getChats || []
  const filteredChats = chats.filter((chat) =>
    chat.name.toLowerCase().includes(debouncedQuery.toLowerCase())
  )

  // Dummy suggestions for AI agent use cases
  const dummySuggestions = [
    { id: 'dummy1', name: 'Help me plan my day and set priorities', icon: Calendar },
    { id: 'dummy2', name: 'Research and summarize a topic for me', icon: Search },
    { id: 'dummy3', name: 'Help me learn a new skill or concept', icon: GraduationCap },
    { id: 'dummy4', name: 'Review and improve my writing', icon: PenTool },
    { id: 'dummy5', name: 'Help me make a decision', icon: Brain }
  ]

  // Always show dummy suggestions when there's no query
  const suggestions = debouncedQuery ? filteredChats : dummySuggestions

  // Reset selection when query changes
  useEffect(() => {
    if (!debouncedQuery) {
      setSelectedIndex(-1)
    } else {
      setSelectedIndex(0)
    }
  }, [debouncedQuery])

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

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
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

  const handleSubmit = (e: React.FormEvent) => {
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
    const handleKeyDown = (e: KeyboardEvent) => {
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
      layout="position"
      className="flex flex-col items-center justify-center gap-6 w-full max-w-2xl mx-auto px-4"
    >
      <div className="flex flex-col items-center gap-4 w-full">
        {isEditingName ? (
          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.5 }}
            className="w-full"
          >
            <Input
              value={editedName}
              onChange={(e) => setEditedName(e.target.value)}
              onKeyDown={handleKeyDown}
              onBlur={handleNameUpdate}
              autoFocus
              className="!text-2xl font-bold text-center"
            />
          </motion.div>
        ) : (
          <h1
            className="text-2xl font-bold cursor-pointer hover:text-gray-600 transition-all text-center"
            onClick={() => {
              setEditedName(twinName)
              setIsEditingName(true)
            }}
          >
            {twinName}
          </h1>
        )}
        {/* <SoundWaveContainer audioUrl="/path/to/your/audio/file.mp3" size={0.5} color="#4f46e5" /> */}
        <ContextCard />
      </div>

      <motion.div
        initial={{
          opacity: 0,
          scale: 0.9
        }}
        animate={{
          opacity: 1,
          scale: 1
        }}
        transition={{
          type: 'spring',
          stiffness: 100,
          damping: 15
        }}
        className="relative w-full"
      >
        <form onSubmit={handleSubmit} className="relative w-full space-y-6">
          <div className="flex items-center gap-2 p-1">
            <Input
              type="text"
              value={query}
              onChange={(e) => {
                setQuery(e.target.value)
              }}
              placeholder="Start a new chat..."
              className="flex-1 p-4 h-fit shadow-sm"
            />
            <div className="p-1">
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button variant="outline" type="button" className="rounded-full w-12 h-12">
                      <AudioLines className="h-9 w-9" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>
                    <p>Start voice chat</p>
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
            </div>
            {query.trim() && (
              <button type="submit" className="rounded-full p-2 text-primary hover:bg-muted">
                <Send className="h-5 w-5" />
              </button>
            )}
          </div>
          <div className="bg-background/90 backdrop-blur-sm">
            <ScrollArea className="h-[280px]">
              {debouncedQuery ? (
                <>
                  {filteredChats.map((chat, index) => (
                    <motion.button
                      key={chat.id}
                      initial={{ opacity: 0, y: 4 }}
                      animate={{ opacity: 1, y: 0 }}
                      transition={{ duration: 0.2, delay: index * 0.05 }}
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
                  <motion.button
                    initial={{ opacity: 0, y: 4 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ duration: 0.2, delay: filteredChats.length * 0.05 }}
                    type="button"
                    onClick={handleCreateChat}
                    className={cn(
                      'flex w-full items-center gap-2 px-3 py-2 text-left text-sm',
                      'hover:bg-muted/80',
                      selectedIndex === filteredChats.length && 'bg-primary/10 text-primary'
                    )}
                  >
                    <span>New chat: &quot;{debouncedQuery}&quot;</span>
                  </motion.button>
                </>
              ) : (
                <>
                  {suggestions.map((chat, index) => {
                    const Icon = 'icon' in chat ? chat.icon : Brain
                    return (
                      <motion.button
                        key={chat.id}
                        initial={{ opacity: 0, y: 4 }}
                        animate={{ opacity: 1, y: 0 }}
                        transition={{ duration: 0.2, delay: index * 0.05 }}
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
                          selectedIndex === index && 'bg-primary/10 text-primary'
                        )}
                      >
                        <Icon className="h-4 w-4 text-muted-foreground" />
                        <span className="truncate">{chat.name}</span>
                      </motion.button>
                    )
                  })}
                </>
              )}
            </ScrollArea>
          </div>
        </form>
      </motion.div>
    </motion.div>
  )
}
