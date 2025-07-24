import { useState, useCallback, useEffect, useRef } from 'react'
import { motion } from 'framer-motion'
import { GraduationCap, Telescope, VideoIcon, AlarmCheckIcon } from 'lucide-react'
import { useNavigate, useRouter, useSearch } from '@tanstack/react-router'
import { useQuery, useMutation } from '@apollo/client'
import {
  GetChatsDocument,
  CreateChatDocument,
  ChatCategory,
  SendMessageDocument
} from '@renderer/graphql/generated/graphql'
import { client } from '@renderer/graphql/lib'
import { useDebounce } from '@renderer/hooks/useDebounce'
import { useVoiceStore } from '@renderer/lib/stores/voice'
import ChatInputBox from './ChatInputBox'
import useDependencyStatus from '@renderer/hooks/useDependencyStatus'
import { ChatHomeHeader } from './ChatHomeHeader'
import { ChatHomeSuggestions } from './ChatHomeSuggestions'
import { Suggestion } from './ChatHomeSuggestions'
import { ConnectSourcesButton } from '../data-sources/ConnectButton'

interface IndexRouteSearch {
  focusInput?: string
}

export function Home() {
  const { data: chatsData } = useQuery(GetChatsDocument, {
    variables: { first: 20, offset: 0 }
  })
  const { isVoiceMode, startVoiceMode } = useVoiceStore()
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

  // Ensure dummySuggestions match Suggestion type
  const dummySuggestions: Suggestion[] = [
    { id: 'reminder', name: 'Create a reminder', icon: AlarmCheckIcon },
    { id: 'create-video', name: 'Create a video', icon: VideoIcon },
    { id: 'personalize', name: "Let's get to know each other", icon: Telescope },
    { id: 'learn', name: 'Help me learn a new skill or concept', icon: GraduationCap }
  ]

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

  const handleSuggestionClick = async (suggestion: Suggestion) => {
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
      }, 500)
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
      <ChatHomeHeader />

      <div className="relative w-full">
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

        <ChatHomeSuggestions
          showSuggestions={showSuggestions}
          debouncedQuery={debouncedQuery}
          filteredChats={filteredChats}
          dummySuggestions={dummySuggestions}
          selectedIndex={selectedIndex}
          handleCreateChat={handleCreateChat}
          setQuery={setQuery}
          handleSuggestionClick={handleSuggestionClick}
        />
      </div>
      <motion.div
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: showSuggestions ? 1 : 0, y: showSuggestions ? 0 : 10 }}
        transition={{ type: 'spring', stiffness: 150, damping: 15 }}
        className="flex justify-center absolute bottom-0 left-0 right-0 p-4"
      >
        <ConnectSourcesButton />
      </motion.div>
    </motion.div>
  )
}
