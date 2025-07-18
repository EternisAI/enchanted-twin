import { useState } from 'react'
import {
  Dialog,
  DialogTrigger,
  DialogContent,
  DialogTitle,
  DialogDescription,
  DialogPortal
} from '@renderer/components/ui/dialog'
import { VisuallyHidden } from '@radix-ui/react-visually-hidden'
import { cn } from '@renderer/lib/utils'
import { Button } from '@renderer/components/ui/button'
import { XIcon } from 'lucide-react'

interface ImagePreviewProps extends React.ImgHTMLAttributes<HTMLImageElement> {
  src: string
  alt?: string
  thumbClassName?: string
  className?: string
}

export default function ImagePreview({
  src,
  alt,
  thumbClassName,
  className,
  ...props
}: ImagePreviewProps) {
  const [open, setOpen] = useState(false)

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <img
          src={src}
          alt={alt}
          className={cn('cursor-zoom-in rounded-sm', thumbClassName)}
          {...props}
        />
      </DialogTrigger>
      <DialogPortal>
        <Button
          onClick={() => setOpen(false)}
          variant="ghost"
          size="icon"
          className="fixed top-4 right-4 z-[300] text-white hover:text-white/80"
        >
          <XIcon />
        </Button>
      </DialogPortal>
      <DialogContent
        hideCloseButton
        className={cn(
          'p-0 bg-transparent border-none w-full max-w-none rounded-xl max-h-[90vh] sm:max-w-[90vw] sm:max-h-[90vh] shadow-none focus-visible:ring-0 focus-visible:outline-none',
          className
        )}
      >
        <VisuallyHidden>
          <DialogTitle>{alt || 'Image preview'}</DialogTitle>
        </VisuallyHidden>
        <VisuallyHidden>
          <DialogDescription>Preview of the selected image</DialogDescription>
        </VisuallyHidden>
        <div
          className="flex items-center justify-center h-full w-full"
          onClick={(e) => e.stopPropagation()}
        >
          <img
            src={src}
            alt={alt}
            className="rounded-xl object-contain max-h-[90vh] max-w-[90vw]"
          />
        </div>
      </DialogContent>
    </Dialog>
  )
}
