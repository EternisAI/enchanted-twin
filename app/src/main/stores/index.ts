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
  global?: boolean  // Whether this shortcut should work system-wide
}

export interface KeyboardShortcutsStoreData {
  shortcuts: {
    [action: string]: KeyboardShortcut
  }
}

const defaultShortcuts = {
  toggleOmnibar: {
    keys: 'CommandOrControl+Alt+O',
    default: 'CommandOrControl+Alt+O',
    global: true  // This works system-wide
  },
  newChat: {
    keys: 'CommandOrControl+N',
    default: 'CommandOrControl+N',
    global: false  // App must be focused
  },
  toggleSidebar: {
    keys: 'CommandOrControl+S',
    default: 'CommandOrControl+S',
    global: false  // App must be focused
  },
  openSettings: {
    keys: 'CommandOrControl+,',
    default: 'CommandOrControl+,',
    global: false  // App must be focused
  }
}

export const keyboardShortcutsStore = new Store<KeyboardShortcutsStoreData>({
  name: 'keyboard-shortcuts',
  defaults: {
    shortcuts: defaultShortcuts
  },
  // Ensure the store is properly initialized
  clearInvalidConfig: true,
  migrations: {
    '>=0.0.1': (store) => {
      // Ensure all shortcuts have proper structure
      const shortcuts = store.get('shortcuts', {})
      const fixed: any = {}
      
      // For each default shortcut, ensure it exists with proper structure
      Object.entries(defaultShortcuts).forEach(([action, defaultShortcut]) => {
        const existing = shortcuts[action]
        if (!existing || typeof existing !== 'object' || !existing.default) {
          // If shortcut is missing or malformed, use defaults
          fixed[action] = {
            keys: (existing && existing.keys) || defaultShortcut.keys,
            default: defaultShortcut.default,
            global: defaultShortcut.global || false
          }
        } else {
          // Ensure global property exists
          fixed[action] = {
            ...existing,
            global: defaultShortcut.global || false
          }
        }
      })
      
      store.set('shortcuts', fixed)
    }
  }
})

// Add more stores here as needed
