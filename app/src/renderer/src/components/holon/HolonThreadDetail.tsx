// import { useQuery } from '@apollo/client'
// import { GetThreadDocument } from '@renderer/graphql/generated/graphql'
import { Button } from '../ui/button'
import { formatDistanceToNow } from 'date-fns'
import { Eye, Maximize2 } from 'lucide-react'
import { useNavigate, useRouter } from '@tanstack/react-router'
import { motion } from 'framer-motion'
import {
  CreateChatDocument,
  SendMessageDocument,
  Thread
} from '@renderer/graphql/generated/graphql'
import { useMutation } from '@apollo/client'
import { useCallback } from 'react'
import { client } from '@renderer/graphql/lib'

interface HolonThreadDetailProps {
  thread: Thread
}

export default function HolonThreadDetail({ thread }: HolonThreadDetailProps) {
  const navigate = useNavigate()
  const router = useRouter()

  const [createChat] = useMutation(CreateChatDocument)
  const [sendMessage] = useMutation(SendMessageDocument)

  const handleCreateChat = useCallback(
    async (action: string) => {
      // This should check if chat exists or create one
      const chatId = `holon-${thread.id}`

      if (!chatId) return

      try {
        const { data: createData } = await createChat({
          variables: { name: chatId, voice: false }
        })
        const newChatId = createData?.createChat?.id

        if (newChatId) {
          navigate({
            to: '/chat/$chatId',
            params: { chatId: newChatId },
            search: { initialMessage: action, threadId: thread.id }
          })

          await client.cache.evict({ fieldName: 'getChats' })
          await router.invalidate({
            filter: (match) => match.routeId === '/chat/$chatId'
          })

          sendMessage({
            variables: {
              chatId: newChatId,
              text: action,
              reasoning: false,
              voice: false
            }
          })
        }
      } catch (error) {
        console.error('Failed to create chat:', error)
      }
    },
    [navigate, createChat, sendMessage, router, thread.id]
  )

  const handleBack = () => {
    navigate({ to: '/holon' })
  }

  const handleActionClick = (action: string) => {
    handleCreateChat(action)
  }

  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{ duration: 0.5 }}
      className="flex w-full h-full items-center overflow-y-auto flex flex-col gap-3 relative mb-18"
    >
      <div className="w-xl flex flex-col bg-gray-100 rounded-lg p-2">
        {/*  */}
        <div className="flex flex-col gap-6 bg-white rounded-lg p-3">
          {/* header  */}
          <div className="flex items-center justify-between border-b border-border pb-3">
            <div className="flex flex-col ">
              <h2 className="text-lg font-semibold text-foreground">{thread.title}</h2>
              <div className="flex items-center gap-2 text-sm text-muted-foreground">
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

          <p className="text-foreground whitespace-pre-wrap leading-relaxed text-base">
            {thread.content}
          </p>
        </div>
      </div>

      <div className="flex flex-col gap-3 w-xl">
        <div className="px-2 flex w-full gap-6 text-sm text-muted-foreground">
          <div className="flex items-center gap-2">
            <Eye className="w-4 h-4" />
            <span>Read by {thread.views}</span>
          </div>
          <div>{thread.messages.length} messages</div>
          {thread.expiresAt && (
            <div className="text-orange-500">
              Expires {formatDistanceToNow(new Date(thread.expiresAt), { addSuffix: true })}
            </div>
          )}
        </div>

        {thread.messages.length > 0 && (
          <div className="flex flex-col gap-2">
            {thread.messages.map((message) => (
              <div
                key={message.id}
                className="flex flex-col gap-1 border border-border rounded-lg py-3 px-4 bg-card hover:bg-accent/10 transition-colors"
              >
                <div className="flex items-center gap-2 text-primary">
                  <span className="font-semibold">
                    {message.author.alias || message.author.identity}
                  </span>
                  <span className="text-muted-foreground text-sm">•</span>
                  <span className="text-muted-foreground text-sm">
                    {formatDistanceToNow(new Date(message.createdAt), {
                      addSuffix: true
                    })}
                  </span>
                </div>
                <p className="text-sm text-foreground leading-relaxed">{message.content}</p>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="sticky w-xl bottom-0 left-0 right-0 bg-transparent backdrop-blur-xs border-t border-white/20 p-4">
        <div className="flex justify-center items-center gap-4 w-full">
          {thread.actions?.map((action, index) => (
            <Button
              key={index}
              variant={index === 0 ? 'default' : 'outline'}
              size="sm"
              onClick={() => handleActionClick(action)}
            >
              {action}
            </Button>
          ))}
        </div>
      </div>
    </motion.div>
  )
}
