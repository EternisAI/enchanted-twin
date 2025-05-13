import { app, shell, BrowserWindow, ipcMain, dialog, nativeTheme, Menu, session } from 'electron'
import { join } from 'path'
import { electronApp, optimizer, is } from '@electron-toolkit/utils'
import icon from '../../resources/icon.png?asset'
import { spawn, ChildProcess } from 'child_process'
import log from 'electron-log/main'
import { existsSync, mkdirSync } from 'fs'
import fs from 'fs'
import path from 'path'
import { autoUpdater } from 'electron-updater'
import http from 'http'
import { URL } from 'url'
import { createErrorWindow, createSplashWindow, waitForBackend } from './helpers'
import { registerNotificationIpc } from './notifications'
import { registerMediaPermissionHandlers, registerPermissionIpc } from './mediaPermissions'
import { registerScreenpipeIpc, installAndStartScreenpipe, cleanupScreenpipe } from './screenpipe'
import { registerAccessibilityIpc } from './accessibilityPermissions'
import { installPodman, startPodman, stopPodman } from './podman'

const PATHNAME = 'input_data'
const DEFAULT_OAUTH_SERVER_PORT = 8080
const DEFAULT_BACKEND_PORT = Number(process.env.DEFAULT_BACKEND_PORT) || 3000

let mainWindow: BrowserWindow | null = null
// Check if running in production using environment variable
const IS_PRODUCTION = process.env.IS_PROD_BUILD === 'true' || !is.dev

// Configure electron-log
log.transports.file.level = 'info' // Log info level and above to file
log.info(`Log file will be written to: ${log.transports.file.getFile().path}`)
log.info(`Running in ${IS_PRODUCTION ? 'production' : 'development'} mode`)

let goServerProcess: ChildProcess | null = null
let oauthServer: http.Server | null = null

let updateDownloaded = false

function startOAuthCallbackServer(callbackPath: string): Promise<http.Server> {
  return new Promise((resolve, reject) => {
    if (oauthServer) {
      oauthServer.close()
      oauthServer = null
    }

    const server = http.createServer((req, res) => {
      log.info(`[OAuth] Received request: ${req.url}`)

      if (req.url && req.url.startsWith(callbackPath)) {
        log.info(`[OAuth] Received callback: ${req.url}`)

        try {
          const parsedUrl = new URL(`http://localhost:${DEFAULT_OAUTH_SERVER_PORT}${req.url}`)
          const code = parsedUrl.searchParams.get('code')
          const state = parsedUrl.searchParams.get('state')

          if (code && state && mainWindow) {
            mainWindow.webContents.send('oauth-callback', { code, state })
            res.writeHead(200, { 'Content-Type': 'text/html' })
            res.end(`
              <!DOCTYPE html>
              <html>
                <head>
                  <title>Authentication Successful</title>
                  <style>
                    body { font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; text-align: center; padding: 40px; }
                    h1 { color: #333; }
                    p { color: #666; }
                    .success { color: #4CAF50; font-weight: bold; }
                  </style>
                </head>
                <body>
                  <h1>Authentication Successful</h1>
                  <p class="success">You have successfully authenticated!</p>
                  <p>You can close this window and return to the application.</p>
                </body>
              </html>
            `)

            setTimeout(() => {
              if (oauthServer) {
                oauthServer.close()
                oauthServer = null
                log.info('[OAuth] Callback server closed after successful authentication')
              }
            }, 3000)
          } else {
            res.writeHead(400, { 'Content-Type': 'text/html' })
            res.end(
              '<html><body><h1>Authentication Error</h1><p>Invalid or missing parameters.</p></body></html>'
            )
          }
        } catch (err) {
          log.error('[OAuth] Failed to parse callback URL', err)
          res.writeHead(500, { 'Content-Type': 'text/plain' })
          res.end('Internal Server Error')
        }
      } else {
        log.info(`[OAuth] Received non-callback request: ${req.url}`)
        res.writeHead(404, { 'Content-Type': 'text/plain' })
        res.end('Not Found')
      }
    })

    server.on('error', (err) => {
      log.error(`[OAuth] Server error on port ${DEFAULT_OAUTH_SERVER_PORT}:`, err)
      reject(err)
    })

    server.listen(DEFAULT_OAUTH_SERVER_PORT, '127.0.0.1', () => {
      log.info(
        `[OAuth] Callback server listening on http://127.0.0.1:${DEFAULT_OAUTH_SERVER_PORT}${callbackPath}`
      )
      resolve(server)
    })
  })
}

