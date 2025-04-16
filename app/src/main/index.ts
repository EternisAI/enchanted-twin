import { app, shell, BrowserWindow, ipcMain, Menu } from 'electron'
import { join } from 'path'
import { electronApp, optimizer, is } from '@electron-toolkit/utils'
import icon from '../../resources/icon.png?asset'
import { spawn, ChildProcess } from 'child_process'

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
  // Start the Go server only in production
  if (!is.dev) {
    const goBinaryPath = join(process.resourcesPath, 'enchanted-twin')
    console.log(`Attempting to start Go server at: ${goBinaryPath}`)
    try {
      goServerProcess = spawn(goBinaryPath, [], {
        stdio: 'inherit'
      })

      if (goServerProcess) {
        goServerProcess.on('error', (err) => {
          console.error('Failed to start Go server:', err)
        })

        goServerProcess.on('close', (code) => {
          console.log(`Go server process exited with code ${code}`)
          goServerProcess = null // Reset when closed
        })

        goServerProcess.stdout?.on('data', (data) => {
          console.log(`Go Server stdout: ${data}`);
        });
        goServerProcess.stderr?.on('data', (data) => {
          console.error(`Go Server stderr: ${data}`);
        });

        console.log('Go server process started.')
      } else {
         console.error('Failed to spawn Go server process.')
      }

    } catch (error) {
      console.error('Error spawning Go server:', error)
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
  if (!is.dev && goServerProcess) {
    console.log('Attempting to kill Go server process...')
    const killed = goServerProcess.kill() // Sends SIGTERM by default
    if (killed) {
       console.log('Go server process killed successfully.')
    } else {
        console.error('Failed to kill Go server process. It might have already exited.')
    }
    goServerProcess = null
  }
})

// In this file you can include the rest of your app's specific main process
// code. You can also put them in separate files and require them here.
