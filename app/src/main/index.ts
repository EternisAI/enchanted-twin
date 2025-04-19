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

// Configure electron-log
log.transports.file.level = 'info' // Log info level and above to file
log.info(`Log file will be written to: ${log.transports.file.getFile().path}`)

let goServerProcess: ChildProcess | null = null

function createWindow(): BrowserWindow {
  // Create the browser window.
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

  // Add context menu for developer tools
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
  if (is.dev && process.env['ELECTRON_RENDERER_URL']) {
    mainWindow.loadURL(process.env['ELECTRON_RENDERER_URL'])
  } else {
    mainWindow.loadFile(join(__dirname, '../renderer/index.html'))
  }

  return mainWindow
}

// This method will be called when Electron has finished
// initialization and is ready to create browser windows.
// Some APIs can only be used after this event occurs.
app.whenReady().then(() => {
  // Determine the path to the Go binary based on the environment and platform
  const executable = process.platform === 'win32' ? 'enchanted-twin.exe' : 'enchanted-twin'
  const goBinaryPath = is.dev
    ? join(__dirname, '..', '..', 'resources', executable) // Path in development
    : join(process.resourcesPath, 'resources', executable) // Adjusted path in production

  // Create the database directory in user data path
  const userDataPath = app.getPath('userData')
  const dbDir = join(userDataPath, 'db')

  // Ensure the database directory exists
  if (!existsSync(dbDir)) {
    try {
      mkdirSync(dbDir, { recursive: true })
      log.info(`Created database directory: ${dbDir}`)
    } catch (err) {
      log.error(`Failed to create database directory: ${err}`)
    }
  }

  log.info(`Database directory: ${dbDir}`)
  log.info(`Go binary path: ${goBinaryPath}`)

  if (!existsSync(goBinaryPath)) {
    log.info(`Go binary not found at path: ${goBinaryPath}`)
  } else {
    log.info(`Attempting to start Go server at: ${goBinaryPath}`)

    try {
      goServerProcess = spawn(goBinaryPath, [`--db-path=${join(dbDir, 'enchanted-twin.db')}`], {
        // No stdio option here, defaults to 'pipe'
      })

      if (goServerProcess) {
        goServerProcess.on('error', (err) => {
          log.error('Failed to start Go server:', err)
        })

        goServerProcess.on('close', (code) => {
          log.info(`Go server process exited with code ${code}`)
          goServerProcess = null // Reset when closed
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
  }

  // Set app user model id for windows
  electronApp.setAppUserModelId('com.electron')

  // Default open or close DevTools by F12 in development
  // and ignore CommandOrControl + R in production.
  // see https://github.com/alex8088/electron-toolkit/tree/master/packages/utils
  app.on('browser-window-created', (_, window) => {
    optimizer.watchWindowShortcuts(window)
  })

  const mainWindow = createWindow()

  // IPC test
  ipcMain.on('ping', () => console.log('pong'))

  // Handle theme changes
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

  // Listen for native theme changes and notify renderer
  nativeTheme.on('updated', () => {
    const newTheme = nativeTheme.shouldUseDarkColors ? 'dark' : 'light'
    mainWindow.webContents.send('native-theme-updated', newTheme)
  })

  // Handle directory selection
  ipcMain.handle('select-directory', async () => {
    const result = await dialog.showOpenDialog({
      properties: ['openDirectory']
    })
    return result
  })

  // Handle file selection
  ipcMain.handle('select-files', async () => {
    const result = await dialog.showOpenDialog({
      properties: ['openFile', 'multiSelections']
    })
    return result
  })

  // This will be used to copy the files to the app's storage directory to be read later by GO
  ipcMain.handle('copy-dropped-files', async (_event, filePaths) => {
    console.log('copy-dropped-files', filePaths)
    const fileStoragePath =
      process.env.NODE_ENV === 'development'
        ? path.join(app.getAppPath(), PATHNAME)
        : path.join(app.getPath('userData'), PATHNAME)

    // Ensure storage directory exists
    if (!fs.existsSync(fileStoragePath)) {
      fs.mkdirSync(fileStoragePath, { recursive: true })
    }

    const savedFiles: string[] = []

    console.log('fileStoragePath', fileStoragePath)

    for (const filePath of filePaths) {
      const fileName = path.basename(filePath)
      const destinationPath = path.join(fileStoragePath, fileName)

      console.log('destinationPath', destinationPath)

      try {
        // Copy file to storage directory
        fs.copyFileSync(filePath, destinationPath)
        savedFiles.push(destinationPath)
      } catch (error) {
        console.error('File save error:', error)
      }
    }

    console.log('savedFiles', savedFiles)

    return savedFiles
  })

  ipcMain.handle('get-stored-files-path', () => {
    const appPath = app.getAppPath()
    return path.join(appPath, PATHNAME)
  })

  app.on('activate', function () {
    // On macOS it's common to re-create a window in the app when the
    // dock icon is clicked and there are no other windows open.
    if (BrowserWindow.getAllWindows().length === 0) createWindow()
  })
})

// Quit when all windows are closed, except on macOS. There, it's common
// for applications and their menu bar to stay active until the user quits
// explicitly with Cmd + Q.
app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') {
    app.quit()
  }
})

// Ensure Go server is killed when the app quits (only if started by Electron, i.e., production)
app.on('will-quit', () => {
  if (goServerProcess) {
    log.info('Attempting to kill Go server process...')
    const killed = goServerProcess.kill() // Sends SIGTERM by default
    if (killed) {
      log.info('Go server process killed successfully.')
    } else {
      log.warn('Failed to kill Go server process. It might have already exited.')
    }
    goServerProcess = null
  }
})

// In this file you can include the rest of your app's specific main process
// code. You can also put them in separate files and require them here.