function openOAuthWindow(authUrl: string, redirectUri?: string) {
  let callbackPath = '/callback'
  if (redirectUri) {
    try {
      const parsedRedirect = new URL(redirectUri)
      callbackPath = parsedRedirect.pathname

      if (parsedRedirect.protocol === 'https:') {
        log.info('[OAuth] Using custom Electron window for HTTPS redirect')

        const authWindow = new BrowserWindow({
          width: 800,
          height: 600,
          show: true,
          webPreferences: {
            nodeIntegration: false,
            contextIsolation: true,
            webSecurity: true
          }
        })

        app.on('certificate-error', (event, webContents, _url, _error, _certificate, callback) => {
          if (webContents.id === authWindow.webContents.id) {
            log.info('[OAuth] Handling certificate error for auth window')
            event.preventDefault()
            callback(true)
          } else {
            callback(false)
          }
        })

        const filter = { urls: [parsedRedirect.origin + callbackPath + '*'] }

        authWindow.webContents.session.webRequest.onBeforeRequest(filter, (details, callback) => {
          log.info(`[OAuth] Intercepted request to: ${details.url}`)

          try {
            const parsedUrl = new URL(details.url)
            const code = parsedUrl.searchParams.get('code')
            const state = parsedUrl.searchParams.get('state')

            if (code && state && mainWindow) {
              mainWindow.webContents.send('oauth-callback', { code, state })

              authWindow.loadURL(
                'data:text/html,' +
                  encodeURIComponent(`
                <!DOCTYPE html>
                <html>
                  <head>
                    <title>Authentication Successful</title>
                    <style>
                      body { font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; text-align: center; padding: 40px; }
                      h1 { color: #333; }
                      p { color: #666; }
                      .success { color: #4CAF50; font-weight: bold; }
                    </style>
                  </head>
                  <body>
                    <h1>Authentication Successful</h1>
                    <p class="success">You have successfully authenticated!</p>
                    <p>You can close this window and return to the application.</p>
                  </body>
                </html>
              `)
              )

              setTimeout(() => {
                try {
                  if (!authWindow.isDestroyed()) {
                    authWindow.close()
                  }
                } catch (err) {
                  log.error('[OAuth] Error closing auth window:', err)
                }
              }, 3000)
            }
          } catch (err) {
            log.error('[OAuth] Error parsing callback URL:', err)
          }

          callback({})
        })

        authWindow.on('closed', () => {
          log.info('[OAuth] Auth window closed')
          app.removeAllListeners('certificate-error')
        })

        authWindow.webContents.on(
          'did-fail-load',
          (_, errorCode, errorDescription, validatedURL) => {
            log.error(
              `[OAuth] Failed to load URL: ${validatedURL}. Error: ${errorCode} - ${errorDescription}`
            )
          }
        )

        // Load the auth URL
        log.info(`[OAuth] Loading auth URL in BrowserWindow: ${authUrl}`)
        authWindow.loadURL(authUrl).catch((err) => {
          log.error('[OAuth] Error loading auth URL:', err)
        })

        return
      }

      startOAuthCallbackServer(callbackPath)
        .then((server) => {
          oauthServer = server
          log.info('[OAuth] Server started, opening auth URL in browser')
          return shell.openExternal(authUrl)
        })
        .then(() => {
          log.info('[OAuth] Opened auth URL in default browser')
        })
        .catch((err) => {
          log.error('[OAuth] Error in OAuth flow:', err)
        })
    } catch (err) {
      log.error('[OAuth] Failed to parse redirectUri', err)

      shell
        .openExternal(authUrl)
        .then(() => log.info('[OAuth] Opened auth URL in default browser'))
        .catch((err) => log.error('[OAuth] Failed to open auth URL in default browser', err))
    }
  } else {
    shell
      .openExternal(authUrl)
      .then(() => log.info('[OAuth] Opened auth URL in default browser'))
      .catch((err) => log.error('[OAuth] Failed to open auth URL in default browser', err))
  }
}

