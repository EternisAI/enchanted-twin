import { create } from 'zustand'

interface VoiceStore {
  isVoiceMode: boolean

  startVoiceMode: (chatId: string) => void
  stopVoiceMode: () => void
}

export const useVoiceStore = create<VoiceStore>((set) => ({
  isVoiceMode: window.api.voiceStore.get('isVoiceMode') as boolean,

  startVoiceMode: (chatId: string) => {
    window.api.livekit.start(chatId)
    set({ isVoiceMode: true })
  },
  stopVoiceMode: () => {
    window.api.livekit.stop()
    set({ isVoiceMode: false })
  }
}))
