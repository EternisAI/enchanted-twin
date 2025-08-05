import { motion, AnimatePresence } from 'framer-motion'
import React, { useState, useCallback } from 'react'
import { ModalContext } from '@renderer/contexts/ModalContext'

export function ModalProvider({ children }: { children: React.ReactNode }) {
  const [modalContent, setModalContent] = useState<React.ReactNode | null>(null)
  const [closeOnBackdropClick, setCloseOnBackdropClick] = useState(true)

  const openModal = useCallback((content: React.ReactNode, allowBackdropClick: boolean = true) => {
    setCloseOnBackdropClick(allowBackdropClick)
    setModalContent(content)
  }, [])

  const closeModal = useCallback(() => {
    setModalContent(null)
  }, [])

  const handleBackdropClick = () => {
    if (closeOnBackdropClick) {
      closeModal()
    }
  }

  // Note: Route-based modal dismissal will be handled by a separate component
  // inside the router context to avoid hook ordering issues

  return (
    <ModalContext.Provider value={{ openModal, closeModal }}>
      {children}
      <AnimatePresence>
        {modalContent && (
          <>
            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              className="fixed inset-0 z-[100] bg-black/80 backdrop-blur-sm"
            />
            <motion.div
              initial={{ opacity: 0, scale: 0.95 }}
              animate={{ opacity: 1, scale: 1 }}
              exit={{ opacity: 0, scale: 0.95 }}
              className="fixed inset-0 z-[200] flex items-center justify-center p-4"
              onClick={handleBackdropClick}
            >
              <div
                className="bg-background max-h-[90vh] max-w-[90vw] overflow-hidden rounded-lg border shadow-lg"
                onClick={(e) => e.stopPropagation()}
              >
                {modalContent}
              </div>
            </motion.div>
          </>
        )}
      </AnimatePresence>
    </ModalContext.Provider>
  )
}
