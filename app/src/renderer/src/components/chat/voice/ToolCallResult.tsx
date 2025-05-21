import { useState } from 'react'
import { ToolCall } from '@renderer/graphql/generated/graphql'
import { motion } from 'framer-motion'
import { cn } from '@renderer/lib/utils'
import ImageGallery from './ImageGallery'
import ToolCallNotificationItem from './ToolCallNotificationItem'

interface ToolCallResultProps {
  toolCalls: ToolCall[]
}

export default function ToolCallResult({ toolCalls }: ToolCallResultProps) {
  const [selectedImages, setSelectedImages] = useState<{ images: string[]; index: number } | null>(
    null
  )
  const completedToolCalls = toolCalls.filter((tc) => tc.isCompleted && tc.result)

  if (completedToolCalls.length === 0) return null

  return (
    <div className="flex flex-col gap-4 mt-4">
      {completedToolCalls.map((toolCall) => {
        const imageUrls = toolCall.result?.imageUrls ?? []
        const IMAGE_WIDTH = 140
        const IMAGE_HEIGHT = 170
        const SHIFT_PERCENT = 0.33
        const SHIFT_AMOUNT = IMAGE_WIDTH * SHIFT_PERCENT

        return (
          <motion.div
            key={toolCall.id}
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            className="flex flex-col gap-2"
          >
            {imageUrls.length > 0 && (
              <>
                {imageUrls.length <= 2 ? (
                  <div className="flex flex-col gap-2">
                    {imageUrls.map((url, index) => (
                      <motion.img
                        key={url}
                        src={url}
                        alt={`Tool result ${index + 1}`}
                        className="rounded-lg object-cover cursor-pointer transition-transform hover:scale-105 w-full h-[170px]"
                        initial={{ opacity: 0, scale: 0.8 }}
                        animate={{ opacity: 1, scale: 1 }}
                        transition={{ delay: index * 0.1 }}
                        onClick={() =>
                          setSelectedImages({
                            images: imageUrls,
                            index
                          })
                        }
                      />
                    ))}
                  </div>
                ) : (
                  <div
                    className="relative"
                    style={{
                      width: `${IMAGE_WIDTH + SHIFT_AMOUNT * 2}px`, // accommodate shift
                      height: `${IMAGE_HEIGHT}px`
                    }}
                  >
                    {imageUrls.slice(0, 3).map((url, index) => {
                      const rotate = [-5, 0, 5][index]
                      const translateX = index * SHIFT_AMOUNT
                      return (
                        <motion.img
                          key={index}
                          src={url}
                          alt={`Tool result ${index + 1}`}
                          className={cn(
                            'absolute top-0 rounded-lg object-cover shadow-md transition-transform hover:scale-105 cursor-pointer'
                          )}
                          style={{
                            left: `${translateX}px`,
                            width: `${IMAGE_WIDTH}px`,
                            height: `${IMAGE_HEIGHT}px`,
                            transform: `rotate(${rotate}deg)`,
                            zIndex: index
                          }}
                          initial={{ opacity: 0, scale: 0.8 }}
                          animate={{ opacity: 1, scale: 1 }}
                          transition={{ delay: index * 0.1 }}
                          onClick={() =>
                            setSelectedImages({
                              images: imageUrls,
                              index
                            })
                          }
                        />
                      )
                    })}
                  </div>
                )}
              </>
            )}

            {toolCall.result?.content && (
              <motion.p
                className={cn('text-sm text-muted-foreground', imageUrls.length > 0 ? 'mt-2' : '')}
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
              >
                {toolCall.result.content}
              </motion.p>
            )}

            <ToolCallNotificationItem
              notification={{
                id: toolCall.id,
                createdAt: new Date(),
                title: 'Image Generation Complete',
                message: 'Your image has been successfully generated',
                image: imageUrls[0]
              }}
            />
          </motion.div>
        )
      })}

      {selectedImages && (
        <ImageGallery
          images={selectedImages.images}
          initialIndex={selectedImages.index}
          onClose={() => setSelectedImages(null)}
        />
      )}
    </div>
  )
}
