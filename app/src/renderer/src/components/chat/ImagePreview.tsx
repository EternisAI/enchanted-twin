import { useState } from 'react'
import { Dialog, DialogTrigger, DialogContent } from '../ui/dialog'
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
        <img
          src={src}
          alt={alt}
          className={cn('cursor-zoom-in', thumbClassName)}
        />
      </DialogTrigger>
      <DialogContent className={cn('p-0 bg-transparent border-none max-w-fit', className)}>
        <img src={src} alt={alt} className="max-h-[80vh] max-w-[80vw] object-contain rounded" />
      </DialogContent>
    </Dialog>
  )
}
