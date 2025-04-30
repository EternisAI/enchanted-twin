import { ipcMain, Notification, shell } from 'electron'
import { BrowserWindow } from 'electron'
import { AppNotification } from '../renderer/src/graphql/generated/graphql'
import { existsSync } from 'fs'
import { exec } from 'child_process'

export function notificationsSupported(): boolean {
  return Notification.isSupported()
}

function checkNotificationStatus(): Promise<boolean> {
  if (process.platform === 'darwin') {
    //@TODO: Handle macOS notifications checking properly
    return Promise.resolve(true)
  }

  return new Promise((resolve) => {
    if (!Notification.isSupported()) return resolve(false)

    const testToast = new Notification({ title: '', body: '', silent: true })
    let resolved = false

    const timer = setTimeout(() => {
      if (!resolved) resolve(false)
    }, 800)

    testToast.once('show', () => {
      console.log('show')
      resolved = true
      clearTimeout(timer)
      resolve(true)
    })
    testToast.show()
  })
}

export async function showOsNotification(
  win: BrowserWindow,
  notification: AppNotification
): Promise<boolean> {
  if (!notificationsSupported()) return false
  const notificationEnabled = await checkNotificationStatus()

  if (!notificationEnabled) return false

  console.log('showOsNotification', notification, notificationsSupported())
  const toast = new Notification({
    title: notification.title,
    body: notification.message,
    icon: notification.image ?? undefined,
    silent: false
  })

  toast.on('click', () => {
    if (notification.link) {
      win.webContents.send('open-deeplink', notification.link)
    }
  })

  toast
    .once('show', () => console.log('[toast] show event – OS accepted'))
    .once('action', (_, idx) => console.log('[toast] action', idx))
    .once('click', () => console.log('[toast] click'))
    .once('close', () => console.log('[toast] close'))
    .once('failed', (_, err) => console.error('[toast] failed', err))

  toast.show()

  return true
}

export async function openSystemNotificationSettings(): Promise<boolean> {
  try {
    switch (process.platform) {
      case 'darwin':
        await shell.openExternal('x-apple.systempreferences:com.apple.preference.notifications')
        return true

      case 'win32':
        await shell.openExternal('ms-settings:notifications')
        return true

      default: {
        if (existsSync('/usr/bin/gnome-control-center')) {
          exec('gnome-control-center notifications')
          return true
        }
        if (existsSync('/usr/bin/kcmshell5')) {
          exec('kcmshell5 kcm_notifications')
          return true
        }
        exec('xdg-open .')
        return false
      }
    }
  } catch (error) {
    console.error('Failed to open OS notification settings:', error)
  }
  return false
}

export function registerNotificationIpc(win: BrowserWindow) {
  ipcMain.handle('notify', (_evt, raw: unknown) => {
    const notification = raw as Partial<AppNotification>
    if (!notification.id || !notification.title || !notification.message) return
    showOsNotification(win, notification as AppNotification)
  })

  ipcMain.on('open-url', (_evt, url: string) => {
    win.webContents.send('open-deeplink', url)
  })

  ipcMain.handle('notification-status', async () => {
    const status = await checkNotificationStatus()
    return status
  })

  ipcMain.handle('open-notification-settings', async () => {
    return openSystemNotificationSettings()
  })
}
