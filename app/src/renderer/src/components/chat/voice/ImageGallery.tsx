import { useState } from 'react'
import { motion } from 'framer-motion'
import { Dialog, DialogContent } from '../../ui/dialog'
import { ChevronLeft, ChevronRight, X } from 'lucide-react'

interface ImageGalleryProps {
  images: string[]
  initialIndex?: number
  onClose: () => void
}

export default function ImageGallery({ images, initialIndex = 0, onClose }: ImageGalleryProps) {
  const [currentIndex, setCurrentIndex] = useState(initialIndex)

  const nextImage = () => {
    setCurrentIndex((prev) => (prev + 1) % images.length)
  }

  const previousImage = () => {
    setCurrentIndex((prev) => (prev - 1 + images.length) % images.length)
  }

  return (
    <Dialog open>
      <DialogContent className="min-w-full w-full h-full p-0 border-none bg-background">
        <button
          onClick={onClose}
          className="absolute top-6 right-6 text-primary hover:text-gray-300 transition-colors z-50"
        >
          <X className="h-8 w-8" />
        </button>

        <div className="relative w-full h-full flex items-center justify-center">
          <button
            onClick={previousImage}
            className="absolute left-6 text-primary hover:text-gray-300 transition-colors z-50"
          >
            <ChevronLeft className="h-12 w-12" />
          </button>

          <motion.img
            key={currentIndex}
            src={images[currentIndex]}
            alt={`Gallery image ${currentIndex + 1}`}
            className="h-180 w-fit object-contain rounded-lg"
            initial={{ opacity: 0, scale: 0.9 }}
            animate={{ opacity: 1, scale: 1 }}
            exit={{ opacity: 0, scale: 0.9 }}
            transition={{ duration: 0.2 }}
          />

          <button
            onClick={nextImage}
            className="absolute right-6 text-primary hover:text-gray-300 transition-colors z-50"
          >
            <ChevronRight className="h-12 w-12" />
          </button>

          <div className="absolute bottom-6 left-1/2 -translate-x-1/2 text-primary text-base z-50">
            {currentIndex + 1} / {images.length}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
