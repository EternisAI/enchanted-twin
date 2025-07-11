import { useState, useCallback, useEffect, useRef } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { Calendar, Search, GraduationCap, Telescope, Brain } from 'lucide-react'
import { useNavigate, useRouter, useSearch } from '@tanstack/react-router'
import { useQuery, useMutation, gql } from '@apollo/client'
import { toast } from 'sonner'

import { Input } from '@renderer/components/ui/input'
import {
  GetProfileDocument,
  GetChatsDocument,
  CreateChatDocument,
  ChatCategory,
  SendMessageDocument
} from '@renderer/graphql/generated/graphql'
import { client } from '@renderer/graphql/lib'
import { ContextCard } from './ContextCard'
import { cn } from '@renderer/lib/utils'
import { useDebounce } from '@renderer/hooks/useDebounce'
import { ScrollArea } from '../ui/scroll-area'
import { useVoiceStore } from '@renderer/lib/stores/voice'
import ChatInputBox from './ChatInputBox'
import VoiceVisualizer from './voice/VoiceVisualizer'
import useDependencyStatus from '@renderer/hooks/useDependencyStatus'
import { VoiceModeInput } from './voice/ChatVoiceModeView'

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
  const { isVoiceMode, stopVoiceMode, startVoiceMode } = useVoiceStore()
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
  const [updateProfile] = useMutation(UPDATE_PROFILE)
  const [sendMessage] = useMutation(SendMessageDocument)

  const textareaRef = useRef<HTMLTextAreaElement>(null)

  const chats = chatsData?.getChats || []
  const filteredChats = chats.filter((chat) =>
    chat.name.toLowerCase().includes(debouncedQuery.toLowerCase())
  )

  const { isVoiceReady } = useDependencyStatus()

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

  const handleCreateChat = useCallback(
    async (chatTitle?: string, isVoiceMode?: boolean) => {
      const message = query || chatTitle || ''
      if (!message.trim()) return

      try {
        const reducedMessage = message.length > 100 ? message.slice(0, 100) + '...' : message
        const { data: createData } = await createChat({
          variables: {
            name: chatTitle || reducedMessage,
            category: isVoiceMode ? ChatCategory.Voice : ChatCategory.Text
          }
        })
        const newChatId = createData?.createChat?.id

        if (newChatId) {
          navigate({
            to: `/chat/${newChatId}`,
            search: { initialMessage: query }
          })

          sendMessage({
            variables: {
              chatId: newChatId,
              text: query,
              reasoning: isReasonSelected,
              voice: isVoiceMode || false
            }
          })

          await client.cache.evict({ fieldName: 'getChats' })
          await router.invalidate({
            filter: (match) => match.routeId === '/chat/$chatId'
          })

          setQuery('')
          isVoiceMode && startVoiceMode(newChatId)
        }
      } catch (error) {
        console.error('Failed to create chat:', error)
      }
    },
    [query, navigate, createChat, router, startVoiceMode, isReasonSelected, sendMessage]
  )

  const handleSubmit = (e: React.FormEvent | React.KeyboardEvent<HTMLTextAreaElement>) => {
    e.preventDefault()
    if (query.trim()) {
      if (
        debouncedQuery &&
        filteredChats.length > 0 &&
        selectedIndex < filteredChats.length + 1 &&
        selectedIndex >= 1
      ) {
        // selectedIndex 1 corresponds to filteredChats[0], selectedIndex 2 to filteredChats[1], etc.
        navigate({ to: `/chat/${filteredChats[selectedIndex - 1].id}` })
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

  const handleToggleToVoiceMode = async () => {
    await handleCreateChat('New Voice Chat', true)
  }

  useEffect(() => {
    const handleArrowKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'ArrowDown') {
        e.preventDefault()
        const maxIndex = debouncedQuery ? filteredChats.length : dummySuggestions.length - 1
        setSelectedIndex((prev) => Math.min(prev + 1, maxIndex))
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault()
        setSelectedIndex((prev) => Math.max(prev - 1, 0))
      }
    }

    window.addEventListener('keydown', handleArrowKeyDown)
    return () => window.removeEventListener('keydown', handleArrowKeyDown)
  }, [selectedIndex, debouncedQuery, filteredChats.length, dummySuggestions.length])

  const handleSuggestionClick = async (suggestion: (typeof dummySuggestions)[0]) => {
    try {
      const { data: createData } = await createChat({
        variables: {
          name: 'New Chat',
          category: isVoiceMode ? ChatCategory.Voice : ChatCategory.Text,
          initialMessage: suggestion.name
        }
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

        setQuery('')
      }
    } catch (error) {
      console.error('Failed to create chat:', error)
    }
  }

  const twinName = profile?.profile?.name || 'Your Twin'

  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{ type: 'spring', stiffness: 350, damping: 55 }}
      className="flex flex-col w-full max-w-2xl mx-auto px-4 h-full justify-center"
    >
      {!isVoiceMode && (
        <motion.div
          key="header"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ type: 'spring', stiffness: 350, damping: 55 }}
          className="flex flex-col items-center gap-4 w-full py-8"
        >
          <motion.div layout="position" className="w-full max-w-md">
            <AnimatePresence mode="wait">
              {isEditingName ? (
                <motion.div
                  key="name-input"
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  exit={{ opacity: 0 }}
                  transition={{
                    duration: 0.2,
                    ease: [0.4, 0, 0.2, 1],
                    layout: { duration: 0.3, ease: [0.4, 0, 0.2, 1] }
                  }}
                  layout="position"
                  className="w-full"
                >
                  <Input
                    value={editedName}
                    onChange={(e) => setEditedName(e.target.value)}
                    onKeyDown={handleNameEditKeyDown}
                    onBlur={handleNameUpdate}
                    autoFocus
                    className="!text-2xl font-bold text-center min-h-10 w-full mx-auto"
                  />
                </motion.div>
              ) : (
                <motion.h1
                  key="name-display"
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  exit={{ opacity: 0 }}
                  transition={{
                    duration: 0.2,
                    ease: [0.4, 0, 0.2, 1],
                    layout: { duration: 0.3, ease: [0.4, 0, 0.2, 1] }
                  }}
                  layout="position"
                  className="text-2xl font-bold cursor-pointer hover:bg-muted/50 rounded-lg transition-all text-center w-fit mx-auto min-h-10 flex items-center justify-center px-4"
                  onClick={() => {
                    setEditedName(twinName)
                    setIsEditingName(true)
                  }}
                >
                  {twinName}
                </motion.h1>
              )}
            </AnimatePresence>
          </motion.div>
          <motion.div layout="position">
            <ContextCard />
          </motion.div>
        </motion.div>
      )}

      {isVoiceMode && (
        <motion.div
          key="voice-visualizer"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ type: 'spring', stiffness: 350, damping: 55 }}
          className="flex-1 w-full flex items-center justify-center min-h-[300px]"
        >
          <VoiceVisualizer
            visualState={1}
            getFreqData={() => new Uint8Array()}
            className="min-w-60 min-h-40"
          />
        </motion.div>
      )}

      <div className="relative w-full">
        {isVoiceMode ? (
          <VoiceModeInput onStop={stopVoiceMode} />
        ) : (
          <motion.div layout="position">
            <ChatInputBox
              isVoiceReady={isVoiceReady}
              query={query}
              textareaRef={textareaRef}
              isReasonSelected={isReasonSelected}
              isVoiceMode={isVoiceMode}
              onVoiceModeChange={handleToggleToVoiceMode}
              onInputChange={setQuery}
              handleSubmit={handleSubmit}
              setIsReasonSelected={setIsReasonSelected}
              handleCreateChat={handleCreateChat}
            />
          </motion.div>
        )}

        <AnimatePresence mode="wait">
          {!isVoiceMode && (
            <motion.div
              key="suggestions"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              transition={{
                opacity: { duration: 0.2, delay: 0.4 }
              }}
              className="relative w-full overflow-hidden"
              layout="position"
            >
              <div className="bg-background/90 backdrop-blur-sm">
                <ScrollArea className="h-[280px] mt-4 pr-4">
                  {debouncedQuery ? (
                    <>
                      <motion.button
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        transition={{ duration: 0.15, delay: 0 }}
                        type="button"
                        onClick={() => {
                          handleCreateChat()
                          setQuery('')
                        }}
                        className={cn(
                          'flex w-full items-center gap-2 px-3 py-2 text-left text-sm rounded-md',
                          'hover:bg-muted/80',
                          selectedIndex === 0 && 'bg-primary/10 text-primary'
                        )}
                      >
                        <span className="truncate">Create new chat</span>
                      </motion.button>
                      {filteredChats.map((chat, index) => (
                        <motion.button
                          key={chat.id}
                          initial={{ opacity: 0 }}
                          animate={{ opacity: 1 }}
                          transition={{ duration: 0.15, delay: index * 0.07 + 0.4 }}
                          type="button"
                          onClick={() => {
                            navigate({ to: `/chat/${chat.id}` })
                            setQuery('')
                          }}
                          className={cn(
                            'flex w-full items-center gap-2 px-3 py-2 text-left text-sm rounded-md',
                            'hover:bg-muted/80',
                            selectedIndex === index + 1 && 'bg-primary/10 text-primary'
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
                        const isEmphasized = 'emphasized' in chat && chat.emphasized === true
                        return (
                          <motion.button
                            key={chat.id}
                            initial={{ opacity: 0 }}
                            animate={{ opacity: 1 }}
                            transition={{ duration: 0.15, delay: index * 0.07 }}
                            type="button"
                            onClick={() => {
                              if (chat.id.startsWith('dummy')) {
                                handleSuggestionClick(chat as (typeof dummySuggestions)[0])
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
      </div>
    </motion.div>
  )
}
