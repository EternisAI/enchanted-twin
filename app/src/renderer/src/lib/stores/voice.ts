import { create } from 'zustand'
import { useSidebarStore } from './sidebar'
interface VoiceStore {
  isVoiceMode: boolean

  startVoiceMode: (chatId: string, isOnboarding?: boolean) => void
  stopVoiceMode: () => void
}

export const useVoiceStore = create<VoiceStore>((set) => ({
  isVoiceMode: window.api.voiceStore.get('isVoiceMode') as boolean,

  startVoiceMode: async (chatId: string, isOnboarding?: boolean) => {
    await window.api.livekit.stop()
    window.api.livekit.start(chatId, isOnboarding)
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
