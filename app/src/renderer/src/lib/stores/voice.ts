import { create } from 'zustand'
import { useSidebarStore } from './sidebar'

interface VoiceStore {
  isVoiceMode: boolean
  toggleVoiceMode: (toggleSidebar?: boolean) => void
  setVoiceMode: (isVoiceMode: boolean, toggleSidebar?: boolean) => void
}

export const useVoiceStore = create<VoiceStore>((set, get) => ({
  isVoiceMode: window.api.voiceStore.get('isVoiceMode') as boolean,
  toggleVoiceMode: (toggleSidebar = true) => {
    const { setOpen } = useSidebarStore.getState()
    const { isVoiceMode } = get()
    if (toggleSidebar) {
      setOpen(isVoiceMode)
    }
    const newVoiceMode = !isVoiceMode
    window.api.voiceStore.set('isVoiceMode', newVoiceMode)
    set(() => ({ isVoiceMode: newVoiceMode }))
    
    // Start/stop LiveKit agent based on voice mode
    if (newVoiceMode) {
      window.api.livekit.start()
    } else {
      window.api.livekit.stop()
    }
  },
  setVoiceMode: (isVoiceMode: boolean, toggleSidebar = true) => {
    if (toggleSidebar) {
      const { setOpen } = useSidebarStore.getState()
      setOpen(!isVoiceMode)
    }
    window.api.voiceStore.set('isVoiceMode', isVoiceMode)
    set({ isVoiceMode })
    
    // Start/stop LiveKit agent based on voice mode
    if (isVoiceMode) {
      window.api.livekit.start()
    } else {
      window.api.livekit.stop()
    }
  }
}))
