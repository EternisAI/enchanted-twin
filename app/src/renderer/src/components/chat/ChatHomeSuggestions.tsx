import { motion, AnimatePresence } from 'framer-motion'
import { Brain } from 'lucide-react'
import { cn } from '@renderer/lib/utils'
import { ScrollArea } from '../ui/scroll-area'
import { useNavigate } from '@tanstack/react-router'
import { LucideIcon } from 'lucide-react'

export interface Suggestion {
  id: string
  name: string
  icon?: LucideIcon
  emphasized?: boolean
}

interface Chat {
  id: string
  name: string
}

interface ChatHomeSuggestionsProps {
  showSuggestions: boolean
  debouncedQuery: string
  filteredChats: Chat[]
  dummySuggestions: Suggestion[]
  selectedIndex: number
  handleCreateChat: () => void
  setQuery: (query: string) => void
  handleSuggestionClick: (suggestion: Suggestion) => void
}

export function ChatHomeSuggestions({
  showSuggestions,
  debouncedQuery,
  filteredChats,
  dummySuggestions,
  selectedIndex,
  handleCreateChat,
  setQuery,
  handleSuggestionClick
}: ChatHomeSuggestionsProps) {
  const navigate = useNavigate()

  return (
    <AnimatePresence mode="wait">
      <motion.div
        key="suggestions"
        initial={{ opacity: 0 }}
        animate={{ opacity: showSuggestions ? 1 : 0 }}
        exit={{ opacity: 0 }}
        transition={{ duration: 0.2 }}
        className={cn('relative w-full overflow-hidden', !showSuggestions && 'pointer-events-none')}
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
                {dummySuggestions.map((chat, index) => {
                  const Icon = chat.icon ?? Brain
                  const isEmphasized = chat.emphasized === true
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
                      onClick={() => handleSuggestionClick(chat)}
                      className={cn(
                        'flex w-full items-center gap-2 px-3 py-2 text-left text-sm rounded-md text-muted-foreground',
                        'hover:bg-muted hover:text-foreground',
                        selectedIndex === index && 'bg-muted text-foreground',
                        isEmphasized && 'relative before:absolute before:inset-0 before:rounded-'
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
    </AnimatePresence>
  )
}
