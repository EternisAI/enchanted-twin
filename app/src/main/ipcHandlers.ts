import { app, dialog, ipcMain, nativeTheme, shell, globalShortcut } from 'electron'
import log from 'electron-log/main'
import path from 'path'
import fs from 'fs'
import { windowManager } from './windows'
import { openOAuthWindow } from './oauthHandler'
import { checkForUpdates } from './autoUpdater'
import { keyboardShortcutsStore } from './stores'
import { updateMenu } from './menuSetup'
// import { getKokoroState } from './kokoroManager'
import {
  setupLiveKitAgent,
  startLiveKitAgent,
  stopLiveKitAgent,
  isLiveKitAgentRunning,
  getLiveKitAgentState,
  isLiveKitSessionReady,
  muteLiveKitAgent,
  unmuteLiveKitAgent,
  getCurrentAgentState
} from './livekitManager'

const PATHNAME = 'input_data'

export function registerIpcHandlers() {
  ipcMain.on('ping', () => console.log('pong'))

  // Handle new chat creation from menu
  ipcMain.on('new-chat', () => {
    if (windowManager.mainWindow && !windowManager.mainWindow.isDestroyed()) {
      windowManager.mainWindow.webContents.send('new-chat')
    }
  })

  // Handle renderer ready state for navigation
  ipcMain.on('renderer-ready', () => {
    log.info('Renderer process is ready for navigation')
    windowManager.processPendingNavigation()
  })

  ipcMain.on('open-oauth-url', async (_, url, redirectUri) => {
    console.log('[Main] Opening OAuth window for:', url, 'with redirect:', redirectUri)
    openOAuthWindow(url, redirectUri)
  })

  ipcMain.handle('get-native-theme', () => {
    return nativeTheme.shouldUseDarkColors ? 'dark' : 'light'
  })

  ipcMain.handle('set-native-theme', (_, theme: 'system' | 'light' | 'dark') => {
    if (theme === 'system') {
      nativeTheme.themeSource = 'system'
    } else {
      nativeTheme.themeSource = theme
    }
    return nativeTheme.shouldUseDarkColors ? 'dark' : 'light'
  })

  ipcMain.handle('get-app-version', () => {
    return app.getVersion()
  })

  nativeTheme.on('updated', () => {
    const newTheme = nativeTheme.shouldUseDarkColors ? 'dark' : 'light'
    if (windowManager.mainWindow) {
      windowManager.mainWindow.webContents.send('native-theme-updated', newTheme)
    }
  })

  ipcMain.handle('select-directory', async () => {
    const result = await dialog.showOpenDialog({
      properties: ['openDirectory']
    })
    return result
  })

  ipcMain.handle(
    'select-files',
    async (_event, options?: { filters?: { name: string; extensions: string[] }[] }) => {
      const result = await dialog.showOpenDialog({
        properties: ['openFile'],
        filters: options?.filters
      })
      return result
    }
  )

  ipcMain.handle('copy-dropped-files', async (_event, filePaths) => {
    const fileStoragePath =
      process.env.NODE_ENV === 'development'
        ? path.join(app.getAppPath(), PATHNAME)
        : path.join(app.getPath('userData'), PATHNAME)

    if (!fs.existsSync(fileStoragePath)) {
      fs.mkdirSync(fileStoragePath, { recursive: true })
    }

    const savedFiles: string[] = []

    for (const filePath of filePaths) {
      const fileName = path.basename(filePath)
      const destinationPath = path.join(fileStoragePath, fileName)

      try {
        fs.copyFileSync(filePath, destinationPath)
        savedFiles.push(destinationPath)
      } catch (error) {
        console.error('File save error:', error)
      }
    }

    return savedFiles
  })

  ipcMain.handle('get-stored-files-path', () => {
    const appPath = app.getAppPath()
    return path.join(appPath, PATHNAME)
  })

  ipcMain.handle('restart-app', async () => {
    log.info('Restarting app manually')
    app.relaunch()
    app.exit(0)
  })

  ipcMain.handle('open-logs-folder', async () => {
    try {
      const logsPath = app.getPath('logs')
      log.info(`Opening logs folder: ${logsPath}`)
      await shell.openPath(logsPath)
      return true
    } catch (error) {
      log.error(`Failed to open logs folder: ${error}`, error)
      throw error
    }
  })

  ipcMain.handle('open-appdata-folder', async () => {
    try {
      const appDataPath = app.getPath('userData')
      log.info(`Opening app data folder: ${appDataPath}`)
      await shell.openPath(appDataPath)
      return true
    } catch (error) {
      log.error(`Failed to open app data folder: ${error}`, error)
      throw error
    }
  })

  ipcMain.handle('delete-app-data', async () => {
    try {
      const userDataPath = app.getPath('userData')
      const dbPath = path.join(userDataPath, 'db')
      const weaviatePath = path.join(userDataPath, 'weaviate')
      log.info(`Checking for database folder: ${dbPath}`)
      log.info(`Checking for weaviate folder: ${weaviatePath}`)

      let dbDeleted = false
      if (fs.existsSync(dbPath)) {
        log.info(`Deleting database folder: ${dbPath}`)
        fs.rmSync(dbPath, { recursive: true, force: true })
        log.info(`Successfully deleted database folder: ${dbPath}`)
        dbDeleted = true
      } else {
        log.info(`Database folder does not exist: ${dbPath}`)
      }

      let weaviateDeleted = false
      if (fs.existsSync(weaviatePath)) {
        log.info(`Deleting weaviate folder: ${weaviatePath}`)
        fs.rmSync(weaviatePath, { recursive: true, force: true })
        log.info(`Successfully deleted weaviate folder: ${weaviatePath}`)
        weaviateDeleted = true
      } else {
        log.info(`Weaviate folder does not exist: ${weaviatePath}`)
      }

      const logsPath = app.getPath('logs')
      const mainLogPath = path.join(logsPath, 'main.log')
      log.info(`Checking for main.log: ${mainLogPath}`)

      let logDeleted = false
      if (fs.existsSync(mainLogPath)) {
        try {
          fs.unlinkSync(mainLogPath)
          log.info(`Successfully deleted main.log`)
          logDeleted = true
        } catch (err) {
          log.error(`Failed to delete main.log: ${err}`)
        }
      } else {
        log.info(`main.log does not exist: ${mainLogPath}`)
      }

      return dbDeleted || weaviateDeleted || logDeleted
    } catch (error) {
      log.error(`Failed to delete application data: ${error}`, error)
      throw error
    }
  })

  ipcMain.handle('isPackaged', () => {
    return app.isPackaged
  })

  ipcMain.handle('check-for-updates', async (_, silent = false) => {
    log.info(`Manual update check requested (silent: ${silent})`)
    checkForUpdates(silent)
    return true
  })

  ipcMain.handle('launch-get-current-state', async () => {
    try {
      const livekitState = await getLiveKitAgentState()
      return livekitState
    } catch (error) {
      log.error('Failed to get current launch state:', error)
      return null
    }
  })

  // LiveKit Agent IPC handlers
  ipcMain.handle('livekit:setup', async () => {
    try {
      await setupLiveKitAgent()
      return { success: true }
    } catch (error) {
      log.error('Failed to setup LiveKit agent:', error)
      return { success: false, error: error instanceof Error ? error.message : 'Unknown error' }
    }
  })

  ipcMain.handle('livekit:start', async (_, chatId: string, isOnboarding = false) => {
    try {
      await startLiveKitAgent(chatId, isOnboarding)
      return { success: true }
    } catch (error) {
      log.error('Failed to start LiveKit agent:', error)
      return { success: false, error: error instanceof Error ? error.message : 'Unknown error' }
    }
  })

  ipcMain.handle('livekit:stop', async () => {
    try {
      await stopLiveKitAgent()
      return { success: true }
    } catch (error) {
      log.error('Failed to stop LiveKit agent:', error)
      return { success: false, error: error instanceof Error ? error.message : 'Unknown error' }
    }
  })

  ipcMain.handle('livekit:is-running', () => {
    return isLiveKitAgentRunning()
  })

  ipcMain.handle('livekit:is-session-ready', () => {
    return isLiveKitSessionReady()
  })

  ipcMain.handle('livekit:get-state', async () => {
    try {
      const state = await getLiveKitAgentState()
      return state
    } catch (error) {
      log.error('Failed to get LiveKit agent state:', error)
      return null
    }
  })

  ipcMain.handle('livekit:mute', () => {
    try {
      return muteLiveKitAgent()
    } catch (error) {
      log.error('Failed to mute LiveKit agent:', error)
      return false
    }
  })

  ipcMain.handle('livekit:unmute', () => {
    try {
      return unmuteLiveKitAgent()
    } catch (error) {
      log.error('Failed to unmute LiveKit agent:', error)
      return false
    }
  })

  ipcMain.handle('livekit:get-agent-state', () => {
    try {
      return getCurrentAgentState()
    } catch (error) {
      log.error('Failed to get LiveKit agent state:', error)
      return 'idle' as const
    }
  })

  ipcMain.handle(
    'open-main-window-with-chat',
    async (_, chatId?: string, initialMessage?: string) => {
      try {
        log.info(`Opening main window with chat: ${chatId}, message: ${initialMessage}`)
        let windowWasCreated = false

        // Create main window if it doesn't exist
        if (!windowManager.mainWindow || windowManager.mainWindow.isDestroyed()) {
          log.info('Creating new main window')
          windowManager.createMainWindow()
          windowWasCreated = true
        } else {
          log.info('Using existing main window')
        }

        // Show and focus the window
        if (windowManager.mainWindow) {
          windowManager.mainWindow.show()
          windowManager.mainWindow.focus()

          if (chatId) {
            const url = initialMessage
              ? `/chat/${chatId}?initialMessage=${encodeURIComponent(initialMessage)}`
              : `/chat/${chatId}`

            log.info(`Navigation URL: ${url}`)

            if (windowWasCreated) {
              // Store the navigation to be processed when renderer is ready
              log.info('Storing pending navigation for new window')
              windowManager.setPendingNavigation(url)
            } else {
              // Window already exists, navigate immediately
              log.info('Navigating immediately on existing window')
              windowManager.mainWindow.webContents.send('navigate-to', url)
            }
          }

          return { success: true }
        }
        return { success: false, error: 'Failed to create main window' }
      } catch (error) {
        log.error('Failed to open main window with chat:', error)
        return { success: false, error: error instanceof Error ? error.message : 'Unknown error' }
      }
    }
  )

  ipcMain.handle('resize-omnibar-window', async (_, width: number, height: number) => {
    try {
      if (windowManager.omnibarWindow && !windowManager.omnibarWindow.isDestroyed()) {
        const minHeight = 80
        const maxHeight = 500
        const constrainedHeight = Math.max(minHeight, Math.min(height, maxHeight))
        windowManager.omnibarWindow.setSize(width, constrainedHeight)
        return { success: true }
      }
      return { success: false, error: 'Omnibar window not available' }
    } catch (error) {
      log.error('Failed to resize omnibar window:', error)
      return { success: false, error: error instanceof Error ? error.message : 'Unknown error' }
    }
  })

  ipcMain.handle('hide-omnibar-window', async () => {
    try {
      if (windowManager.omnibarWindow && !windowManager.omnibarWindow.isDestroyed()) {
        windowManager.omnibarWindow.hide()
        return { success: true }
      }
      return { success: false, error: 'Omnibar window not available' }
    } catch (error) {
      log.error('Failed to hide omnibar window:', error)
      return { success: false, error: error instanceof Error ? error.message : 'Unknown error' }
    }
  })

  // Keyboard shortcuts IPC handlers
  ipcMain.handle('keyboard-shortcuts:get', () => {
    try {
      const shortcuts = keyboardShortcutsStore.get('shortcuts')
      log.info('Getting keyboard shortcuts:', shortcuts)
      return shortcuts
    } catch (error) {
      log.error('Failed to get keyboard shortcuts:', error)
      // Return default shortcuts if store fails
      return {
        toggleOmnibar: {
          keys: 'CommandOrControl+Alt+O',
          default: 'CommandOrControl+Alt+O',
          global: true
        },
        newChat: {
          keys: 'CommandOrControl+N',
          default: 'CommandOrControl+N',
          global: false
        },
        toggleSidebar: {
          keys: 'CommandOrControl+S',
          default: 'CommandOrControl+S',
          global: false
        },
        openSettings: {
          keys: 'CommandOrControl+,',
          default: 'CommandOrControl+,',
          global: false
        }
      }
    }
  })

  ipcMain.handle('keyboard-shortcuts:set', (_, action: string, keys: string) => {
    try {
      // Handle empty string (removing shortcut)
      if (!keys) {
        const shortcuts = keyboardShortcutsStore.get('shortcuts')
        
        // Unregister the old shortcut
        if (shortcuts[action] && shortcuts[action].keys) {
          try {
            globalShortcut.unregister(shortcuts[action].keys)
          } catch (err) {
            log.warn(`Failed to unregister shortcut: ${shortcuts[action].keys}`)
          }
        }

        // Clear the shortcut
        if (!shortcuts[action]) {
          shortcuts[action] = { keys: '', default: '' }
        }
        shortcuts[action] = {
          ...shortcuts[action],
          keys: ''
        }
        keyboardShortcutsStore.set('shortcuts', shortcuts)
        
        log.info(`Removed shortcut for ${action}`)
        
        // Update the menu to reflect the removed shortcut
        updateMenu()
        
        return { success: true }
      }

      // Validate the shortcut keys
      if (keys.includes('Dead') || keys.includes('Process') || keys.includes('Unidentified')) {
        return { success: false, error: 'Invalid key combination' }
      }

      const shortcuts = keyboardShortcutsStore.get('shortcuts')
      
      // Unregister the old shortcut (only for global shortcuts)
      if (shortcuts[action] && shortcuts[action].keys && shortcuts[action].global) {
        try {
          globalShortcut.unregister(shortcuts[action].keys)
        } catch (err) {
          log.warn(`Failed to unregister old shortcut: ${shortcuts[action].keys}`)
        }
      }

      // Test if the new shortcut can be registered (only for global shortcuts)
      if (shortcuts[action] && shortcuts[action].global) {
        const testRegister = globalShortcut.register(keys, () => {})
        if (!testRegister) {
          return { success: false, error: 'This key combination cannot be registered or is already in use' }
        }
        globalShortcut.unregister(keys)
      }

      // Update the stored shortcut
      if (!shortcuts[action]) {
        shortcuts[action] = { keys: '', default: '' }
      }
      shortcuts[action] = {
        ...shortcuts[action],
        keys
      }
      keyboardShortcutsStore.set('shortcuts', shortcuts)
      
      // Log the updated shortcuts
      log.info(`Updated shortcut for ${action} to ${keys}`)
      log.info('Current shortcuts:', keyboardShortcutsStore.get('shortcuts'))

      // Register the new shortcut
      registerShortcut(action, keys, shortcuts[action]?.global || false)
      
      // Update the menu to reflect the new shortcut
      updateMenu()

      return { success: true }
    } catch (error) {
      log.error('Failed to set keyboard shortcut:', error)
      return { success: false, error: error instanceof Error ? error.message : 'Unknown error' }
    }
  })

  ipcMain.handle('keyboard-shortcuts:reset', (_, action: string) => {
    try {
      const shortcuts = keyboardShortcutsStore.get('shortcuts')
      
      if (!shortcuts[action]) {
        return { success: false, error: 'Unknown shortcut action' }
      }

      // Unregister current shortcut (only for global shortcuts)
      if (shortcuts[action] && shortcuts[action].global) {
        try {
          globalShortcut.unregister(shortcuts[action].keys)
        } catch (err) {
          log.warn(`Failed to unregister invalid shortcut: ${shortcuts[action].keys}`)
        }
      }

      // Reset to default
      shortcuts[action].keys = shortcuts[action].default
      keyboardShortcutsStore.set('shortcuts', shortcuts)

      // Register the default shortcut
      registerShortcut(action, shortcuts[action].default, shortcuts[action].global || false)
      
      // Update the menu to reflect the reset shortcut
      updateMenu()

      return { success: true }
    } catch (error) {
      log.error('Failed to reset keyboard shortcut:', error)
      return { success: false, error: error instanceof Error ? error.message : 'Unknown error' }
    }
  })

  ipcMain.handle('keyboard-shortcuts:reset-all', () => {
    try {
      const shortcuts = keyboardShortcutsStore.get('shortcuts')
      
      // Unregister all current shortcuts (only global shortcuts)
      Object.keys(shortcuts).forEach(action => {
        if (shortcuts[action] && shortcuts[action].global) {
          try {
            globalShortcut.unregister(shortcuts[action].keys)
          } catch (err) {
            log.warn(`Failed to unregister invalid shortcut for ${action}: ${shortcuts[action].keys}`)
          }
        }
      })

      // Reset all to defaults
      Object.keys(shortcuts).forEach(action => {
        shortcuts[action].keys = shortcuts[action].default
      })
      keyboardShortcutsStore.set('shortcuts', shortcuts)

      // Re-register all shortcuts
      Object.keys(shortcuts).forEach(action => {
        registerShortcut(action, shortcuts[action].keys, shortcuts[action].global || false)
      })
      
      // Update the menu to reflect all reset shortcuts
      updateMenu()

      return { success: true }
    } catch (error) {
      log.error('Failed to reset all keyboard shortcuts:', error)
      return { success: false, error: error instanceof Error ? error.message : 'Unknown error' }
    }
  })
}

