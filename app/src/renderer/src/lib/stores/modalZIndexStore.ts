import { create } from 'zustand'

const BASE_Z_INDEX = 1000 // Base z-index for the first modal layer
const Z_INDEX_INCREMENT = 10 // Increment for subsequent modal layers

interface ModalLayer {
  id: string // Unique ID for the modal instance
  baseZIndex: number
}

interface ModalZIndexState {
  modalStack: ModalLayer[]
  requestZIndex: (modalId: string) => number
  releaseZIndex: (modalId: string) => void
}

export const useModalZIndexStore = create<ModalZIndexState>((set, get) => ({
  modalStack: [],
  requestZIndex: (modalId) => {
    const stack = get().modalStack
    const highestCurrentZ =
      stack.length > 0 ? stack[stack.length - 1].baseZIndex : BASE_Z_INDEX - Z_INDEX_INCREMENT
    const newBaseZIndex = highestCurrentZ + Z_INDEX_INCREMENT

    set((state) => ({
      modalStack: [...state.modalStack, { id: modalId, baseZIndex: newBaseZIndex }]
    }))
    return newBaseZIndex
  },
  releaseZIndex: (modalId) => {
    set((state) => ({
      modalStack: state.modalStack.filter((m) => m.id !== modalId)
    }))
  }
})) 