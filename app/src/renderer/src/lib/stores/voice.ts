import { create } from 'zustand'
import { useSidebarStore } from './sidebar'

interface VoiceStore {
  isVoiceMode: boolean
  toggleVoiceMode: () => void
  setVoiceMode: (isVoiceMode: boolean) => void
}

export const useVoiceStore = create<VoiceStore>((set, get) => ({
  isVoiceMode: false,
  toggleVoiceMode: () => {
    const { setOpen } = useSidebarStore.getState()
    const { isVoiceMode } = get()
    setOpen(isVoiceMode)
    set((state) => ({ isVoiceMode: !state.isVoiceMode }))
  },
  setVoiceMode: (isVoiceMode: boolean) => {
    const { setOpen } = useSidebarStore.getState()
    setOpen(!isVoiceMode)
    set({ isVoiceMode })
  }
}))
