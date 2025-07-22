import { create } from 'zustand'

const DEFAULT_PLACEHOLDER = 'Send a message privately…'

interface OmnibarStore {
  isOpen: boolean
  placeholder: string
  setPlaceholder: (placeholder: string) => void
  query: string
  setQuery: (query: string) => void
  openOmnibar: (placeholder?: string) => void
  closeOmnibar: () => void
  toggleOmnibar: (placeholder?: string) => void
}

export const useOmnibarStore = create<OmnibarStore>((set) => ({
  isOpen: false,
  placeholder: DEFAULT_PLACEHOLDER,
  setPlaceholder: (placeholder) => set({ placeholder }),
  query: '',
  setQuery: (query) => set({ query }),
  openOmnibar: (placeholder?: string) =>
    set({ isOpen: true, placeholder: placeholder || DEFAULT_PLACEHOLDER }),
  closeOmnibar: () => set({ isOpen: false, query: '' }),
  toggleOmnibar: (placeholder?: string) =>
    set((state) => ({
      isOpen: !state.isOpen,
      placeholder: placeholder || DEFAULT_PLACEHOLDER
    }))
}))
