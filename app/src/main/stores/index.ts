import Store from 'electron-store'

export const voiceStore = new Store({
  name: 'voice-settings',
  defaults: {
    isVoiceMode: false
  }
})

// Add more stores here as needed
