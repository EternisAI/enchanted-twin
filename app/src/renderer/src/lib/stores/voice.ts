import { create } from 'zustand'
import { useSidebarStore } from './sidebar'

interface VoiceStore {
  isVoiceMode: boolean
  toggleVoiceMode: (toggleSidebar?: boolean) => void
  setVoiceMode: (isVoiceMode: boolean, toggleSidebar?: boolean) => void
}

export const useVoiceStore = create<VoiceStore>((set, get) => ({
  isVoiceMode: false,
  toggleVoiceMode: (toggleSidebar = true) => {
    const { setOpen } = useSidebarStore.getState()
    const { isVoiceMode } = get()
    if (toggleSidebar) {
      setOpen(isVoiceMode)
    }
    set((state) => ({ isVoiceMode: !state.isVoiceMode }))
  },
  setVoiceMode: (isVoiceMode: boolean, toggleSidebar = true) => {
    if (toggleSidebar) {
      const { setOpen } = useSidebarStore.getState()
      setOpen(!isVoiceMode)
    }
    set({ isVoiceMode })
  }
}))
