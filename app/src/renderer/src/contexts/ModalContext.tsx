import React, { createContext } from 'react'

export interface ModalContextType {
  openModal: (content: React.ReactNode, closeOnBackdropClick?: boolean) => void
  closeModal: () => void
}

export const ModalContext = createContext<ModalContextType | undefined>(undefined)