function setupAutoUpdater() {
  if (!IS_PRODUCTION) {
    log.info('Skipping auto-updater in development mode')
    return
  }

  autoUpdater.logger = log
  log.transports.file.level = 'debug'
  autoUpdater.autoDownload = true

  autoUpdater.on('checking-for-update', () => {
    log.info('Checking for update...')
    if (mainWindow) {
      mainWindow.webContents.send('update-status', 'Checking for update...')
    }
  })

  autoUpdater.on('update-available', (info) => {
    log.info('Update available:', info)
    if (mainWindow) {
      mainWindow.webContents.send('update-status', 'Update available, downloading...')
    }
  })

  autoUpdater.on('update-not-available', (info) => {
    log.info('Update not available:', info)

    if (mainWindow) {
      mainWindow.webContents.send('update-status', 'No updates available')
    }
  })

  autoUpdater.on('error', (err) => {
    log.error('Error in auto-updater:', err)

    if (mainWindow) {
      mainWindow.webContents.send('update-status', `Error: ${err.message}`)

      dialog.showErrorBox(
        'Update Error',
        `An error occurred while updating the application: ${err.message}`
      )
    }
  })

  autoUpdater.on('download-progress', (progressObj) => {
    let logMessage = `Download speed: ${progressObj.bytesPerSecond}`
    logMessage += ` - Downloaded ${progressObj.percent}%`
    logMessage += ` (${progressObj.transferred}/${progressObj.total})`
    log.info(logMessage)

    if (mainWindow) {
      mainWindow.webContents.send('update-progress', progressObj)
    }
  })

  autoUpdater.on('update-downloaded', (info) => {
    log.info('Update downloaded:', info)
    updateDownloaded = true
    mainWindow?.webContents.send('update-status', 'Update downloaded – ready to install')

    dialog
      .showMessageBox(mainWindow!, {
        type: 'info',
        title: 'Update Ready',
        message: `Version ${info.version} has been downloaded. Install and restart now?`,
        buttons: ['Install & Restart', 'Later'],
        defaultId: 0,
        cancelId: 1
      })
      .then(({ response }) => {
        if (response === 0) {
          log.info('User accepted update – restarting')
          // small delay so the dialog can close cleanly
          setTimeout(() => autoUpdater.quitAndInstall(true, true), 300)
        } else {
          log.info('User chose to install later')
        }
      })
  })
}

function checkForUpdates(silent = false) {
  log.info(`[checkForUpdates] Called with silent=${silent}`)

  if (updateDownloaded) {
    log.info(`[checkForUpdates] Update previously downloaded. Silent check: ${silent}`)
    if (silent) {
      log.info(
        '[checkForUpdates] Silent check found downloaded update. Initiating quit and install...'
      )
      autoUpdater.quitAndInstall(true, true)
    } else {
      if (mainWindow) {
        dialog
          .showMessageBox(mainWindow, {
            type: 'info',
            title: 'Install Updates',
            message: 'Updates downloaded previously are ready to be installed.',
            buttons: ['Install and Restart', 'Later'],
            defaultId: 0,
            cancelId: 1
          })
          .then(({ response }) => {
            if (response === 0) {
              log.info('User initiated app restart for previously downloaded update')
              autoUpdater.quitAndInstall(true, true)
            } else {
              log.info('User chose to install later.')
            }
          })
      } else {
        log.warn('Cannot prompt user to install update, mainWindow is not available.')
      }
    }
    return
  }
  log.info(`Checking for updates... (Silent: ${silent})`)
  if (mainWindow && !silent) {
    mainWindow.webContents.send('update-status', 'Checking for update...')
  }

  autoUpdater
    .checkForUpdates()
    .then((result) => {
      if (!result || !result.updateInfo || result.updateInfo.version === app.getVersion()) {
        if (mainWindow && !silent) {
          mainWindow.webContents.send('update-status', 'No updates available')
          dialog.showMessageBox(mainWindow, {
            type: 'info',
            title: 'No Updates',
            message: 'You are using the latest version of the application.',
            buttons: ['OK']
          })
        }
      }
    })
    .catch((err) => {
      log.error('Error checking for updates:', err)
      if (mainWindow && !silent) {
        mainWindow.webContents.send('update-status', `Error checking for updates: ${err.message}`)
        dialog.showErrorBox(
          'Update Check Error',
          `An error occurred while checking for updates: ${err.message}`
        )
      }
    })
}

