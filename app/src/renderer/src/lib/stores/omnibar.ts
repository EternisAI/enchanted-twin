import { create } from 'zustand'

interface OmnibarStore {
  isOpen: boolean
  query: string
  setQuery: (query: string) => void
  openOmnibar: () => void
  closeOmnibar: () => void
  toggleOmnibar: () => void
}

export const useOmnibarStore = create<OmnibarStore>((set) => ({
  isOpen: false,
  query: '',
  setQuery: (query) => set({ query }),
  openOmnibar: () => set({ isOpen: true }),
  closeOmnibar: () => set({ isOpen: false, query: '' }),
  toggleOmnibar: () => set((state) => ({ isOpen: !state.isOpen }))
})) 