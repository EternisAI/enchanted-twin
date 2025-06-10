import { useState } from 'react'
import { Edit3, Check, X } from 'lucide-react'

import { Thread } from '@renderer/graphql/generated/graphql'
import { Button } from '../ui/button'
import { cn } from '@renderer/lib/utils'
import { useChatActions } from '@renderer/contexts/ChatContext'

export default function HolonThreadPreview({
  thread
}: {
  thread: Pick<Thread, 'id' | 'title' | 'content' | 'imageURLs' | 'actions'>
}) {
  const [isEditing, setIsEditing] = useState(false)
  const [editedTitle, setEditedTitle] = useState(thread.title)
  const [editedContent, setEditedContent] = useState(thread.content)

  const { sendMessage } = useChatActions()

  const handleSave = () => {
    console.log('Saving title:', editedTitle)
    console.log('Saving content:', editedContent)
    setIsEditing(false)
    const messageContent = `Please update the holon thread with the following title: ${editedTitle} and content: ${editedContent}`
    sendMessage(messageContent, false, false)
  }

  const handleCancel = () => {
    setEditedTitle(thread.title)
    setEditedContent(thread.content)
    setIsEditing(false)
  }

  return (
    <div
      key={thread.id}
      className={cn(
        'w-full bg-card border border-border rounded-lg p-3 flex flex-col gap-3 hover:bg-accent/5 transition-colors'
      )}
    >
      <div className="w-full flex items-start justify-between border-b border-border pb-3">
        <div className="flex flex-col flex-1">
          {isEditing ? (
            <div className="flex items-center gap-2">
              <input
                type="text"
                value={editedTitle}
                onChange={(e) => setEditedTitle(e.target.value)}
                className="text-lg font-semibold text-foreground bg-transparent border-b border-primary focus:outline-none flex-1"
                autoFocus
                onKeyDown={(e) => {
                  if (e.key === 'Enter') handleSave()
                  if (e.key === 'Escape') handleCancel()
                }}
              />
            </div>
          ) : (
            <div className="flex items-center gap-2 group">
              <h3 className="font-semibold text-foreground text-lg">{editedTitle}</h3>
              <Button
                variant="ghost"
                size="sm"
                className="transition-opacity"
                onClick={() => setIsEditing(true)}
              >
                <Edit3 className="w-4 h-4" />
              </Button>
            </div>
          )}
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <span className="font-semibold">You</span>
            {/* <span>•</span> */}
            {/* <span>{formatDistanceToNow(new Date(thread.createdAt), { addSuffix: true })}</span> */}
          </div>
        </div>
        {/* <Button
          variant="ghost"
          size="icon"
          className="text-muted-foreground"
          onClick={handleMoreClick}
        >
          <Maximize2 className="w-4 h-4" />
        </Button> */}
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

        {isEditing ? (
          <div className="flex flex-col gap-2">
            <textarea
              value={editedContent}
              onChange={(e) => setEditedContent(e.target.value)}
              className="text-foreground bg-transparent border border-primary rounded-lg p-3 focus:outline-none resize-none min-h-[120px]"
              onKeyDown={(e) => {
                if (e.key === 'Escape') handleCancel()
              }}
            />
            <div className="flex items-center gap-2">
              <Button variant="default" size="sm" onClick={handleSave}>
                <Check className="w-4 h-4 mr-1" />
                Save
              </Button>
              <Button variant="outline" size="sm" onClick={handleCancel}>
                <X className="w-4 h-4 mr-1" />
                Cancel
              </Button>
            </div>
          </div>
        ) : (
          <div className="group relative">
            <p className="text-foreground whitespace-pre-wrap">{editedContent}</p>
          </div>
        )}

        <div className="w-xl bg-transparent backdrop-blur-xs  pt-2">
          <div className="flex items-center gap-4 w-full">
            <Button
              variant="default"
              size="sm"
              onClick={() => {
                sendMessage('Send to Holon', false, false)
              }}
            >
              Send To Holon
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}
