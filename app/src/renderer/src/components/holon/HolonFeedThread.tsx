import { formatDistanceToNow } from 'date-fns'
import { Eye, Maximize2 } from 'lucide-react'
import { useNavigate } from '@tanstack/react-router'

import { Thread } from '@renderer/graphql/generated/graphql'
import { Button } from '../ui/button'
import { cn } from '@renderer/lib/utils'

export default function HolonFeedItem({
  thread,
  collapsed = false
}: {
  thread: Thread
  collapsed?: boolean
}) {
  const navigate = useNavigate()

  const handleMoreClick = () => {
    console.log('thread.id', thread.id)
    navigate({ to: '/holon/$threadId', params: { threadId: thread.id } })
  }

  return (
    <div
      key={thread.id}
      className={cn(
        'bg-card border border-border rounded-lg p-3 flex flex-col gap-3 hover:bg-accent/5 transition-colors',
        collapsed && 'overflow-hidden max-h-[260px] relative pb-0'
      )}
    >
      <div className="flex items-start justify-between border-b border-border pb-3">
        <div className="flex flex-col ">
          <h3 className="font-semibold text-foreground text-lg">{thread.title}</h3>
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <span className="font-semibold">{thread.author.alias || thread.author.identity}</span>
            <span>•</span>
            <span>{formatDistanceToNow(new Date(thread.createdAt), { addSuffix: true })}</span>
          </div>
        </div>
        <Button
          variant="ghost"
          size="icon"
          className="text-muted-foreground"
          onClick={handleMoreClick}
        >
          <Maximize2 className="w-4 h-4" />
        </Button>
      </div>

      <div className="flex flex-col gap-2">
        {thread.imageURLs && thread.imageURLs.length > 0 && (
          <div className="grid gap-2">
            {thread.imageURLs.length === 1 ? (
              <img
                src={thread.imageURLs[0]}
                alt="Thread image"
                className="w-full rounded-lg max-h-96 object-cover"
              />
            ) : (
              <div className="grid grid-cols-2 gap-2">
                {thread.imageURLs.slice(0, 4).map((imageUrl, index) => (
                  <img
                    key={index}
                    src={imageUrl}
                    alt={`Thread image ${index + 1}`}
                    className="w-full h-32 rounded-lg object-cover"
                  />
                ))}
              </div>
            )}
          </div>
        )}

        <p className="text-foreground whitespace-pre-wrap">{thread.content}</p>
        {collapsed && (
          <div className="pointer-events-none absolute bottom-0 left-0 w-full h-20 bg-gradient-to-t from-card to-transparent" />
        )}
      </div>

      <div className="flex items-center justify-between pt-2 border-t border-border/50">
        <div className="flex items-center gap-4 text-sm text-muted-foreground">
          <div className="flex items-center gap-1">
            <Eye className="w-4 h-4" />
            <span>Read by {thread.views}</span>
          </div>
          <div>{thread.messages.length} messages</div>
        </div>

        {thread.expiresAt && (
          <div className="text-xs text-orange-500">
            Expires {formatDistanceToNow(new Date(thread.expiresAt), { addSuffix: true })}
          </div>
        )}
      </div>
    </div>
  )
}
