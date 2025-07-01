import { Menu, app } from 'electron'
import { windowManager } from './windows'

export function setupMenu() {
  const isMac = process.platform === 'darwin'

  const template: Electron.MenuItemConstructorOptions[] = [
    // On macOS, create the Application menu
    ...(isMac
      ? [
          {
            label: app.getName(),
            submenu: [
              { role: 'about' as const },
              { type: 'separator' as const },
              {
                label: 'Preferences...',
                accelerator: 'Command+,',
                click: () => {
                  if (!windowManager.mainWindow || windowManager.mainWindow.isDestroyed()) {
                    windowManager.createMainWindow()
                    // Store the settings navigation to be processed when renderer is ready
                    windowManager.setPendingNavigation('/settings')
                  } else {
                    windowManager.mainWindow.show()
                    windowManager.mainWindow.focus()
                    windowManager.mainWindow.webContents.send('open-settings')
                  }
                }
              },
              { type: 'separator' as const },
              { role: 'services' as const },
              { type: 'separator' as const },
              { role: 'hide' as const },
              { role: 'hideOthers' as const },
              { role: 'unhide' as const },
              { type: 'separator' as const },
              { role: 'quit' as const }
            ]
          }
        ]
      : []),
    {
      label: 'File',
      submenu: [
        {
          label: 'New Chat',
          accelerator: isMac ? 'Command+N' : 'Ctrl+N',
          click: () => {
            // Ensure main window exists and is visible
            if (!windowManager.mainWindow || windowManager.mainWindow.isDestroyed()) {
              windowManager.createMainWindow()
              // Store the home navigation to be processed when renderer is ready
              windowManager.setPendingNavigation('/')
            } else {
              windowManager.mainWindow.show()
              windowManager.mainWindow.focus()
              // Send new chat command to renderer
              windowManager.mainWindow.webContents.send('new-chat')
            }
          }
        },
        { type: 'separator' as const },
        // Only show Settings in File menu on non-macOS platforms
        ...(!isMac
          ? [
              {
                label: 'Settings',
                accelerator: 'Ctrl+,',
                click: () => {
                  if (!windowManager.mainWindow || windowManager.mainWindow.isDestroyed()) {
                    windowManager.createMainWindow()
                    // Store the settings navigation to be processed when renderer is ready
                    windowManager.setPendingNavigation('/settings')
                  } else {
                    windowManager.mainWindow.show()
                    windowManager.mainWindow.focus()
                    windowManager.mainWindow.webContents.send('open-settings')
                  }
                }
              },
              { type: 'separator' as const }
            ]
          : []),
        ...(isMac ? [] : [{ role: 'quit' as const }])
      ]
    },
    {
      label: 'Edit',
      submenu: [
        { role: 'undo' },
        { role: 'redo' },
        { type: 'separator' },
        { role: 'cut' },
        { role: 'copy' },
        { role: 'paste' },
        { role: 'delete' },
        { type: 'separator' },
        { role: 'selectAll' }
      ]
    },
    {
      label: 'View',
      submenu: [
        { role: 'reload' },
        { role: 'forceReload' },
        { role: 'toggleDevTools' },
        { type: 'separator' },
        { role: 'resetZoom' },
        { role: 'zoomIn' },
        { role: 'zoomOut' },
        { type: 'separator' },
        { role: 'togglefullscreen' }
      ]
    },
    {
      label: 'Window',
      submenu: [
        { role: 'minimize' },
        { role: 'zoom' },
        ...(isMac
          ? [
              {
                label: 'Close',
                accelerator: 'Command+W',
                click: () => {
                  if (windowManager.mainWindow && !windowManager.mainWindow.isDestroyed()) {
                    windowManager.mainWindow.close()
                  }
                }
              },
              { type: 'separator' as const },
              { role: 'front' as const },
              { type: 'separator' as const },
              { role: 'window' as const }
            ]
          : [{ role: 'close' as const }])
      ]
    }
  ]

  const menu = Menu.buildFromTemplate(template)
  Menu.setApplicationMenu(menu)
}
