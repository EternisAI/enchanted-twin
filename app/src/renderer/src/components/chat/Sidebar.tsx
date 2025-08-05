import { Link, useNavigate, useRouter, useRouterState } from '@tanstack/react-router'
import { useCallback, useState, useEffect } from 'react'

import {
  Chat,
  ChatCategory,
  DeleteChatDocument,
  GetChatsDocument
} from '@renderer/graphql/generated/graphql'
import { cn } from '@renderer/lib/utils'
import logo from '@resources/icon.png'
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
  Trash2,
  PanelLeftClose,
  PanelLeftOpen,
  SettingsIcon,
  SearchIcon,
  ChevronDown,
  ChevronUp,
  Globe,
  AlarmCheckIcon,
  HistoryIcon,
  SquarePen,
  Chrome
} from 'lucide-react'
import { useMutation } from '@apollo/client'
import { client } from '@renderer/graphql/lib'
import { isToday, isYesterday, isWithinInterval, subDays } from 'date-fns'
import { Tooltip, TooltipTrigger, TooltipContent, TooltipProvider } from '../ui/tooltip'
import { motion, AnimatePresence } from 'framer-motion'
import { useVoiceStore } from '@renderer/lib/stores/voice'
import { formatShortcutForDisplay } from '@renderer/lib/utils/shortcuts'
import { useOmnibarStore } from '@renderer/lib/stores/omnibar'
import { checkHolonsDisabled, checkTasksDisabled, checkBrowserEnabled } from '@renderer/lib/utils'

