import { Link, useNavigate, useRouter, useRouterState } from '@tanstack/react-router'
import { Chat, DeleteChatDocument, GetChatsDocument } from '@renderer/graphql/generated/graphql'
import { cn } from '@renderer/lib/utils'
import {
  AlertDialog,
  AlertDialogTrigger,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogCancel
} from '../ui/alert-dialog'
import { Button } from '../ui/button'
import {
  Plus,
  Trash2,
  PanelLeftClose,
  SettingsIcon,
  SearchIcon,
  ChevronDown,
  ChevronUp,
  CheckSquare
} from 'lucide-react'
import { useMutation } from '@apollo/client'
import { client } from '@renderer/graphql/lib'
import { Omnibar } from '../Omnibar'
import { isToday, isYesterday, isWithinInterval, subDays } from 'date-fns'
import { useSettingsStore } from '@renderer/lib/stores/settings'
import { useOmnibarStore } from '@renderer/lib/stores/omnibar'
import { Tooltip, TooltipTrigger, TooltipContent, TooltipProvider } from '../ui/tooltip'
import { useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'

interface SidebarProps {
  chats: Chat[]
  setSidebarOpen: (open: boolean) => void
}

const groupChatsByTime = (chats: Chat[]) => {
  const now = new Date()
  const groups: { [key: string]: Chat[] } = {
    Today: [],
    Yesterday: [],
    'Previous 7 Days': [],
    'Previous 30 Days': [],
    Older: []
  }

  chats.forEach((chat) => {
    const chatDate = new Date(chat.createdAt)
    if (isToday(chatDate)) {
      groups.Today.push(chat)
    } else if (isYesterday(chatDate)) {
      groups.Yesterday.push(chat)
    } else if (isWithinInterval(chatDate, { start: subDays(now, 7), end: now })) {
      groups['Previous 7 Days'].push(chat)
    } else if (isWithinInterval(chatDate, { start: subDays(now, 30), end: now })) {
      groups['Previous 30 Days'].push(chat)
    } else {
      groups.Older.push(chat)
    }
  })
  return groups
}

export function Sidebar({ chats, setSidebarOpen }: SidebarProps) {
  const { location } = useRouterState()
  const navigate = useNavigate()
  const { open: openSettings } = useSettingsStore()
  const { openOmnibar } = useOmnibarStore()
  const [showAllChats, setShowAllChats] = useState(false)

  const handleNewChat = () => {
    navigate({ to: '/', search: { focusInput: 'true' } })
  }

  const handleNavigateTasks = () => {
    navigate({ to: '/tasks' })
  }

  const chatsToDisplay = showAllChats ? chats : chats.slice(0, 5)
  const groupedChats = groupChatsByTime(chatsToDisplay)

  const renderGroup = (title: string, groupChats: Chat[]) => {
    if (groupChats.length === 0) return null
    return (
      <motion.div
        key={title}
        className="mb-6"
        initial="hidden"
        animate="visible"
        exit="exit"
        variants={{
          visible: {
            transition: {
              staggerChildren: 0.1,
              delayChildren: 0.01
            }
          },
          exit: {
            transition: {
              staggerChildren: 0.02,
              staggerDirection: -1
            }
          }
        }}
      >
        <h3 className="text-xs font-medium text-muted-foreground uppercase p-2">{title}</h3>
        {groupChats.map((chat) => (
          <SidebarItem
            key={chat.id}
            chat={chat}
            isActive={location.pathname === `/chat/${chat.id}`}
          />
        ))}
      </motion.div>
    )
  }

  return (
    <>
      <aside className="flex flex-col bg-muted p-4 rounded-tr-lg h-full gap-2 w-64">
        <div className="flex items-center justify-between mb-3">
          <div className="flex items-center gap-1.5">
            <Button
              variant="ghost"
              size="icon"
              onClick={() => setSidebarOpen(false)}
              className="text-muted-foreground hover:text-foreground h-7 w-7"
            >
              <PanelLeftClose className="w-4 h-4" />
            </Button>
          </div>
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={openOmnibar}
                  className="text-muted-foreground hover:text-foreground h-7 w-7"
                >
                  <SearchIcon className="w-4 h-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent side="bottom" align="center">
                <div className="flex items-center gap-2">
                  <span>Search</span>
                  <kbd className="rounded bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground font-sans">
                    ⌘ K
                  </kbd>
                </div>
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
        </div>

        <Button
          variant="outline"
          className="w-full justify-between px-2 h-9 mb-1 group"
          onClick={handleNewChat}
        >
          <div className="flex items-center gap-2">
            <Plus className="w-3 h-3" />
            <span className="text-sm">New chat</span>
          </div>
          <div className="group-hover:opacity-100 transition-opacity opacity-0 flex items-center gap-2 text-[10px] text-muted-foreground">
            <kbd className="rounded bg-muted px-1.5 py-0.5">⌘ N</kbd>
            {/* TODO: show control for windows */}
          </div>
        </Button>

        <Button
          variant="outline"
          className="w-full justify-start px-2 text-foreground hover:bg-accent h-9 mb-2"
          onClick={handleNavigateTasks}
        >
          <CheckSquare className="w-4 h-4 mr-2 text-muted-foreground" />
          <span className="text-sm">Tasks</span>
        </Button>

        <div className="flex-1 overflow-y-auto scrollbar-thin scrollbar-thumb-gray-300 scrollbar-track-transparent pt-2">
          <AnimatePresence initial={false} mode="popLayout">
            <motion.div className="flex flex-col gap-2">
              {Object.entries(groupedChats).map(([title, groupChats]) =>
                renderGroup(title, groupChats)
              )}
            </motion.div>
          </AnimatePresence>

          {chats.length > 5 && (
            <motion.div>
              <Button
                variant="ghost"
                className="w-full justify-center text-xs text-muted-foreground hover:text-foreground h-8 mt-2 mb-1"
                onClick={() => setShowAllChats(!showAllChats)}
              >
                {showAllChats ? (
                  <>
                    <ChevronUp className="w-3.5 h-3.5 mr-1" /> Show less
                  </>
                ) : (
                  <>
                    <ChevronDown className="w-3.5 h-3.5 mr-1" /> Show more
                  </>
                )}
              </Button>
            </motion.div>
          )}
        </div>

        <div className="pt-2 border-t border-border/50 shrink-0">
          <Button
            variant="ghost"
            className="w-full justify-between px-2 text-secondary-foreground hover:text-foreground h-9 group"
            onClick={openSettings}
          >
            <div className="flex items-center gap-2">
              <SettingsIcon className="w-4 h-4 mr-2" />
              <span className="text-sm">Settings</span>
            </div>
            <div className="group-hover:opacity-100 transition-opacity opacity-0 flex items-center gap-2 text-[10px] text-muted-foreground">
              <kbd className="rounded bg-muted px-1.5 py-0.5">⌘ ,</kbd>
            </div>
          </Button>
        </div>
      </aside>
      <Omnibar />
    </>
  )
}

function SidebarItem({ chat, isActive }: { chat: Chat; isActive: boolean }) {
  const navigate = useNavigate()
  const router = useRouter()
  const [deleteChat] = useMutation(DeleteChatDocument, {
    refetchQueries: [GetChatsDocument],
    onError: (error) => {
      console.error(error)
    },
    onCompleted: async () => {
      await client.cache.evict({ fieldName: 'getChats' })
      await router.invalidate()
      if (isActive) {
        navigate({ to: '/' })
      }
    }
  })

  return (
    <motion.div
      key={chat.id}
      initial="visible"
      animate="visible"
      exit="exit"
      variants={{
        hidden: {
          opacity: 0
        },
        visible: {
          opacity: 1,
          transition: {
            duration: 1,
            ease: [0.2, 0.65, 0.4, 0.9]
          }
        },
        exit: {
          opacity: 0,
          transition: {
            duration: 0.25,
            ease: [0.4, 0, 0.6, 0]
          }
        }
      }}
      className={cn('flex items-center h-fit gap-2 justify-between rounded-md group text-sm', {
        'bg-primary/10 text-primary': isActive,
        'hover:bg-accent text-foreground': !isActive
      })}
    >
      <Link
        key={chat.id}
        disabled={isActive}
        to="/chat/$chatId"
        params={{ chatId: chat.id }}
        className={cn('block px-2 py-1.5 flex-1 truncate', {
          'text-primary font-medium': isActive,
          'text-foreground': !isActive
        })}
      >
        {chat.name || 'Untitled Chat'}
      </Link>
      {
        <AlertDialog>
          <AlertDialogTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="ml-1 opacity-0 group-hover:opacity-100 transition-opacity hover:bg-destructive/10 h-6 w-6"
            >
              <Trash2 className="w-3.5 h-3.5 text-destructive" />
            </Button>
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>Delete chat</AlertDialogTitle>
              <AlertDialogDescription>
                This action cannot be undone. It will permanently delete the chat.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>Do not delete</AlertDialogCancel>
              <Button
                variant="destructive"
                onClick={() => {
                  deleteChat({ variables: { chatId: chat.id } })
                }}
              >
                Delete
              </Button>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      }
    </motion.div>
  )
}
