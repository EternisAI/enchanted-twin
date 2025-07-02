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

export interface KeyboardShortcut {
  keys: string
  default: string
}

export interface KeyboardShortcutsStoreData {
  shortcuts: {
    [action: string]: KeyboardShortcut
  }
}

export const keyboardShortcutsStore = new Store<KeyboardShortcutsStoreData>({
  name: 'keyboard-shortcuts',
  defaults: {
    shortcuts: {
      toggleOmnibar: {
        keys: 'CommandOrControl+Alt+O',
        default: 'CommandOrControl+Alt+O'
      }
    }
  },
  // Ensure the store is properly initialized
  clearInvalidConfig: true
})

// Add more stores here as needed
