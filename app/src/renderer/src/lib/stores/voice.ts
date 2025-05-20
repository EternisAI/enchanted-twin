import { create } from 'zustand'

interface VoiceStore {
  isVoiceMode: boolean
  toggleVoiceMode: () => void
}

export const useVoiceStore = create<VoiceStore>((set) => ({
  isVoiceMode: false,
  toggleVoiceMode: () => set((state) => ({ isVoiceMode: !state.isVoiceMode }))
}))
