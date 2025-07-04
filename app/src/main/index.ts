// Load environment variables from .env file
import 'dotenv/config'

import { app, session, globalShortcut } from 'electron'
import { electronApp, is, optimizer } from '@electron-toolkit/utils'
import log from 'electron-log/main'
import { registerNotificationIpc } from './notifications'
import { registerMediaPermissionHandlers, registerPermissionIpc } from './mediaPermissions'
import {
  registerScreenpipeIpc,
  cleanupScreenpipe,
  autoStartScreenpipeIfEnabled
} from './screenpipe'
import { registerAccessibilityIpc } from './accessibilityPermissions'
import { windowManager } from './windows'
import { registerIpcHandlers, registerShortcut } from './ipcHandlers'
import { setupMenu } from './menuSetup'
import { setupAutoUpdater } from './autoUpdater'
import { cleanupOAuthServer } from './oauthHandler'
import { cleanupGoServer, initializeGoServer } from './goServer'
// import { startKokoro, cleanupKokoro } from './kokoroManager'
import { startLiveKitSetup, cleanupLiveKitAgent } from './livekitManager'
import { initializeAnalytics } from './analytics'
import { keyboardShortcutsStore } from './stores'

const DEFAULT_BACKEND_PORT = Number(process.env.DEFAULT_BACKEND_PORT) || 44999

// Check if running in production using environment variable
const IS_PRODUCTION = process.env.IS_PROD_BUILD === 'true' || !is.dev

// Configure electron-log
log.transports.file.level = 'info' // Log info level and above to file
log.info(`Log file will be written to: ${log.transports.file.getFile().path}`)
log.info(`Running in ${IS_PRODUCTION ? 'production' : 'development'} mode`)

// Inject build-time environment variables into runtime process.env
// __APP_ENV__ is replaced with a JSON object by electron-vite at build time
declare const __APP_ENV__: Record<string, string>

for (const [key, val] of Object.entries(typeof __APP_ENV__ === 'object' ? __APP_ENV__ : {})) {
  if (!(key in process.env) && (key.startsWith('TTS') || key.startsWith('STT'))) {
    process.env[key] = val
  }
}

// Function to register global shortcuts from store
function registerStoredShortcuts() {
  try {
    // First, unregister all existing shortcuts
    globalShortcut.unregisterAll()
    log.info('Unregistered all existing global shortcuts')
    
    // Get shortcuts from store (electron-store handles defaults automatically)
    const shortcuts = keyboardShortcutsStore.get('shortcuts')
    log.info('Loading keyboard shortcuts from store:', JSON.stringify(shortcuts, null, 2))

    // Register each shortcut
    Object.entries(shortcuts).forEach(([action, shortcut]) => {
      if (shortcut && shortcut.keys) {
        registerShortcut(action, shortcut.keys, shortcut.global || false)
      }
    })
  } catch (error) {
    log.error('Failed to register stored shortcuts:', error)
  }
}

app.whenReady().then(async () => {
  log.info(`App version: ${app.getVersion()}`)

  await initializeGoServer(IS_PRODUCTION, DEFAULT_BACKEND_PORT)

  const mainWindow = windowManager.createMainWindow()
  registerNotificationIpc(mainWindow)
  registerMediaPermissionHandlers(session.defaultSession)
  registerPermissionIpc()
  registerScreenpipeIpc()
  registerAccessibilityIpc()
  registerIpcHandlers()
  initializeAnalytics()

  setupAutoUpdater()
  setupMenu()

  // Register global shortcuts from store
  registerStoredShortcuts()

  // Setup LiveKit cleanup on renderer issues
  setupLiveKitCleanup(mainWindow)

  // startKokoro(mainWindow)
  startLiveKitSetup(mainWindow)
  autoStartScreenpipeIfEnabled()

  electronApp.setAppUserModelId('com.electron')

  app.on('browser-window-created', (_, window) => {
    optimizer.watchWindowShortcuts(window)
  })
})

app.on('window-all-closed', () => {
  // Check if only omnibar window is left and close it too
  if (windowManager.omnibarWindow && !windowManager.omnibarWindow.isDestroyed()) {
    windowManager.omnibarWindow.destroy()
  }

  if (process.platform !== 'darwin') {
    app.quit()
  }
})

app.on('activate', function () {
  // On macOS, re-create the main window when dock icon is clicked
  if (!windowManager.mainWindow || windowManager.mainWindow.isDestroyed()) {
    windowManager.createMainWindow()
  } else {
    windowManager.mainWindow.show()
    windowManager.mainWindow.focus()
  }
})

app.on('before-quit', () => {
  log.info('App before-quit event triggered')

  // Set the quitting flag so omnibar window can close properly
  windowManager.setAppQuitting(true)

  // Destroy omnibar window before quitting to prevent it from blocking the quit process
  if (windowManager.omnibarWindow && !windowManager.omnibarWindow.isDestroyed()) {
    log.info('Destroying omnibar window before quit')
    windowManager.omnibarWindow.destroy()
  }
})

app.on('will-quit', async () => {
  log.info('App will-quit event triggered')

  // Unregister all global shortcuts
  globalShortcut.unregisterAll()

  // Final cleanup of omnibar window if it still exists
  if (windowManager.omnibarWindow && !windowManager.omnibarWindow.isDestroyed()) {
    log.info('Force destroying omnibar window in will-quit')
    windowManager.omnibarWindow.destroy()
  }

  cleanupGoServer()
  cleanupOAuthServer()
  // await cleanupKokoro()
  await cleanupLiveKitAgent()
  cleanupScreenpipe()
})

// Simple rule: Non-voice mode = no process should live
function setupLiveKitCleanup(mainWindow: Electron.BrowserWindow) {
  // Any renderer issue = stop process (keep agent ready)
  mainWindow.webContents.on('render-process-gone', async (_event, details) => {
    log.error(`Renderer process gone: ${details.reason} - stopping LiveKit process`)
    const { stopLiveKitAgent } = await import('./livekitManager')
    await stopLiveKitAgent()
  })

  // Page refresh = stop process (keep agent ready)
  mainWindow.webContents.on(
    'did-start-navigation',
    async (_event, _url, isInPlace, isMainFrame) => {
      if (isMainFrame && isInPlace) {
        log.info('Page refresh - stopping LiveKit process')
        const { stopLiveKitAgent } = await import('./livekitManager')
        await stopLiveKitAgent()
      }
    }
  )

  // Window close = stop process (keep agent ready)
  mainWindow.on('close', async () => {
    log.info('Window closing - stopping LiveKit process')
    const { stopLiveKitAgent } = await import('./livekitManager')
    await stopLiveKitAgent()
  })
}