// Helper function to register shortcuts
export function registerShortcut(action: string, keys: string, isGlobal: boolean = false) {
  try {
    // Don't register empty shortcuts
    if (!keys) {
      log.info(`Skipping registration for ${action}: no keys set`)
      return
    }
    
    // Only register global shortcuts in the main process
    if (isGlobal) {
      let handler: () => void
      
      switch (action) {
        case 'toggleOmnibar':
          handler = () => {
            log.info('Global shortcut triggered: Toggle Omnibar Overlay')
            windowManager.toggleOmnibarWindow()
          }
          break
          
        case 'newChat':
          handler = () => {
            log.info('Global shortcut triggered: New Chat')
            if (windowManager.mainWindow && !windowManager.mainWindow.isDestroyed()) {
              windowManager.mainWindow.webContents.send('new-chat')
            }
          }
          break
          
        case 'toggleSidebar':
          handler = () => {
            log.info('Global shortcut triggered: Toggle Sidebar')
            if (windowManager.mainWindow && !windowManager.mainWindow.isDestroyed()) {
              windowManager.mainWindow.webContents.send('toggle-sidebar')
            }
          }
          break
          
        case 'openSettings':
          handler = () => {
            log.info('Global shortcut triggered: Open Settings')
            if (windowManager.mainWindow && !windowManager.mainWindow.isDestroyed()) {
              windowManager.mainWindow.webContents.send('open-settings')
            }
          }
          break
          
        default:
          log.warn(`Unknown shortcut action: ${action}`)
          return
      }
      
      const registered = globalShortcut.register(keys, handler)
      
      if (registered) {
        log.info(`Successfully registered global shortcut for ${action}: ${keys}`)
      } else {
        log.error(`Failed to register global shortcut for ${action}: ${keys} (may be in use by system)`)
      }
    } else {
      log.info(`Skipping global registration for ${action} (handled locally in renderer)`)
    }
  } catch (error) {
    log.error(`Failed to register shortcut for ${action}:`, error)
  }
}
