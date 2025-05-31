import Store from 'electron-store'

export const voiceStore = new Store({
  name: 'voice-settings',
  defaults: {
    isVoiceMode: false
  }
})

export const screenpipeStore = new Store({
  name: 'screenpipe-settings',
  defaults: {
    autoStart: false
  }
})

// Add more stores here as needed
