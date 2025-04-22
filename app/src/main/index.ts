import { app, shell, BrowserWindow, ipcMain, dialog, nativeTheme, Menu } from 'electron'
import { join } from 'path'
import { electronApp, optimizer, is } from '@electron-toolkit/utils'
import icon from '../../resources/icon.png?asset'
import { spawn, ChildProcess } from 'child_process'
import log from 'electron-log/main'
import { existsSync, mkdirSync } from 'fs'
import fs from 'fs'
import path from 'path'

const PATHNAME = 'input_data'

let mainWindow: BrowserWindow | null = null
// Check if running in production using environment variable
const IS_PRODUCTION = process.env.IS_PROD_BUILD === 'true' || !is.dev

// Configure electron-log
log.transports.file.level = 'info' // Log info level and above to file
log.info(`Log file will be written to: ${log.transports.file.getFile().path}`)
log.info(`Running in ${IS_PRODUCTION ? 'production' : 'development'} mode`)

let goServerProcess: ChildProcess | null = null

function openOAuthWindow(authUrl: string, redirectUri?: string) {
  const authWindow = new BrowserWindow({
    width: 600,
    height: 800,
    show: true,
    autoHideMenuBar: true,
    webPreferences: {
      nodeIntegration: false,
      contextIsolation: true
    }
  })

  // Parse the redirectUri if provided, or use default values
  let callbackHost = '127.0.0.1'
  let callbackPath = '/callback'

  if (redirectUri) {
    try {
      const parsedRedirect = new URL(redirectUri)
      callbackHost = parsedRedirect.hostname
      callbackPath = parsedRedirect.pathname
      console.log(`[OAuth] Using redirect parameters: host=${callbackHost}, path=${callbackPath}`)
    } catch (err) {
      console.error('[OAuth] Failed to parse redirectUri, using defaults', err)
    }
  }

  const handleUrl = (url: string) => {
    try {
      const parsedUrl = new URL(url)
      if (parsedUrl.hostname === callbackHost && parsedUrl.pathname === callbackPath) {
        const code = parsedUrl.searchParams.get('code')
        const state = parsedUrl.searchParams.get('state')
        if (code && state && mainWindow) {
          console.log('[OAuth] Received callback', { code, state })
          mainWindow.webContents.send('oauth-callback', { code, state })
          authWindow.close()
        }
      }
    } catch (err) {
      console.error('[OAuth] Failed to parse redirect URL', err)
    }
  }

  authWindow.webContents.on('will-redirect', (_event, url) => {
    handleUrl(url)
  })

  authWindow.webContents.on('will-navigate', (_event, url) => {
    handleUrl(url)
  })

  authWindow.on('closed', () => {
    console.log('[OAuth] Auth window closed by user')
  })

  authWindow.loadURL(authUrl)
}

// Configure electron-log
log.transports.file.level = 'info'
log.info(`Log file will be written to: ${log.transports.file.getFile().path}`)

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

app.whenReady().then(() => {
  const executable = process.platform === 'win32' ? 'enchanted-twin.exe' : 'enchanted-twin'
  const goBinaryPath = !IS_PRODUCTION
    ? join(__dirname, '..', '..', 'resources', executable) // Path in development
    : join(process.resourcesPath, 'resources', executable) // Adjusted path in production

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

  // Only start the Go server in production environment
  if (IS_PRODUCTION) {
    log.info(`Attempting to start Go server at: ${goBinaryPath}`)

    try {
      goServerProcess = spawn(goBinaryPath, [], {
        env: {
          ...process.env,
          DB_PATH: dbPath,
          OPENAI_BASE_URL: process.env.OPENAI_BASE_URL,
          COMPLETIONS_MODEL: process.env.COMPLETIONS_MODEL,
          EMBEDDINGS_API_URL: process.env.EMBEDDINGS_API_URL,
          EMBEDDINGS_MODEL: process.env.EMBEDDINGS_MODEL
        }
      })

      if (goServerProcess) {
        goServerProcess.on('error', (err) => {
          log.error('Failed to start Go server:', err)
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

        log.info('Go server process spawned.')
      } else {
        log.error('Failed to spawn Go server process.')
      }
    } catch (error) {
      log.error('Error spawning Go server:', error)
    }
  } else {
    log.info('Running in development mode - packaged Go server not started')
  }

  electronApp.setAppUserModelId('com.electron')

  app.on('browser-window-created', (_, window) => {
    optimizer.watchWindowShortcuts(window)
  })

  mainWindow = createWindow()

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

  ipcMain.handle('select-files', async () => {
    const result = await dialog.showOpenDialog({
      properties: ['openFile', 'multiSelections']
    })
    return result
  })

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
})
