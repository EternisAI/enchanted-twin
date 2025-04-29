import { ipcMain, Notification } from 'electron'
import { BrowserWindow } from 'electron'
import { AppNotification } from '../renderer/src/graphql/generated/graphql'

export function notificationsSupported(): boolean {
  return Notification.isSupported()
}

export function showOsNotification(win: BrowserWindow, notification: AppNotification): void {
  if (!notificationsSupported()) return

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
    .once('failed', (_, err) => console.error('[toast] failed', err)) // <-- fires if the system rejects it

  toast.show()
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
}