interface SidebarProps {
  chats: Chat[]
  setSidebarOpen: (open: boolean) => void
  shortcuts: Record<string, { keys: string; default: string; global?: boolean }>
  collapsed?: boolean
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

export function Sidebar({ chats, setSidebarOpen, shortcuts, collapsed = false }: SidebarProps) {
  const { location } = useRouterState()
  const navigate = useNavigate()
  const { openOmnibar } = useOmnibarStore()
  const { isVoiceMode, stopVoiceMode } = useVoiceStore()
  const [showAllChats, setShowAllChats] = useState(false)
  const isHolonsDisabled = checkHolonsDisabled()
  const isTasksDisabled = checkTasksDisabled()
  const isBrowserEnabled = checkBrowserEnabled()

  const handleNewChat = () => {
    if (isVoiceMode) {
      stopVoiceMode()
    }
    navigate({ to: '/', search: { focusInput: 'true' } })
  }

  const handleNavigateTasks = () => {
    navigate({ to: '/tasks' })
  }

  const chatsToDisplay = showAllChats ? chats : chats.slice(0, 5)
  const groupedChats = groupChatsByTime(chatsToDisplay)

  const renderGroup = useCallback(
    (title: string, groupChats: Chat[]) => {
      if (groupChats.length === 0 || collapsed) return null
      return (
        <motion.div
          key={title}
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
          <h3 className="text-xs font-medium text-sidebar-foreground/50 uppercase p-2">{title}</h3>
          {groupChats.map((chat) => (
            <SidebarItem
              key={chat.id}
              chat={chat}
              isActive={location.pathname === `/chat/${chat.id}`}
            />
          ))}
        </motion.div>
      )
    },
    [location.pathname, collapsed]
  )

  return (
    <>
      <aside
        className={cn(
          'flex flex-col text-sidebar-foreground h-full transition-all duration-300 relative',
          collapsed ? 'w-14 px-2 pt-10 pb-4 items-center' : 'w-64 p-4 px-2 pt-10 bg-sidebar '
        )}
      >
        <div className="flex items-center justify-between mb-4">
          <motion.div
            className="flex items-center"
            initial={{ opacity: 0, scale: 0.95 }}
            animate={{ opacity: 1, scale: 1 }}
            exit={{ opacity: 0, scale: 0.95 }}
            transition={{ duration: 0.5, ease: 'easeInOut' }}
          >
            {collapsed ? (
              <TooltipProvider>
                <Tooltip delayDuration={500}>
                  <TooltipTrigger asChild>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => setSidebarOpen(collapsed)}
                      className="hover:bg-sidebar-accent group size-10 text-foreground/60 hover:text-foreground"
                    >
                      <PanelLeftOpen className="w-4 h-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent side="right" align="center">
                    <div className="flex items-center gap-2">
                      <span>Open sidebar</span>
                      {shortcuts.toggleSidebar?.keys && (
                        <kbd className="text-[10px] text-primary-foreground/50 font-kbd">
                          {formatShortcutForDisplay(shortcuts.toggleSidebar.keys)}
                        </kbd>
                      )}
                    </div>
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
            ) : (
              <>
                <img src={logo} alt="logo" className="mx-2 w-6 h-6" />
                <span className="text-sm font-medium">Enchanted</span>
              </>
            )}
          </motion.div>
          {!collapsed && (
            <motion.div
              initial={{ opacity: 0, scale: 0.95 }}
              animate={{ opacity: 1, scale: 1 }}
              exit={{ opacity: 0, scale: 0.95 }}
              transition={{ duration: 0.5, ease: 'easeInOut' }}
            >
              <Tooltip delayDuration={300}>
                <TooltipTrigger asChild>
                  <Button
                    onClick={() => setSidebarOpen(false)}
                    variant="ghost"
                    size="icon"
                    className="text-sidebar-foreground/60 size-8 hover:text-sidebar-foreground hover:bg-sidebar-accent"
                  >
                    <PanelLeftClose className="w-4 h-4" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent side="right" align="center">
                  <div className="flex items-center gap-2">
                    <span>Collapse sidebar</span>
                    {shortcuts.toggleSidebar?.keys && (
                      <kbd className="text-[10px] text-primary-foreground/50 font-kbd">
                        {formatShortcutForDisplay(shortcuts.toggleSidebar.keys)}
                      </kbd>
                    )}
                  </div>
                </TooltipContent>
              </Tooltip>
            </motion.div>
          )}
        </div>
        <div className="flex flex-col gap-1 mb-2 items-center">
          <TooltipProvider>
            <Tooltip delayDuration={collapsed ? 300 : 1000}>
              <TooltipTrigger asChild>
                <Button
                  variant={collapsed ? 'outline' : 'ghost'}
                  size="icon"
                  className={cn(
                    'group transition-all',
                    collapsed ? 'p-0 justify-center ' : 'rounded-md w-full justify-start px-3'
                  )}
                  onClick={handleNewChat}
                >
                  <SquarePen
                    className={cn(
                      'w-4 h-4 transition-colors duration-100',
                      collapsed
                        ? 'w-5 h-5 text-foreground/60 group-hover:text-foreground'
                        : 'text-sidebar-foreground/60 group-hover:text-sidebar-foreground'
                    )}
                  />
                  {!collapsed && (
                    <>
                      <span className="text-sm">Chat</span>
                      {shortcuts.newChat?.keys && (
                        <div className="absolute right-2 group-hover:opacity-100 transition-opacity opacity-0 flex items-center gap-2 text-[10px] text-sidebar-foreground/60">
                          {formatShortcutForDisplay(shortcuts.newChat.keys)}
                        </div>
                      )}
                    </>
                  )}
                </Button>
              </TooltipTrigger>
              {collapsed && (
                <TooltipContent side="right" align="center">
                  <div className="flex items-center gap-2">
                    <span>New chat</span>
                    {shortcuts.newChat?.keys && (
                      <kbd className="text-[10px] text-primary-foreground/50 font-kbd">
                        {formatShortcutForDisplay(shortcuts.newChat.keys)}
                      </kbd>
                    )}
                  </div>
                </TooltipContent>
              )}
            </Tooltip>
          </TooltipProvider>

          <TooltipProvider>
            <Tooltip delayDuration={collapsed ? 300 : 1000}>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  className={cn(
                    'group h-9 transition-all',
                    collapsed
                      ? 'w-10 p-0 justify-center text-foreground hover:bg-accent'
                      : 'w-full justify-start px-2 text-sidebar-foreground'
                  )}
                  onClick={() => openOmnibar('Find a conversation...')}
                >
                  <SearchIcon
                    className={cn(
                      'w-4 h-4 transition-colors duration-100',
                      collapsed
                        ? 'w-5 h-5 text-foreground/60 group-hover:text-foreground'
                        : 'text-sidebar-foreground/60 group-hover:text-sidebar-foreground'
                    )}
                  />
                  {!collapsed && (
                    <>
                      <span className="text-sm">Search</span>
                      {shortcuts.search?.keys && (
                        <div className="absolute right-2 group-hover:opacity-100 transition-opacity opacity-0 flex items-center gap-2 text-[10px] text-sidebar-foreground/60">
                          {formatShortcutForDisplay(shortcuts.search.keys)}
                        </div>
                      )}
                    </>
                  )}
                </Button>
              </TooltipTrigger>
              {collapsed && (
                <TooltipContent side="right" align="center">
                  <div className="flex items-center gap-2">
                    <span>Search</span>
                    {shortcuts.search?.keys && (
                      <kbd className="text-[10px] text-primary-foreground/50 font-kbd">
                        {formatShortcutForDisplay(shortcuts.search.keys)}
                      </kbd>
                    )}
                  </div>
                </TooltipContent>
              )}
            </Tooltip>
          </TooltipProvider>
          {!isTasksDisabled && (
            <TooltipProvider>
              <Tooltip delayDuration={collapsed ? 300 : 1000}>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    data-active={location.pathname === '/tasks'}
                    className={cn(
                      'group h-9 transition-all',
                      collapsed
                        ? 'w-10 p-0 justify-center text-foreground hover:bg-accent [&[data-active=true]]:text-foreground [&[data-active=true]]:bg-accent'
                        : 'w-full justify-start px-2 text-sidebar-foreground hover:text-sidebar-accent-foreground [&[data-active=true]]:text-sidebar-accent-foreground [&[data-active=true]]:bg-sidebar-accent'
                    )}
                    onClick={handleNavigateTasks}
                  >
                    <AlarmCheckIcon
                      className={cn(
                        'w-4 h-4 transition-colors duration-100',
                        collapsed
                          ? 'w-5 h-5 text-foreground/60 group-hover:text-foreground'
                          : 'text-sidebar-foreground/60 group-hover:text-sidebar-foreground'
                      )}
                    />
                    {!collapsed && <span className="text-sm">Tasks</span>}
                  </Button>
                </TooltipTrigger>
                {collapsed && (
                  <TooltipContent side="right" align="center">
                    <span>Tasks</span>
                  </TooltipContent>
                )}
              </Tooltip>
            </TooltipProvider>
          )}

          {!isHolonsDisabled && (
            <TooltipProvider>
              <Tooltip delayDuration={collapsed ? 300 : 1000}>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    data-active={location.pathname === '/holon'}
                    className={cn(
                      'group h-9 transition-all',
                      collapsed
                        ? 'w-10 p-0 justify-center text-foreground hover:bg-accent [&[data-active=true]]:text-foreground [&[data-active=true]]:bg-accent'
                        : 'w-full justify-start px-2 text-sidebar-foreground hover:text-sidebar-accent-foreground [&[data-active=true]]:text-sidebar-accent-foreground [&[data-active=true]]:bg-sidebar-accent'
                    )}
                    onClick={() => navigate({ to: '/holon' })}
                  >
                    <Globe
                      className={cn(
                        'w-4 h-4 transition-colors duration-100',
                        collapsed
                          ? 'w-5 h-5 text-foreground/60 group-hover:text-foreground'
                          : 'text-sidebar-foreground/60 group-hover:text-sidebar-foreground'
                      )}
                    />
                    {!collapsed && <span className="text-sm">Holon Networks</span>}
                  </Button>
                </TooltipTrigger>
                {collapsed && (
                  <TooltipContent side="right" align="center">
                    <span>Holon Networks</span>
                  </TooltipContent>
                )}
              </Tooltip>
            </TooltipProvider>
          )}

          {isBrowserEnabled && (
            <TooltipProvider>
              <Tooltip delayDuration={collapsed ? 300 : 1000}>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    data-active={location.pathname === '/browser'}
                    className={cn(
                      'group h-9 transition-all',
                      collapsed
                        ? 'w-10 p-0 justify-center text-foreground hover:bg-accent [&[data-active=true]]:text-foreground [&[data-active=true]]:bg-accent'
                        : 'w-full justify-start px-2 text-sidebar-foreground hover:text-sidebar-accent-foreground [&[data-active=true]]:text-sidebar-accent-foreground [&[data-active=true]]:bg-sidebar-accent'
                    )}
                    onClick={() => navigate({ to: '/browser' })}
                  >
                    <Chrome
                      className={cn(
                        'w-4 h-4 transition-colors duration-100',
                        collapsed
                          ? 'w-5 h-5 text-foreground/60 group-hover:text-foreground'
                          : 'text-sidebar-foreground/60 group-hover:text-sidebar-foreground'
                      )}
                    />
                    {!collapsed && <span className="text-sm">Browser</span>}
                  </Button>
                </TooltipTrigger>
                {collapsed && (
                  <TooltipContent side="right" align="center">
                    <span>Browser</span>
                  </TooltipContent>
                )}
              </Tooltip>
            </TooltipProvider>
          )}
          {collapsed && (
            <TooltipProvider>
              <Tooltip delayDuration={collapsed ? 300 : 1000}>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    data-active={location.pathname === '/holon'}
                    className={cn(
                      'group h-9 transition-all',
                      collapsed
                        ? 'w-10 p-0 justify-center text-foreground hover:bg-accent [&[data-active=true]]:text-foreground [&[data-active=true]]:bg-accent'
                        : 'w-full justify-start px-2 text-sidebar-foreground hover:text-sidebar-accent-foreground [&[data-active=true]]:text-sidebar-accent-foreground [&[data-active=true]]:bg-sidebar-accent'
                    )}
                    onClick={() => setSidebarOpen(true)}
                  >
                    <HistoryIcon
                      className={cn(
                        'w-4 h-4 transition-colors duration-100',
                        collapsed
                          ? 'w-5 h-5 text-foreground/60 group-hover:text-foreground'
                          : 'text-sidebar-foreground/60 group-hover:text-sidebar-foreground'
                      )}
                    />
                    {!collapsed && <span className="text-sm">History</span>}
                  </Button>
                </TooltipTrigger>
                {collapsed && (
                  <TooltipContent side="right" align="center">
                    <span>History</span>
                  </TooltipContent>
                )}
              </Tooltip>
            </TooltipProvider>
          )}
        </div>

        {!collapsed && (
          <>
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
                    className="w-full justify-center text-xs text-sidebar-foreground/60 hover:text-sidebar-foreground h-8 mt-2 mb-1"
                    onClick={() => setShowAllChats(!showAllChats)}
                  >
                    {showAllChats ? (
                      <>
                        <ChevronUp className="w-3.5 h-3.5 mr-1" /> Show fewer
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
          </>
        )}

        <div className={cn('shrink-0', collapsed && 'mt-auto')}>
          <TooltipProvider>
            <Tooltip delayDuration={collapsed ? 300 : 1000}>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  className={cn(
                    'group h-9 transition-all',
                    collapsed
                      ? 'w-10 p-0 justify-center text-foreground hover:bg-accent'
                      : 'w-full justify-between px-2 text-sidebar-foreground hover:text-sidebar-accent-foreground'
                  )}
                  onClick={() => navigate({ to: '/settings' })}
                >
                  <div className={cn('flex items-center gap-2', collapsed && 'justify-center')}>
                    <SettingsIcon
                      className={cn(
                        'w-4 h-4',
                        collapsed ? 'w-5 h-5 text-foreground/60 group-hover:text-foreground' : ''
                      )}
                    />
                    {!collapsed && <span className="text-sm">Settings</span>}
                  </div>
                  {!collapsed && shortcuts.openSettings?.keys && (
                    <div className="group-hover:opacity-100 transition-opacity opacity-0 flex items-center gap-2 text-[10px] text-sidebar-foreground/60">
                      {formatShortcutForDisplay(shortcuts.openSettings.keys)}
                    </div>
                  )}
                </Button>
              </TooltipTrigger>
              {collapsed && (
                <TooltipContent side="right" align="center">
                  <div className="flex items-center gap-2">
                    <span>Settings</span>
                    {shortcuts.openSettings?.keys && (
                      <kbd className="text-[10px] text-primary-foreground/50 font-kbd">
                        {formatShortcutForDisplay(shortcuts.openSettings.keys)}
                      </kbd>
                    )}
                  </div>
                </TooltipContent>
              )}
            </Tooltip>
          </TooltipProvider>
        </div>
      </aside>
    </>
  )
}

function SidebarItem({ chat, isActive }: { chat: Chat; isActive: boolean }) {
  const navigate = useNavigate()
  const router = useRouter()
  const { startVoiceMode, stopVoiceMode } = useVoiceStore()
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
        'bg-sidebar-accent text-sidebar-accent-foreground font-medium': isActive,
        'hover:bg-sidebar-accent/50 text-sidebar-foreground/80': !isActive
      })}
    >
      <Link
        key={chat.id}
        disabled={isActive}
        to="/chat/$chatId"
        params={{ chatId: chat.id }}
        onClick={() => {
          if (chat.category === ChatCategory.Voice) {
            startVoiceMode(chat.id)
          } else {
            stopVoiceMode()
          }
          window.api.analytics.capture('open_chat', {
            method: 'ui'
          })
        }}
        className={cn('block px-2 py-1.5 flex-1 truncate', {
          'text-sidebar-accent-foreground font-medium': isActive,
          'text-sidebar-foreground/80': !isActive
        })}
      >
        {chat.name || 'Untitled Chat'}
      </Link>
      <AlertDialog>
        <AlertDialogTrigger asChild>
          <Button
            variant="ghost"
            size="icon"
            className="opacity-0 group-hover:opacity-100 transition-opacity hover:bg-destructive/10 h-6 w-6"
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
    </motion.div>
  )
}
