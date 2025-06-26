import { BrowserWindow, Menu, shell } from 'electron'
import { join } from 'path'
import { is } from '@electron-toolkit/utils'
import icon from '../../resources/icon.png?asset'

const IS_PRODUCTION = process.env.IS_PROD_BUILD === 'true' || !is.dev

export interface WindowManager {
  mainWindow: BrowserWindow | null
  createMainWindow: () => BrowserWindow
}

class WindowManagerImpl implements WindowManager {
  mainWindow: BrowserWindow | null = null

  createMainWindow(): BrowserWindow {
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
        sandbox: false,
        webSecurity: false
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

    if (!IS_PRODUCTION && process.env['ELECTRON_RENDERER_URL']) {
      mainWindow.loadURL(process.env['ELECTRON_RENDERER_URL'])
    } else {
      mainWindow.loadFile(join(__dirname, '../renderer/index.html'))
    }

    this.mainWindow = mainWindow
    return mainWindow
  }
}

export const windowManager = new WindowManagerImpl()
