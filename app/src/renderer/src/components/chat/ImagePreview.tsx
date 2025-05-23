import { useState } from 'react'
import {
  Dialog,
  DialogTrigger,
  DialogContent,
  DialogTitle,
  DialogDescription,
  DialogOverlay,
  DialogPortal
} from '../ui/dialog'
import { VisuallyHidden } from '@radix-ui/react-visually-hidden'
import { cn } from '@renderer/lib/utils'

interface ImagePreviewProps {
  src: string
  alt?: string
  thumbClassName?: string
  className?: string
}

export default function ImagePreview({ src, alt, thumbClassName, className }: ImagePreviewProps) {
  const [open, setOpen] = useState(false)

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <img src={src} alt={alt} className={cn('cursor-zoom-in', thumbClassName)} />
      </DialogTrigger>
      <DialogPortal>
        <DialogOverlay onClick={() => setOpen(false)} />
        <DialogContent
          className={cn('p-0 bg-transparent border-none w-full max-w-none rounded-xl', className)}
          onClick={(e) => e.stopPropagation()}
        >
          <VisuallyHidden>
            <DialogTitle>{alt || 'Image preview'}</DialogTitle>
          </VisuallyHidden>
          <VisuallyHidden>
            <DialogDescription>Preview of the selected image</DialogDescription>
          </VisuallyHidden>
          <div className="flex items-center justify-center">
            <img src={src} alt={alt} className="object-contain rounded-xl" />
          </div>
        </DialogContent>
      </DialogPortal>
    </Dialog>
  )
}
