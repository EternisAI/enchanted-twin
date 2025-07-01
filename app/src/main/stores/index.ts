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

export const omnibarStore = new Store({
  name: 'omnibar-settings',
  defaults: {
    position: { x: 0, y: 0 }, // Will be calculated on first show
    hasCustomPosition: false
  }
})

// Add more stores here as needed