function createWindow(): BrowserWindow {
  const mainWindow = new BrowserWindow({
    width: 1200,
    height: 800,
    show: false,
    titleBarStyle: 'hidden',
    autoHideMenuBar: true,
    ...(process.platform !== 'darwin' ? { titleBarOverlay: true } : {}),
    ...(process.platform === 'linux' ? { icon } : {}),
    webPreferences: {
      preload: join(__dirname, '../preload/index.js'),
      sandbox: false
    }
  })

  mainWindow.webContents.on('context-menu', (_, params) => {
    const menu = Menu.buildFromTemplate([
      {
        label: 'Toggle Developer Tools',
        click: () => {
          mainWindow.webContents.toggleDevTools()
        }
      }
    ])
    menu.popup({ window: mainWindow, x: params.x, y: params.y })
  })

  mainWindow.on('ready-to-show', () => {
    mainWindow.show()
  })

  mainWindow.webContents.setWindowOpenHandler((details) => {
    shell.openExternal(details.url)
    return { action: 'deny' }
  })

  // HMR for renderer base on electron-vite cli.
  // Load the remote URL for development or the local html file for production.
  if (!IS_PRODUCTION && process.env['ELECTRON_RENDERER_URL']) {
    mainWindow.loadURL(process.env['ELECTRON_RENDERER_URL'])
  } else {
    mainWindow.loadFile(join(__dirname, '../renderer/index.html'))
  }

  return mainWindow
}

