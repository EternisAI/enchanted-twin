import { create } from 'zustand'
import { useSidebarStore } from './sidebar'
interface VoiceStore {
  isVoiceMode: boolean

  startVoiceMode: (chatId: string) => void
  stopVoiceMode: () => void
}

export const useVoiceStore = create<VoiceStore>((set) => ({
  isVoiceMode: window.api.voiceStore.get('isVoiceMode') as boolean,

  startVoiceMode: (chatId: string) => {
    window.api.livekit.start(chatId)
    const { setOpen } = useSidebarStore.getState()
    setOpen(false)
    set({ isVoiceMode: true })
  },
  stopVoiceMode: () => {
    window.api.livekit.stop()
    const { setOpen } = useSidebarStore.getState()
    setOpen(true)
    set({ isVoiceMode: false })
  }
}))
