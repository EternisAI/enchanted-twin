import { app, BrowserWindow, session } from 'electron'
import { electronApp, is, optimizer } from '@electron-toolkit/utils'
import log from 'electron-log/main'
import { existsSync, mkdirSync } from 'fs'
import { join } from 'path'
import { registerNotificationIpc } from './notifications'
import { registerMediaPermissionHandlers, registerPermissionIpc } from './mediaPermissions'
import { registerScreenpipeIpc, cleanupScreenpipe } from './screenpipe'
import { registerAccessibilityIpc } from './accessibilityPermissions'
import { windowManager } from './windows'
import { registerIpcHandlers } from './ipcHandlers'
import { setupMenu } from './menuSetup'
import { setupAutoUpdater } from './autoUpdater'
import { cleanupOAuthServer } from './oauthHandler'
import { startGoServer, cleanupGoServer } from './goServer'
import { startKokoro, cleanupKokoro } from './kokoroManager'
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

  const userDataPath = app.getPath('userData')
  const dbDir = join(userDataPath, 'db')

  if (!existsSync(dbDir)) {
    try {
      mkdirSync(dbDir, { recursive: true })
      log.info(`Created database directory: ${dbDir}`)
    } catch (err) {
      log.error(`Failed to create database directory: ${err}`)
    }
  }

  const dbPath = join(dbDir, 'enchanted-twin.db')
  log.info(`Database path: ${dbPath}`)

  startKokoro(mainWindow)

  const executable = process.platform === 'win32' ? 'enchanted-twin.exe' : 'enchanted-twin'
  const goBinaryPath = !IS_PRODUCTION
    ? join(__dirname, '..', '..', 'resources', executable)
    : join(process.resourcesPath, 'resources', executable)

  // Only start the Go server in production environment
  if (IS_PRODUCTION) {
    await startGoServer(goBinaryPath, userDataPath, dbPath, DEFAULT_BACKEND_PORT)
  } else {
    log.info('Running in development mode - packaged Go server not started')
  }

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
  await cleanupKokoro()
  cleanupScreenpipe()
})
