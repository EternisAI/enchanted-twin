import { app, dialog } from 'electron'
import { autoUpdater } from 'electron-updater'
import log from 'electron-log/main'
import { windowManager } from './windows'

let updateDownloaded = false

export function setupAutoUpdater() {
  if (!process.env.IS_PROD_BUILD) {
    log.info('Skipping auto-updater in development mode')
    return
  }

  autoUpdater.logger = log
  log.transports.file.level = 'debug'
  autoUpdater.autoDownload = true

  autoUpdater.on('checking-for-update', () => {
    log.info('Checking for update...')
    if (windowManager.mainWindow) {
      windowManager.mainWindow.webContents.send('update-status', 'Checking for update...')
    }
  })

  autoUpdater.on('update-available', (info) => {
    log.info('Update available:', info)
    if (windowManager.mainWindow) {
      windowManager.mainWindow.webContents.send('update-status', 'Update available, downloading...')
    }
  })

  autoUpdater.on('update-not-available', (info) => {
    log.info('Update not available:', info)
    if (windowManager.mainWindow) {
      windowManager.mainWindow.webContents.send('update-status', 'No updates available')
    }
  })

  autoUpdater.on('error', (err) => {
    log.error('Error in auto-updater:', err)
    if (windowManager.mainWindow) {
      windowManager.mainWindow.webContents.send('update-status', `Error: ${err.message}`)
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

    if (windowManager.mainWindow) {
      windowManager.mainWindow.webContents.send('update-progress', progressObj)
    }
  })

  autoUpdater.on('update-downloaded', (info) => {
    log.info('Update downloaded:', info)
    updateDownloaded = true
    windowManager.mainWindow?.webContents.send(
      'update-status',
      'Update downloaded – ready to install'
    )

    dialog
      .showMessageBox(windowManager.mainWindow!, {
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

export function checkForUpdates(silent = false) {
  log.info(`[checkForUpdates] Called with silent=${silent}`)

  if (updateDownloaded) {
    log.info(`[checkForUpdates] Update previously downloaded. Silent check: ${silent}`)
    if (silent) {
      log.info(
        '[checkForUpdates] Silent check found downloaded update. Initiating quit and install...'
      )
      autoUpdater.quitAndInstall(true, true)
    } else {
      if (windowManager.mainWindow) {
        dialog
          .showMessageBox(windowManager.mainWindow, {
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
  if (windowManager.mainWindow && !silent) {
    windowManager.mainWindow.webContents.send('update-status', 'Checking for update...')
  }

  autoUpdater
    .checkForUpdates()
    .then((result) => {
      if (!result || !result.updateInfo || result.updateInfo.version === app.getVersion()) {
        if (windowManager.mainWindow && !silent) {
          windowManager.mainWindow.webContents.send('update-status', 'No updates available')
          dialog.showMessageBox(windowManager.mainWindow, {
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
      if (windowManager.mainWindow && !silent) {
        windowManager.mainWindow.webContents.send(
          'update-status',
          `Error checking for updates: ${err.message}`
        )
        dialog.showErrorBox(
          'Update Check Error',
          `An error occurred while checking for updates: ${err.message}`
        )
      }
    })
}
