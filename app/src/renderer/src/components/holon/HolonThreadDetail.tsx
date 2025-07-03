import { Button } from '../ui/button'
import { formatDistanceToNow } from 'date-fns'
import { Eye, Maximize2 } from 'lucide-react'
import { useNavigate, useRouter } from '@tanstack/react-router'
import { motion } from 'framer-motion'
import { ChatCategory, CreateChatDocument, Thread } from '@renderer/graphql/generated/graphql'
import { useMutation } from '@apollo/client'
import { useCallback } from 'react'
import { client } from '@renderer/graphql/lib'
import MessageInput from '../chat/MessageInput'

interface HolonThreadDetailProps {
  thread: Thread
}

export default function HolonThreadDetail({ thread }: HolonThreadDetailProps) {
  const navigate = useNavigate()
  const router = useRouter()

  const [createChat] = useMutation(CreateChatDocument)

  const handleCreateChat = useCallback(
    async (action: string) => {
      const chatId = `holon-${thread.id}`
      const text = `Send a message to this holon thread: ${action}`

      try {
        const { data: createData } = await createChat({
          variables: {
            name: chatId,
            category: ChatCategory.Holon,
            holonThreadId: thread.id,
            initialMessage: text
          }
        })
        const newChatId = createData?.createChat?.id

        if (newChatId) {
          navigate({
            to: '/chat/$chatId',
            params: { chatId: newChatId },
            search: { initialMessage: text, threadId: thread.id }
          })

          await client.cache.evict({ fieldName: 'getChats' })
          await router.invalidate({
            filter: (match) => match.routeId === '/chat/$chatId'
          })
        }
      } catch (error) {
        console.error('Failed to create chat:', error)
      }
    },
    [navigate, createChat, router, thread.id]
  )

  const handleBack = () => {
    router.history.back()
  }

  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{ duration: 0.5 }}
      className="flex w-full h-full items-center overflow-y-auto flex flex-col gap-3 relative mb-18"
    >
      <div className="w-xl flex flex-col bg-gray-100 dark:bg-gray-900 rounded-lg p-2">
        {/*  */}
        <div className="flex flex-col gap-6 bg-white dark:bg-gray-800 rounded-lg p-3">
          {/* header  */}
          <div className="flex items-center justify-between border-b border-border dark:border-gray-700 pb-3">
            <div className="flex flex-col ">
              <h2 className="text-lg font-semibold text-foreground dark:text-white">
                {thread.title}
              </h2>
              <div className="flex items-center gap-2 text-sm text-muted-foreground dark:text-gray-400">
                <span className="font-medium">{thread.author.alias || thread.author.identity}</span>
                <span>•</span>
                <span>{formatDistanceToNow(new Date(thread.createdAt), { addSuffix: true })}</span>
              </div>
            </div>
            <Button variant="ghost" size="icon" onClick={handleBack}>
              <Maximize2 className="w-4 h-4" />
            </Button>
          </div>

          {/* body */}
          {thread.imageURLs && thread.imageURLs.length > 0 && (
            <div className="grid gap-4">
              {thread.imageURLs.length === 1 ? (
                <img
                  src={thread.imageURLs[0]}
                  alt="Thread image"
                  className="w-full rounded-lg max-h-96 object-cover"
                />
              ) : (
                <div className="grid grid-cols-2 gap-4">
                  {thread.imageURLs.map((imageUrl, index) => (
                    <img
                      key={index}
                      src={imageUrl}
                      alt={`Thread image ${index + 1}`}
                      className="w-full h-48 rounded-lg object-cover"
                    />
                  ))}
                </div>
              )}
            </div>
          )}

          <p className="text-foreground dark:text-gray-200 whitespace-pre-wrap leading-relaxed text-base">
            {thread.content}
          </p>
        </div>
      </div>

      <div className="flex flex-col gap-3 w-xl">
        <div className="px-2 flex w-full gap-6 text-sm text-muted-foreground dark:text-gray-400">
          <div className="flex items-center gap-2">
            <Eye className="w-4 h-4" />
            <span>Read by {thread.views}</span>
          </div>
          <div>{thread.messages.length} messages</div>
          {thread.expiresAt && (
            <div className="text-orange-500 dark:text-orange-400">
              Expires {formatDistanceToNow(new Date(thread.expiresAt), { addSuffix: true })}
            </div>
          )}
        </div>

        {thread.messages.length > 0 && (
          <div className="flex flex-col gap-2">
            {thread.messages.map((message) => (
              <div
                key={message.id}
                className="flex flex-col gap-1 border border-border dark:border-gray-700 rounded-lg py-3 px-4 bg-card dark:bg-gray-800 hover:bg-accent/10 dark:hover:bg-gray-700/50 transition-colors"
              >
                <div className="flex items-center gap-2 text-primary dark:text-blue-400">
                  <span className="font-semibold">
                    {message.author.alias || message.author.identity}
                  </span>
                  <span className="text-muted-foreground dark:text-gray-500 text-sm">•</span>
                  <span className="text-muted-foreground dark:text-gray-500 text-sm">
                    {formatDistanceToNow(new Date(message.createdAt), {
                      addSuffix: true
                    })}
                  </span>
                </div>
                <p className="text-sm text-foreground dark:text-gray-200 leading-relaxed">
                  {message.content}
                </p>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="sticky w-xl bottom-0 left-0 right-0 bg-transparent backdrop-blur-xs border-t border-white/20 dark:border-gray-700/50 p-4">
        <div className="flex justify-center items-center gap-4 w-full">
          {/* {thread.actions?.map((action, index) => (
            <Button
              key={index}
              variant={index === 0 ? 'default' : 'outline'}
              size="sm"
              className="capitalize"
              onClick={() => handleActionClick(action)}
            >
              {action}
            </Button>
          ))} */}
          <MessageInput
            onSend={(text) => {
              handleCreateChat(text)
            }}
            voiceMode={false}
            onStop={() => {}}
            isWaitingTwinResponse={false}
            isReasonSelected={false}
            placeholder="Send to Holon Thread"
          />
        </div>
      </div>
    </motion.div>
  )
}
