import { app, shell, BrowserWindow, ipcMain, Menu } from 'electron'
import { join } from 'path'
import { electronApp, optimizer, is } from '@electron-toolkit/utils'
import icon from '../../resources/icon.png?asset'
import { spawn, ChildProcess } from 'child_process'
import log from 'electron-log/main';

// Configure electron-log
log.transports.file.level = 'info'; // Log info level and above to file
log.info(`Log file will be written to: ${log.transports.file.getFile().path}`);

let goServerProcess: ChildProcess | null = null

function createWindow(): void {
  // Create the browser window.
  const mainWindow = new BrowserWindow({
    width: 1200,
    height: 800,
    show: false,
    autoHideMenuBar: true,
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
}

// This method will be called when Electron has finished
// initialization and is ready to create browser windows.
// Some APIs can only be used after this event occurs.
app.whenReady().then(() => {
  // Determine the path to the Go binary based on the environment
  const goBinaryPath = is.dev
    ? join(__dirname, '..', '..', 'resources', 'enchanted-twin') // Path in development
    : join(process.resourcesPath, 'resources', 'enchanted-twin'); // Adjusted path in production

  log.info(`Attempting to start Go server at: ${goBinaryPath}`)
  try {
    goServerProcess = spawn(goBinaryPath, [], {
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
        log.info(`Go Server stdout: ${data.toString().trim()}`);
      });
      goServerProcess.stderr?.on('data', (data) => {
        log.error(`Go Server stderr: ${data.toString().trim()}`);
      });

      log.info('Go server process spawned.')
    } else {
        log.error('Failed to spawn Go server process.')
    }

  } catch (error) {
    log.error('Error spawning Go server:', error)
  }

  // Set app user model id for windows
  electronApp.setAppUserModelId('com.electron')

  // Default open or close DevTools by F12 in development
  // and ignore CommandOrControl + R in production.
  // see https://github.com/alex8088/electron-toolkit/tree/master/packages/utils
  app.on('browser-window-created', (_, window) => {
    optimizer.watchWindowShortcuts(window)
  })

  // IPC test
  ipcMain.on('ping', () => console.log('pong'))

  createWindow()

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
