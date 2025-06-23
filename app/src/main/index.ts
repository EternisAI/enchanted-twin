// Load environment variables from .env file
import 'dotenv/config'

import { app, BrowserWindow, session } from 'electron'
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
import { registerIpcHandlers } from './ipcHandlers'
import { setupMenu } from './menuSetup'
import { setupAutoUpdater } from './autoUpdater'
import { cleanupOAuthServer } from './oauthHandler'
import { cleanupGoServer, initializeGoServer } from './goServer'
// import { startKokoro, cleanupKokoro } from './kokoroManager'
import { startLiveKitSetup, cleanupLiveKitAgent } from './livekitManager'
import { initializeAnalytics } from './analytics'

const DEFAULT_BACKEND_PORT = Number(process.env.DEFAULT_BACKEND_PORT) || 44999

// Check if running in production using environment variable
const IS_PRODUCTION = process.env.IS_PROD_BUILD === 'true' || !is.dev

// Configure electron-log
log.transports.file.level = 'info' // Log info level and above to file
log.info(`Log file will be written to: ${log.transports.file.getFile().path}`)
log.info(`Running in ${IS_PRODUCTION ? 'production' : 'development'} mode`)

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

  // startKokoro(mainWindow)
  startLiveKitSetup(mainWindow)
  autoStartScreenpipeIfEnabled()

  electronApp.setAppUserModelId('com.electron')

  app.on('browser-window-created', (_, window) => {
    optimizer.watchWindowShortcuts(window)
  })
})

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') {
    app.quit()
  }
})

app.on('activate', function () {
  if (BrowserWindow.getAllWindows().length === 0) {
    windowManager.createMainWindow()
  }
})

app.on('will-quit', async () => {
  cleanupGoServer()
  cleanupOAuthServer()
  // await cleanupKokoro()
  await cleanupLiveKitAgent()
  cleanupScreenpipe()
})
