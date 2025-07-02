import { BrowserWindow, Menu, shell, screen } from 'electron'
import { join } from 'path'
import { is } from '@electron-toolkit/utils'
import icon from '../../resources/icon.png?asset'
import { omnibarStore } from './stores'

const IS_PRODUCTION = process.env.IS_PROD_BUILD === 'true' || !is.dev

export interface WindowManager {
  mainWindow: BrowserWindow | null
  omnibarWindow: BrowserWindow | null
  createMainWindow: () => BrowserWindow
  createOmnibarWindow: () => BrowserWindow
  toggleOmnibarWindow: () => void
  setAppQuitting: (quitting: boolean) => void
  setPendingNavigation: (url: string) => void
  processPendingNavigation: () => void
}

class WindowManagerImpl implements WindowManager {
  mainWindow: BrowserWindow | null = null
  omnibarWindow: BrowserWindow | null = null
  // @ts-expect-error - this is used in the main process
  private isAppQuitting = false
  private pendingNavigation: string | null = null

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

    mainWindow.on('closed', () => {
      // When main window is closed, keep reference but mark as closed
      // Don't destroy omnibar window - it should remain functional
      this.mainWindow = null
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

  createOmnibarWindow(): BrowserWindow {
    if (this.omnibarWindow && !this.omnibarWindow.isDestroyed()) {
      return this.omnibarWindow
    }

    const omnibarWindow = new BrowserWindow({
      width: 500,
      height: 64,
      minHeight: 64,
      maxHeight: 500,
      minWidth: 500,
      maxWidth: 800,
      show: false,
      // backgroundColor: '#00000000',
      frame: false,
      transparent: true,
      alwaysOnTop: true,
      skipTaskbar: true,
      resizable: true,
      movable: true,
      minimizable: false,
      maximizable: false,
      closable: false,
      focusable: true,
      hasShadow: true,
      webPreferences: {
        preload: join(__dirname, '../preload/index.js'),
        sandbox: false,
        nodeIntegration: false,
        contextIsolation: true
      }
    })

    // Set position - use stored position if available, otherwise center
    const primaryDisplay = screen.getPrimaryDisplay()
    const { width, height } = primaryDisplay.workAreaSize
    const storedPosition = omnibarStore.get('position') as { x: number; y: number }
    const hasCustomPosition = omnibarStore.get('hasCustomPosition') as boolean

    if (hasCustomPosition && storedPosition) {
      // Ensure the stored position is still on screen
      const x = Math.max(0, Math.min(storedPosition.x, width - 500))
      const y = Math.max(0, Math.min(storedPosition.y, height - 80))
      omnibarWindow.setPosition(x, y)
    } else {
      // Default centered position
      const defaultX = Math.round((width - 500) / 2)
      const defaultY = Math.round(height * 0.25) // 25% from top
      omnibarWindow.setPosition(defaultX, defaultY)
      omnibarStore.set('position', { x: defaultX, y: defaultY })
    }

    // Save position when window is moved
    omnibarWindow.on('moved', () => {
      if (this.omnibarWindow && !this.omnibarWindow.isDestroyed()) {
        const [x, y] = this.omnibarWindow.getPosition()
        omnibarStore.set('position', { x, y })
        omnibarStore.set('hasCustomPosition', true)
      }
    })

    // Hide when losing focus
    omnibarWindow.on('blur', () => {
      if (this.omnibarWindow && !this.omnibarWindow.isDestroyed()) {
        this.omnibarWindow.hide()
      }
    })

    // Load the overlay page
    if (!IS_PRODUCTION && process.env['ELECTRON_RENDERER_URL']) {
      omnibarWindow.loadURL(`${process.env['ELECTRON_RENDERER_URL']}#/omnibar-overlay`)
    } else {
      omnibarWindow.loadFile(join(__dirname, '../renderer/index.html'), {
        hash: 'omnibar-overlay'
      })
    }

    this.omnibarWindow = omnibarWindow
    return omnibarWindow
  }

  toggleOmnibarWindow(): void {
    if (!this.omnibarWindow || this.omnibarWindow.isDestroyed()) {
      this.createOmnibarWindow()
    }

    if (this.omnibarWindow!.isVisible()) {
      this.omnibarWindow!.hide()
    } else {
      this.omnibarWindow!.show()
      this.omnibarWindow!.focus()
    }
  }

  setAppQuitting(quitting: boolean): void {
    this.isAppQuitting = quitting
  }

  setPendingNavigation(url: string): void {
    this.pendingNavigation = url
  }

  processPendingNavigation(): void {
    if (this.pendingNavigation && this.mainWindow && !this.mainWindow.isDestroyed()) {
      this.mainWindow.webContents.send('navigate-to', this.pendingNavigation)
      this.pendingNavigation = null
    }
  }
}

export const windowManager = new WindowManagerImpl()
