import { useState, useCallback, useEffect, useRef } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { Input } from '@renderer/components/ui/input'
import { Calendar, Search, GraduationCap, Telescope, Brain } from 'lucide-react'
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
import { useDebounce } from '@renderer/hooks/useDebounce'
import { ScrollArea } from '../ui/scroll-area'
import { useVoiceStore } from '@renderer/lib/stores/voice'
import ChatInputBox from './ChatInputBox'
import VoiceVisualizer from './voice/VoiceVisualizer'
import useKokoroInstallationStatus from '@renderer/hooks/useDepencyStatus'

interface IndexRouteSearch {
  focusInput?: string
}

const UPDATE_PROFILE = gql`
  mutation UpdateProfile($input: UpdateProfileInput!) {
    updateProfile(input: $input)
  }
`

export function Home() {
  const { data: profile, refetch: refetchProfile } = useQuery(GetProfileDocument)
  const { data: chatsData } = useQuery(GetChatsDocument, {
    variables: { first: 20, offset: 0 }
  })
  const { isVoiceMode, toggleVoiceMode } = useVoiceStore()
  const navigate = useNavigate()
  const router = useRouter()
  const searchParams = useSearch({ from: '/' }) as IndexRouteSearch
  const [query, setQuery] = useState('')
  const [editedName, setEditedName] = useState('')
  const [isEditingName, setIsEditingName] = useState(false)
  const [selectedIndex, setSelectedIndex] = useState(-1)
  const debouncedQuery = useDebounce(query, 300)
  const [isReasonSelected, setIsReasonSelected] = useState(false)

  const [createChat] = useMutation(CreateChatDocument)
  const [sendMessage] = useMutation(SendMessageDocument)
  const [updateProfile] = useMutation(UPDATE_PROFILE)

  const textareaRef = useRef<HTMLTextAreaElement>(null)

  const chats = chatsData?.getChats || []
  const filteredChats = chats.filter((chat) =>
    chat.name.toLowerCase().includes(debouncedQuery.toLowerCase())
  )

  const { installationStatus } = useKokoroInstallationStatus()
  const isVoiceInstalled =
    installationStatus.status?.toLowerCase() === 'completed' || installationStatus.progress === 100

  const dummySuggestions = [
    // {
    //   id: 'dummy0',
    //   name: 'I notice you seem stressed. Would you like to talk about it?',
    //   icon: MessageCircle,
    //   emphasized: true
    // },
    { id: 'dummya', name: "Let's get to know each other", icon: Telescope },
    { id: 'dummy1', name: 'Help me plan my day and set priorities', icon: Calendar },
    { id: 'dummy2', name: 'Research and summarize a topic for me', icon: Search },
    { id: 'dummy3', name: 'Help me learn a new skill or concept', icon: GraduationCap }
    // { id: 'dummy4', name: 'Review and improve my writing', icon: PenTool },
    // { id: 'dummy5', name: 'Help me make a decision', icon: Brain }
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
        variables: { name: query, voice: isVoiceMode }
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

        sendMessage({
          variables: {
            chatId: newChatId,
            text: query,
            reasoning: isReasonSelected,
            voice: isVoiceMode
          }
        })
        setQuery('')
      }
    } catch (error) {
      console.error('Failed to create chat:', error)
    }
  }, [query, navigate, createChat, sendMessage, router, isReasonSelected, isVoiceMode])

  const handleSubmit = (e: React.FormEvent | React.KeyboardEvent<HTMLTextAreaElement>) => {
    e.preventDefault()
    if (query.trim()) {
      if (
        debouncedQuery &&
        filteredChats.length > 0 &&
        selectedIndex < filteredChats.length &&
        selectedIndex >= 0
      ) {
        navigate({ to: `/chat/${filteredChats[selectedIndex].id}` })
        setQuery('')
      } else {
        handleCreateChat()
      }
    } else {
      if (!debouncedQuery && selectedIndex >= 0 && selectedIndex < dummySuggestions.length) {
        const selectedSuggestion = dummySuggestions[selectedIndex]
        handleSuggestionClick(selectedSuggestion)
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
        variables: { name: suggestion.name, voice: isVoiceMode }
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

        sendMessage({
          variables: {
            chatId: newChatId,
            text: suggestion.name,
            reasoning: isReasonSelected,
            voice: isVoiceMode
          }
        })
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
      className="flex flex-col w-full max-w-2xl mx-auto px-4 h-full justify-center"
    >
      {!isVoiceMode && (
        <motion.div
          key="header"
          initial={{ opacity: 0, y: -20 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: -20 }}
          transition={{ duration: 0.3, ease: 'linear' }}
          className="flex flex-col items-center gap-4 w-full py-8"
        >
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
        </motion.div>
      )}

      {isVoiceMode && (
        <motion.div
          key="voice-visualizer"
          initial={{ opacity: 0, scale: 0.8 }}
          animate={{ opacity: 1, scale: 1 }}
          exit={{ opacity: 0, scale: 0.8 }}
          transition={{ duration: 0.3, ease: 'linear' }}
          className="flex-1 w-full flex items-center justify-center min-h-[300px]"
        >
          <VoiceVisualizer
            visualState={1}
            getFreqData={() => new Uint8Array()}
            className="min-w-60 min-h-40"
          />
        </motion.div>
      )}

      <motion.div
        layout="position"
        transition={{
          layout: { duration: 0.3, ease: 'linear' }
        }}
        className="relative w-full"
      >
        <ChatInputBox
          isVoiceInstalled={isVoiceInstalled}
          query={query}
          textareaRef={textareaRef}
          isReasonSelected={isReasonSelected}
          isVoiceMode={isVoiceMode}
          onVoiceModeChange={toggleVoiceMode}
          onInputChange={setQuery}
          handleSubmit={handleSubmit}
          setIsReasonSelected={setIsReasonSelected}
          handleCreateChat={handleCreateChat}
        />

        <AnimatePresence mode="wait">
          {!isVoiceMode && (
            <motion.div
              key="suggestions"
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: 296 }}
              exit={{ opacity: 0, height: 0 }}
              transition={{
                opacity: { duration: 0.2 },
                height: { duration: 0.3, ease: 'easeOut' }
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
                                'relative before:absolute before:inset-0 before:rounded-'
                            )}
                          >
                            <Icon
                              className={cn(
                                'h-4 w-4 relative z-10',
                                isEmphasized
                                  ? 'text-indigo-800 dark:text-indigo-400'
                                  : 'text-muted-foreground'
                              )}
                            />
                            <span
                              className={cn(
                                'truncate relative z-10',
                                isEmphasized && 'font-medium text-indigo-800 dark:text-indigo-200'
                              )}
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
          )}
        </AnimatePresence>
      </motion.div>
    </motion.div>
  )
}
