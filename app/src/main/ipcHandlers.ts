import { app, dialog, ipcMain, nativeTheme, shell } from 'electron'
import log from 'electron-log/main'
import path from 'path'
import fs from 'fs'
import { windowManager } from './windows'
import { openOAuthWindow } from './oauthHandler'
import { checkForUpdates } from './autoUpdater'

const PATHNAME = 'input_data'

export function registerIpcHandlers() {
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
    if (windowManager.mainWindow) {
      windowManager.mainWindow.webContents.send('native-theme-updated', newTheme)
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

  ipcMain.handle('restart-app', async () => {
    log.info('Restarting app manually')
    app.relaunch()
    app.exit(0)
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
      const dbPath = path.join(userDataPath, 'db')
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
      const mainLogPath = path.join(logsPath, 'main.log')
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
}
