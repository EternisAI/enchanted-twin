import {
  ipcMain,
  IpcMainEvent,
  WebContents,
  BrowserWindow,
  WebContentsView,
  Rectangle
} from 'electron'
import log from 'electron-log/main'
import * as path from 'path'
import { fileURLToPath } from 'url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

interface BrowserContent {
  text: string
  html: string
  metadata: {
    title: string
    description?: string
    keywords?: string[]
    author?: string
  }
}

interface BrowserAction {
  type: string
  params: Record<string, unknown>
}

interface ActionResult {
  actionId: string
  result: {
    success: boolean
    data?: unknown
    error?: string
  }
}

interface BrowserSession {
  view: WebContentsView
  webContents: WebContents
  content?: BrowserContent
  url?: string
}

// Add interface
interface BrowserSecuritySettings {
  enableJavaScript: boolean
  enablePlugins: boolean
  enableWebSecurity: boolean
  requireUserApproval: boolean
}

// Store active browser sessions
const browserSessions = new Map<string, BrowserSession>()

export function registerBrowserIpc(mainWindow: BrowserWindow) {
  // Clean up all browser sessions when main window refreshes or navigates
  const cleanupAllSessions = () => {
    log.info('Cleaning up all browser sessions')
    for (const [, session] of browserSessions.entries()) {
      if (!mainWindow.isDestroyed()) {
        mainWindow.contentView.removeChildView(session.view)
      }
      session.webContents.close()
    }
    browserSessions.clear()
  }

  // Listen for main window refresh/navigation
  mainWindow.webContents.on('did-start-navigation', (_event, _url, isInPlace, isMainFrame) => {
    if (isMainFrame && isInPlace) {
      log.info('Main window refresh detected - cleaning up browser sessions')
      cleanupAllSessions()
    }
  })

  mainWindow.webContents.on('render-process-gone', (_event, details) => {
    log.error(`Renderer process gone: ${details.reason} - cleaning up browser sessions`)
    cleanupAllSessions()
  })

  // Handle content updates from webview
  ipcMain.on('browser:content-update', (event: IpcMainEvent, content: BrowserContent) => {
    const sessionId = findSessionId(event.sender)
    if (sessionId) {
      const session = browserSessions.get(sessionId)
      if (session) {
        session.content = content

        // Notify renderer about content update
        const mainWindow = BrowserWindow.getAllWindows().find((w) => !w.isDestroyed())
        if (mainWindow) {
          mainWindow.webContents.send('browser:session-updated', {
            sessionId,
            content
          })
        }
      }
    }
    log.info('Browser content updated', { sessionId, title: content.metadata.title })
  })

  // Handle navigation events
  ipcMain.on('browser:navigation', (event: IpcMainEvent, url: string) => {
    const sessionId = findSessionId(event.sender)
    if (sessionId) {
      const session = browserSessions.get(sessionId)
      if (session) {
        session.url = url

        // Notify renderer about navigation
        const mainWindow = BrowserWindow.getAllWindows().find((w) => !w.isDestroyed())
        if (mainWindow) {
          mainWindow.webContents.send('browser:navigation-occurred', {
            sessionId,
            url
          })
        }
      }
    }
    log.info('Browser navigation', { sessionId, url })
  })

  // Handle scroll position updates
  ipcMain.on('browser:scroll', (event: IpcMainEvent, position: { x: number; y: number }) => {
    const sessionId = findSessionId(event.sender)
    log.debug('Browser scroll', { sessionId, position })
  })

  // Handle action results from webview
  ipcMain.on('browser:action-result', (event: IpcMainEvent, data: ActionResult) => {
    const sessionId = findSessionId(event.sender)

    // Forward result to renderer
    const mainWindow = BrowserWindow.getAllWindows().find((w) => !w.isDestroyed())
    if (mainWindow) {
      mainWindow.webContents.send('browser:action-completed', {
        sessionId,
        ...data
      })
    }

    log.info('Browser action result', {
      sessionId,
      actionId: data.actionId,
      success: data.result.success
    })
  })

  // Create browser session
  ipcMain.handle(
    'browser:create-session',
    async (
      _,
      sessionId: string,
      initialUrl: string,
      partition: string,
      securitySettings: BrowserSecuritySettings
    ) => {
      try {
        const mainWindow = BrowserWindow.getAllWindows().find((w) => !w.isDestroyed())
        if (!mainWindow) {
          throw new Error('No main window found')
        }

        const preloadPath = path.join(__dirname, 'browserPreload.js') // Adjust if needed to point to the correct preload file

        const view = new WebContentsView({
          webPreferences: {
            preload: preloadPath,
            partition,
            contextIsolation: true,
            nodeIntegration: false,
            webSecurity: securitySettings.enableWebSecurity,
            javascript: securitySettings.enableJavaScript,
            plugins: securitySettings.enablePlugins
          }
        })

        const webContents = view.webContents
        browserSessions.set(sessionId, { view, webContents, url: initialUrl })

        mainWindow.contentView.addChildView(view)

        // Load initial URL
        await webContents.loadURL(initialUrl)

        // Send initial navigation state
        mainWindow.webContents.send('browser:navigation-state', sessionId, {
          canGoBack: false,
          canGoForward: false
        })

        // Attach event listeners
        webContents.on('did-start-loading', () => {
          mainWindow.webContents.send('browser:did-start-loading', sessionId)
        })

        webContents.on('did-stop-loading', () => {
          mainWindow.webContents.send('browser:did-stop-loading', sessionId)
        })

        webContents.on(
          'did-fail-load',
          (_event, errorCode, errorDescription, validatedURL, isMainFrame) => {
            mainWindow.webContents.send('browser:did-fail-load', sessionId, {
              errorCode,
              errorDescription,
              validatedURL,
              isMainFrame
            })
          }
        )

        webContents.on('will-navigate', (_event, url) => {
          log.info('Browser will-navigate', { sessionId, url })
          mainWindow.webContents.send('browser:did-navigate', sessionId, url)
        })

        webContents.on('did-navigate', (_event, url) => {
          log.info('Browser did-navigate', { sessionId, url })
          mainWindow.webContents.send('browser:did-navigate', sessionId, url)
          // Send navigation state
          mainWindow.webContents.send('browser:navigation-state', sessionId, {
            canGoBack: webContents.navigationHistory.canGoBack(),
            canGoForward: webContents.navigationHistory.canGoForward()
          })
        })

        webContents.on('did-navigate-in-page', (_event, url, isMainFrame) => {
          if (isMainFrame) {
            log.info('Browser did-navigate-in-page', { sessionId, url })
            mainWindow.webContents.send('browser:did-navigate', sessionId, url)
            // Send navigation state
            mainWindow.webContents.send('browser:navigation-state', sessionId, {
              canGoBack: webContents.navigationHistory.canGoBack(),
              canGoForward: webContents.navigationHistory.canGoForward()
            })
          }
        })

        webContents.on('page-title-updated', (_event, title) => {
          mainWindow.webContents.send('browser:page-title-updated', sessionId, title)
        })

        webContents.setWindowOpenHandler(() => {
          return { action: 'deny' }
        })

        webContents.session.setPermissionRequestHandler((_webContents, permission, callback) => {
          callback(false) // Deny by default
          log.info('Permission denied', { sessionId, permission })
        })

        log.info('Browser session created', { sessionId })
        return { success: true }
      } catch (error) {
        log.error('Failed to create browser session', error)
        return { success: false, error: error instanceof Error ? error.message : 'Unknown error' }
      }
    }
  )

  // Execute action in browser
  ipcMain.handle('browser:execute-action', async (_, sessionId: string, action: BrowserAction) => {
    try {
      const session = browserSessions.get(sessionId)
      if (!session) {
        throw new Error(`Browser session not found: ${sessionId}`)
      }

      // Send action to webview
      session.webContents.send('browser:execute-action', action)

      log.info('Executing browser action', { sessionId, actionType: action.type })
      return { success: true }
    } catch (error) {
      log.error('Failed to execute browser action', error)
      return { success: false, error: error instanceof Error ? error.message : 'Unknown error' }
    }
  })

  // Take screenshot of browser
  ipcMain.handle('browser:capture-screenshot', async (_, sessionId: string) => {
    try {
      const session = browserSessions.get(sessionId)
      if (!session) {
        throw new Error(`Browser session not found: ${sessionId}`)
      }

      // Capture the webview content
      const image = await session.webContents.capturePage()
      const buffer = image.toPNG()
      const base64 = buffer.toString('base64')

      log.info('Browser screenshot captured', { sessionId, size: buffer.length })
      return { success: true, data: `data:image/png;base64,${base64}` }
    } catch (error) {
      log.error('Failed to capture browser screenshot', error)
      return { success: false, error: error instanceof Error ? error.message : 'Unknown error' }
    }
  })

  // Get browser session content
  ipcMain.handle('browser:get-content', async (_, sessionId: string) => {
    try {
      const session = browserSessions.get(sessionId)
      if (!session) {
        throw new Error(`Browser session not found: ${sessionId}`)
      }

      return { success: true, data: session.content }
    } catch (error) {
      log.error('Failed to get browser content', error)
      return { success: false, error: error instanceof Error ? error.message : 'Unknown error' }
    }
  })

  // Handle set bounds
  ipcMain.on('browser:set-bounds', (_, sessionId: string, rect: Rectangle) => {
    const session = browserSessions.get(sessionId)
    if (session) {
      session.view.setBounds(rect)
    }
  })

  // Navigation handlers
  ipcMain.handle('browser:load-url', async (_, sessionId: string, url: string) => {
    try {
      const session = browserSessions.get(sessionId)
      if (!session) throw new Error('Session not found')
      await session.webContents.loadURL(url)
      return { success: true }
    } catch (error) {
      return { success: false, error: error instanceof Error ? error.message : 'Unknown error' }
    }
  })

  ipcMain.handle('browser:go-back', async (_, sessionId: string) => {
    try {
      const session = browserSessions.get(sessionId)
      if (!session) throw new Error('Session not found')
      if (session.webContents.navigationHistory.canGoBack()) {
        session.webContents.navigationHistory.goBack()
      }
      return { success: true }
    } catch (error) {
      return { success: false, error: error instanceof Error ? error.message : 'Unknown error' }
    }
  })

  ipcMain.handle('browser:go-forward', async (_, sessionId: string) => {
    try {
      const session = browserSessions.get(sessionId)
      if (!session) throw new Error('Session not found')
      if (session.webContents.navigationHistory.canGoForward()) {
        session.webContents.navigationHistory.goForward()
      }
      return { success: true }
    } catch (error) {
      return { success: false, error: error instanceof Error ? error.message : 'Unknown error' }
    }
  })

  ipcMain.handle('browser:reload', async (_, sessionId: string) => {
    try {
      const session = browserSessions.get(sessionId)
      if (!session) throw new Error('Session not found')
      session.webContents.reload()
      return { success: true }
    } catch (error) {
      return { success: false, error: error instanceof Error ? error.message : 'Unknown error' }
    }
  })

  ipcMain.handle('browser:stop', async (_, sessionId: string) => {
    try {
      const session = browserSessions.get(sessionId)
      if (!session) throw new Error('Session not found')
      session.webContents.stop()
      return { success: true }
    } catch (error) {
      return { success: false, error: error instanceof Error ? error.message : 'Unknown error' }
    }
  })

  // Clean up browser session
  ipcMain.handle('browser:destroy-session', async (_, sessionId: string) => {
    try {
      const session = browserSessions.get(sessionId)
      if (session) {
        const mainWindow = BrowserWindow.getAllWindows().find((w) => !w.isDestroyed())
        if (mainWindow) {
          mainWindow.contentView.removeChildView(session.view)
        }
        // Note: WebContentsView doesn't have destroy, but we can close the webContents
        session.webContents.close()
        browserSessions.delete(sessionId)
        log.info('Browser session destroyed', { sessionId })
      }
      return { success: true }
    } catch (error) {
      log.error('Failed to destroy browser session', error)
      return { success: false, error: error instanceof Error ? error.message : 'Unknown error' }
    }
  })
}

// Helper function to find session ID by webContents
function findSessionId(webContents: WebContents): string | undefined {
  for (const [sessionId, session] of browserSessions.entries()) {
    if (session.webContents.id === webContents.id) {
      return sessionId
    }
  }
  return undefined
}
