import { useState } from 'react'
import {
  Dialog,
  DialogTrigger,
  DialogContent,
  DialogTitle,
  DialogDescription
} from '@renderer/components/ui/dialog'
import { VisuallyHidden } from '@radix-ui/react-visually-hidden'
import { cn } from '@renderer/lib/utils'

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
        <img src={src} alt={alt} className={cn('cursor-zoom-in', thumbClassName)} {...props} />
      </DialogTrigger>
      <DialogContent
        onClick={() => setOpen(false)}
        className={cn(
          'p-0 bg-transparent border-none w-full max-w-none rounded-xl max-h-[90vh]',
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
          className="flex items-center justify-center h-full"
          onClick={(e) => e.stopPropagation()}
        >
          <img src={src} alt={alt} className="object-contain rounded-xl" />
        </div>
      </DialogContent>
    </Dialog>
  )
}