app.whenReady().then(async () => {
  const splash = createSplashWindow()

  log.info(`App version: ${app.getVersion()}`)

  const executable = process.platform === 'win32' ? 'enchanted-twin.exe' : 'enchanted-twin'
  const goBinaryPath = !IS_PRODUCTION
    ? join(__dirname, '..', '..', 'resources', executable) // Path in development
    : join(process.resourcesPath, 'resources', executable) // Adjusted path in production

  // Set up auto-updater
  setupAutoUpdater()

  // Create the database directory in user data path
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

  // Install and start Podman
  try {
    log.info('Checking and installing Podman if needed')
    const podmanInstalled = await installPodman()
    if (podmanInstalled) {
      log.info('Starting Podman service')
      await startPodman()

      // Wait for Podman to fully start
      log.info('Waiting for Podman to be ready...')
      await new Promise((resolve) => setTimeout(resolve, 5000))

      // Log Podman version
      const podmanVersionProcess = spawn('podman', ['--version'])
      podmanVersionProcess.stdout.on('data', (data) => {
        log.info(`Podman version: ${data.toString().trim()}`)
      })
      podmanVersionProcess.stderr.on('data', (data) => {
        log.error(`Podman version error: ${data.toString().trim()}`)
      })
    } else {
      log.warn('Podman installation or initialization failed')
    }
  } catch (error) {
    log.error('Error setting up Podman:', error)
  }

  installAndStartScreenpipe().then((result) => {
    if (!result.success) {
      log.error(`Failed to install screenpipe: ${result.error}`)
      createErrorWindow(`Failed to install screenpipe: ${result.error}`)
      return
    }
  })

  // Only start the Go server in production environment
  if (IS_PRODUCTION) {
    if (!existsSync(goBinaryPath)) {
      log.error(`Go binary not found at: ${goBinaryPath}`)
      createErrorWindow(`Go binary not found at: ${goBinaryPath}`)
      return
    }
    log.info(`Attempting to start Go server at: ${goBinaryPath}`)

    try {
      goServerProcess = spawn(goBinaryPath, [], {
        env: {
          ...process.env,
          APP_DATA_PATH: userDataPath,
          DB_PATH: dbPath,
          OPENAI_BASE_URL: process.env.OPENAI_BASE_URL,
          COMPLETIONS_MODEL: process.env.COMPLETIONS_MODEL,
          EMBEDDINGS_API_URL: process.env.EMBEDDINGS_API_URL,
          EMBEDDINGS_MODEL: process.env.EMBEDDINGS_MODEL,
          TELEGRAM_TOKEN: process.env.TELEGRAM_TOKEN,
          OLLAMA_BASE_URL: process.env.OLLAMA_BASE_URL,
          TELEGRAM_CHAT_SERVER: process.env.TELEGRAM_CHAT_SERVER
        }
      })

      if (goServerProcess) {
        goServerProcess.on('error', (err) => {
          log.error('Failed to start Go server, on error:', err)
          createErrorWindow(
            `Failed to start Go server: ${err instanceof Error ? err.message : 'Unknown error'}`
          )
        })

        goServerProcess.on('close', (code) => {
          log.info(`Go server process exited with code ${code}`)
          goServerProcess = null
        })

        goServerProcess.stdout?.on('data', (data) => {
          log.info(`Go Server stdout: ${data.toString().trim()}`)
        })
        goServerProcess.stderr?.on('data', (data) => {
          log.error(`Go Server stderr: ${data.toString().trim()}`)
        })

        log.info('Go server process spawned. Waiting until it listens …')
        await waitForBackend(DEFAULT_BACKEND_PORT)
      } else {
        log.error('Failed to spawn Go server process.')
      }
    } catch (error: unknown) {
      splash.destroy()
      log.error('Error spawning Go server:', error)
      createErrorWindow(
        `Failed to start Go server: ${error instanceof Error ? error.message : 'Unknown error'}`
      )
    }
  } else {
    log.info('Running in development mode - packaged Go server not started')
  }

  electronApp.setAppUserModelId('com.electron')
  mainWindow = createWindow()
  registerNotificationIpc(mainWindow)
  registerMediaPermissionHandlers(session.defaultSession)
  registerPermissionIpc()
  registerScreenpipeIpc()
  registerAccessibilityIpc()

  splash.destroy()

  // Perform initial check after main window exists, wait a bit for autoUpdater setup
  setTimeout(() => {
    log.info('Performing initial silent update check.')
    checkForUpdates(true)
  }, 5000)

  app.on('browser-window-created', (_, window) => {
    optimizer.watchWindowShortcuts(window)
  })

  ipcMain.on('ping', () => console.log('pong'))

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
    if (mainWindow) {
      mainWindow.webContents.send('native-theme-updated', newTheme)
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

  app.on('activate', function () {
    if (BrowserWindow.getAllWindows().length === 0) createWindow()
  })

  ipcMain.handle('restart-app', async () => {
    log.info('Restarting app manually')
    if (updateDownloaded) {
      log.info('Update pending, installing before manual restart.')
      autoUpdater.quitAndInstall(true, true)
    } else {
      app.relaunch()
      app.exit(0)
    }
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
      const dbPath = join(userDataPath, 'db')
      log.info(`Checking for database folder: ${dbPath}`)

      let dbDeleted = false
      if (fs.existsSync(dbPath)) {
        log.info(`Deleting database folder: ${dbPath}`)
        fs.rmSync(dbPath, { recursive: true, force: true })
        log.info(`Successfully deleted database folder: ${dbPath}`)
        dbDeleted = true
      } else {
        log.info(`Database folder does not exist: ${dbPath}`)
      }

      const logsPath = app.getPath('logs')
      const mainLogPath = join(logsPath, 'main.log')
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

      return dbDeleted || logDeleted
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
  // Create the application menu
  const template: Electron.MenuItemConstructorOptions[] = [
    {
      label: 'File',
      submenu: [
        {
          label: 'Settings',
          accelerator: process.platform === 'darwin' ? 'Command+,' : 'Ctrl+,',
          click: () => {
            if (mainWindow) {
              mainWindow.webContents.send('open-settings')
            }
          }
        },
        { type: 'separator' },
        { role: 'quit' }
      ]
    },
    {
      label: 'Edit',
      submenu: [
        { role: 'undo' },
        { role: 'redo' },
        { type: 'separator' },
        { role: 'cut' },
        { role: 'copy' },
        { role: 'paste' },
        { role: 'delete' },
        { type: 'separator' },
        { role: 'selectAll' }
      ]
    },
    {
      label: 'View',
      submenu: [
        { role: 'reload' },
        { role: 'forceReload' },
        { role: 'toggleDevTools' },
        { type: 'separator' },
        { role: 'resetZoom' },
        { role: 'zoomIn' },
        { role: 'zoomOut' },
        { type: 'separator' },
        { role: 'togglefullscreen' }
      ]
    },
    {
      label: 'Window',
      submenu: [{ role: 'minimize' }, { role: 'zoom' }, { type: 'separator' }, { role: 'front' }]
    }
  ]

  const menu = Menu.buildFromTemplate(template)
  Menu.setApplicationMenu(menu)
})

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') {
    app.quit()
  }
})

app.on('will-quit', () => {
  if (goServerProcess) {
    log.info('Attempting to kill Go server process...')
    const killed = goServerProcess.kill()
    if (killed) {
      log.info('Go server process killed successfully.')
    } else {
      log.warn('Failed to kill Go server process. It might have already exited.')
    }
    goServerProcess = null
  }

  // Clean up the OAuth server if it exists
  if (oauthServer) {
    log.info('Closing OAuth callback server...')
    oauthServer.close()
    oauthServer = null
  }

  // Stop Podman service when app is quitting
  stopPodman()
    .then((success) => {
      if (success) {
        log.info('Podman service stopped successfully')
      } else {
        log.warn('Failed to stop Podman service')
      }
    })
    .catch((err) => {
      log.error('Error stopping Podman service:', err)
    })

  cleanupScreenpipe()
})
