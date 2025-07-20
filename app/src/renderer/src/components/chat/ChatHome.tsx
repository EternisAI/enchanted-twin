import { useState, useCallback, useEffect, useRef } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { Calendar, Search, GraduationCap, Telescope, Brain } from 'lucide-react'
import { useNavigate, useRouter, useSearch } from '@tanstack/react-router'
import { useQuery, useMutation } from '@apollo/client'
import {
  GetChatsDocument,
  CreateChatDocument,
  ChatCategory,
  SendMessageDocument
} from '@renderer/graphql/generated/graphql'
import { client } from '@renderer/graphql/lib'
import { ContextCard } from './personalize/ContextCard'
import { cn } from '@renderer/lib/utils'
import { useDebounce } from '@renderer/hooks/useDebounce'
import { ScrollArea } from '../ui/scroll-area'
import { useVoiceStore } from '@renderer/lib/stores/voice'
import ChatInputBox from './ChatInputBox'
import VoiceVisualizer from './voice/VoiceVisualizer'
import useDependencyStatus from '@renderer/hooks/useDependencyStatus'
import { VoiceModeInput } from './voice/VoiceModeInput'
import { TwinNameInput } from './personalize/TwinNameInput'

interface IndexRouteSearch {
  focusInput?: string
}

export function Home() {
  const { data: chatsData } = useQuery(GetChatsDocument, {
    variables: { first: 20, offset: 0 }
  })
  const { isVoiceMode, stopVoiceMode, startVoiceMode } = useVoiceStore()
  const { isVoiceReady } = useDependencyStatus()

  const navigate = useNavigate()
  const router = useRouter()
  const searchParams = useSearch({ from: '/' }) as IndexRouteSearch
  const [query, setQuery] = useState('')
  const [selectedIndex, setSelectedIndex] = useState(-1)
  const debouncedQuery = useDebounce(query.trim(), 300)
  const [isReasonSelected, setIsReasonSelected] = useState(false)

  const [createChat] = useMutation(CreateChatDocument)
  const [sendMessage] = useMutation(SendMessageDocument)

  const textareaRef = useRef<HTMLTextAreaElement>(null)

  const chats = chatsData?.getChats || []
  const filteredChats = chats.filter((chat) =>
    chat.name.toLowerCase().includes(debouncedQuery.toLowerCase())
  )

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

  useEffect(() => {
    // Reset showSuggestions when voice mode changes
    if (isVoiceMode) {
      setShowSuggestions(false)
    }
  }, [isVoiceMode])

  const handleCreateChat = useCallback(
    async (chatTitle?: string, isVoiceMode?: boolean) => {
      const message = (query || chatTitle || '').trim()
      if (!message) return

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
            search: { initialMessage: message, reasoning: isReasonSelected }
          })

          sendMessage({
            variables: {
              chatId: newChatId,
              text: message,
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
      const textarea = textareaRef.current
      if (!textarea) return

      const isAtEnd =
        textarea.selectionStart === textarea.value.length &&
        textarea.selectionEnd === textarea.value.length
      const isAtStart = textarea.selectionStart === 0 && textarea.selectionEnd === 0

      if (e.key === 'ArrowDown' && isAtEnd) {
        e.preventDefault()
        const maxIndex = debouncedQuery ? filteredChats.length : dummySuggestions.length - 1
        setSelectedIndex((prev) => Math.min(prev + 1, maxIndex))
      }
      if (e.key === 'ArrowUp') {
        // If we're at the first suggestion and user presses up, let textarea handle it normally
        if (selectedIndex === 0 && isAtStart) {
          setSelectedIndex(-1)
          // Don't prevent default - let cursor move in textarea
        } else if (selectedIndex > 0) {
          // Only prevent default if we're navigating suggestions
          e.preventDefault()
          setSelectedIndex((prev) => prev - 1)
        }
      }
    }

    const textarea = textareaRef.current
    if (textarea) {
      textarea.addEventListener('keydown', handleArrowKeyDown)
      return () => textarea.removeEventListener('keydown', handleArrowKeyDown)
    }
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
          search: { initialMessage: suggestion.name, reasoning: isReasonSelected }
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

  const [showSuggestions, setShowSuggestions] = useState(false)

  const initShowSuggestionsTimeout = useRef<ReturnType<typeof setTimeout> | null>(null)
  useEffect(() => {
    if (showSuggestions) {
      if (initShowSuggestionsTimeout.current) {
        clearTimeout(initShowSuggestionsTimeout.current)
      }
    } else {
      initShowSuggestionsTimeout.current = setTimeout(() => {
        setShowSuggestions(true)
      }, 1000)
    }
    return () => {
      if (initShowSuggestionsTimeout.current) {
        clearTimeout(initShowSuggestionsTimeout.current)
      }
    }
  }, [showSuggestions, setShowSuggestions])

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
          className="flex flex-col items-center py-4 px-4 w-full"
        >
          <TwinNameInput />
          <motion.div layout="position" className="w-full mt-2">
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
            onLayoutAnimationComplete={() => {
              console.log('Layout animation complete')
              setShowSuggestions(true)
            }}
          />
        )}

        <AnimatePresence mode="wait">
          {!isVoiceMode && (
            <motion.div
              key="suggestions"
              initial={{ opacity: 0 }}
              animate={{ opacity: showSuggestions ? 1 : 0 }}
              exit={{ opacity: 0 }}
              transition={{ duration: 0.2 }}
              className={cn(
                'relative w-full overflow-hidden',
                !showSuggestions && 'pointer-events-none'
              )}
              layout="position"
            >
              <div className="">
                <ScrollArea className="h-[280px] mt-4 pr-4">
                  {debouncedQuery ? (
                    <>
                      <motion.button
                        initial={{ opacity: 0 }}
                        animate={{ opacity: showSuggestions ? 1 : 0 }}
                        exit={{ opacity: 0 }}
                        transition={{ duration: 0.15, delay: 0 }}
                        type="button"
                        onClick={() => {
                          handleCreateChat()
                          setQuery('')
                        }}
                        className={cn(
                          'flex w-full items-center gap-2 px-3 py-2 text-left text-sm rounded-md',
                          'hover:bg-muted hover:text-foreground',
                          selectedIndex === 0 && 'bg-muted text-foreground'
                        )}
                      >
                        <span className="truncate">Create new chat</span>
                      </motion.button>
                      {filteredChats.map((chat, index) => (
                        <motion.button
                          key={chat.id}
                          initial={{ opacity: 0 }}
                          animate={{ opacity: showSuggestions ? 1 : 0 }}
                          transition={{
                            duration: 0.15,
                            delay: showSuggestions ? index * 0.07 + 0.4 : 0
                          }}
                          type="button"
                          onClick={() => {
                            navigate({ to: `/chat/${chat.id}` })
                            setQuery('')
                          }}
                          className={cn(
                            'flex w-full items-center gap-2 px-3 py-2 text-left text-sm rounded-md text-muted-foreground',
                            'hover:bg-muted hover:text-foreground',
                            selectedIndex === index + 1 && 'bg-muted text-foreground'
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
                            animate={{ opacity: showSuggestions ? 1 : 0 }}
                            transition={{
                              duration: 0.15,
                              delay: showSuggestions ? index * 0.07 : 0
                            }}
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
                              'flex w-full items-center gap-2 px-3 py-2 text-left text-sm rounded-md text-muted-foreground',
                              'hover:bg-muted hover:text-foreground',
                              selectedIndex === index && 'bg-muted text-foreground',
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
