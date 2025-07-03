import { create } from 'zustand'
import { useSidebarStore } from './sidebar'
import { auth } from '../firebase'
interface VoiceStore {
  isVoiceMode: boolean

  startVoiceMode: (chatId: string, isOnboarding?: boolean) => void
  stopVoiceMode: () => void
}

export const useVoiceStore = create<VoiceStore>((set) => ({
  isVoiceMode: window.api.voiceStore.get('isVoiceMode') as boolean,

  startVoiceMode: async (chatId: string, isOnboarding?: boolean) => {
    const token = await auth.currentUser?.getIdToken()
    await window.api.livekit.stop()
    window.api.livekit.start(chatId, isOnboarding, token)
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
