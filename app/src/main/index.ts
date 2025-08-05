// Load environment variables from .env file
import 'dotenv/config'

import Logger from 'electron-log'
import { app, session, globalShortcut } from 'electron'
import { electronApp, is, optimizer } from '@electron-toolkit/utils'
import log from 'electron-log/main'
import { registerNotificationIpc } from './notifications'
import { registerMediaPermissionHandlers, registerPermissionIpc } from './mediaPermissions'
import {
  registerScreenpipeIpc,
  cleanupScreenpipe,
  cleanupScreenpipeSync,
  autoStartScreenpipeIfEnabled
} from './screenpipe'
import { registerAccessibilityIpc } from './accessibilityPermissions'
import { windowManager } from './windows'
import { registerIpcHandlers, registerShortcut } from './ipcHandlers'
import { registerBrowserIpc } from './features/browser/browserIpc'
import { setupMenu } from './menuSetup'
import { checkForUpdates, setupAutoUpdater } from './autoUpdater'
import { cleanupOAuthServer } from './oauthHandler'
import { cleanupGoServer } from './goServer'
import { startLiveKitSetup, cleanupLiveKitAgent } from './livekitManager'
import { initializeAnalytics } from './analytics'
import { keyboardShortcutsStore, voiceStore } from './stores'
import { rotateLog } from './logConfig'

const IS_PRODUCTION = process.env.IS_PROD_BUILD === 'true' || !is.dev

log.transports.file.maxSize = 1024 * 1024 * 10 // 10mb

log.transports.file.archiveLogFn = (logFile: Logger.LogFile) => rotateLog(logFile)

log.transports.file.level = 'info'
log.info(`Log file will be written to: ${log.transports.file.getFile().path}`)
log.info(`Running in ${IS_PRODUCTION ? 'production' : 'development'} mode`)

// Inject build-time environment variables into runtime process.env
// __APP_ENV__ is replaced with a JSON object by electron-vite at build time
declare const __APP_ENV__: Record<string, string>

for (const [key, val] of Object.entries(typeof __APP_ENV__ === 'object' ? __APP_ENV__ : {})) {
  if (
    !(key in process.env) &&
    (key.startsWith('TTS') || key.startsWith('STT') || key.startsWith('VITE_'))
  ) {
    process.env[key] = val
  }
}

// Function to register global shortcuts from store
function registerStoredShortcuts() {
  try {
    globalShortcut.unregisterAll()
    log.info('Unregistered all existing global shortcuts')

    // Get shortcuts from store (electron-store handles defaults automatically)
    const shortcuts = keyboardShortcutsStore.get('shortcuts')

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

  await setupAutoUpdater()
  checkForUpdates(true)

  const mainWindow = windowManager.createMainWindow()
  registerNotificationIpc(mainWindow)
  registerMediaPermissionHandlers(session.defaultSession)
  registerPermissionIpc()
  registerScreenpipeIpc()
  registerAccessibilityIpc()
  registerIpcHandlers()
  registerBrowserIpc(mainWindow)
  initializeAnalytics()

  setupMenu()

  registerStoredShortcuts()

  setupLiveKitCleanup(mainWindow)

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

  voiceStore.set('isVoiceMode', false)

  // Set the quitting flag so omnibar window can close properly
  windowManager.setAppQuitting(true)

  // Destroy omnibar window before quitting to prevent it from blocking the quit process
  if (windowManager.omnibarWindow && !windowManager.omnibarWindow.isDestroyed()) {
    log.info('Destroying omnibar window before quit')
    windowManager.omnibarWindow.destroy()
  }
  // Start cleanup of Screenpipe early (sync version for immediate effect)
  cleanupScreenpipeSync()
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

// Handle process termination signals for force quit scenarios
process.on('SIGINT', () => {
  log.info('Received SIGINT, cleaning up...')
  cleanupScreenpipeSync()
  process.exit(0)
})

process.on('SIGTERM', () => {
  log.info('Received SIGTERM, cleaning up...')
  cleanupScreenpipeSync()
  process.exit(0)
})

// Handle uncaught exceptions to ensure cleanup
process.on('uncaughtException', (error) => {
  log.error('Uncaught exception:', error)
  cleanupScreenpipeSync()
  process.exit(1)
})

// Handle unhandled promise rejections
process.on('unhandledRejection', (reason, promise) => {
  log.error('Unhandled rejection at:', promise, 'reason:', reason)
  // Don't exit on unhandled rejections, but log them
})

// Windows-specific: Handle CTRL+C and close events
if (process.platform === 'win32') {
  process.on('SIGHUP', () => {
    log.info('Received SIGHUP, cleaning up...')
    cleanupScreenpipeSync()
    process.exit(0)
  })
}

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
