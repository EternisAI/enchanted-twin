import { app, shell, BrowserWindow } from 'electron'
import http from 'http'
import log from 'electron-log/main'
import { windowManager } from './windows'

const DEFAULT_OAUTH_SERVER_PORT = 8080

let oauthServer: http.Server | null = null

function startOAuthCallbackServer(callbackPath: string): Promise<http.Server> {
  return new Promise((resolve, reject) => {
    if (oauthServer) {
      oauthServer.close()
      oauthServer = null
    }

    const server = http.createServer((req, res) => {
      log.info(`[OAuth] Received request: ${req.url}`)

      if (req.url && req.url.startsWith(callbackPath)) {
        log.info(`[OAuth] Received callback: ${req.url}`)

        try {
          const parsedUrl = new URL(`http://localhost:${DEFAULT_OAUTH_SERVER_PORT}${req.url}`)
          const code = parsedUrl.searchParams.get('code')
          const state = parsedUrl.searchParams.get('state')

          if (code && state && windowManager.mainWindow) {
            windowManager.mainWindow.webContents.send('oauth-callback', { code, state })
            res.writeHead(200, { 'Content-Type': 'text/html' })
            res.end(`
              <!DOCTYPE html>
              <html>
                <head>
                  <title>Authentication Successful</title>
                  <style>
                    body { font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; text-align: center; padding: 40px; }
                    h1 { color: #333; }
                    p { color: #666; }
                    .success { color: #4CAF50; font-weight: bold; }
                  </style>
                </head>
                <body>
                  <h1>Authentication Successful</h1>
                  <p class="success">You have successfully authenticated!</p>
                  <p>You can close this window and return to the application.</p>
                </body>
              </html>
            `)

            setTimeout(() => {
              if (oauthServer) {
                oauthServer.close()
                oauthServer = null
                log.info('[OAuth] Callback server closed after successful authentication')
              }
            }, 3000)
          } else {
            res.writeHead(400, { 'Content-Type': 'text/html' })
            res.end(
              '<html><body><h1>Authentication Error</h1><p>Invalid or missing parameters.</p></body></html>'
            )
          }
        } catch (err) {
          log.error('[OAuth] Failed to parse callback URL', err)
          res.writeHead(500, { 'Content-Type': 'text/plain' })
          res.end('Internal Server Error')
        }
      } else {
        log.info(`[OAuth] Received non-callback request: ${req.url}`)
        res.writeHead(404, { 'Content-Type': 'text/plain' })
        res.end('Not Found')
      }
    })

    server.on('error', (err) => {
      log.error(`[OAuth] Server error on port ${DEFAULT_OAUTH_SERVER_PORT}:`, err)
      reject(err)
    })

    server.listen(DEFAULT_OAUTH_SERVER_PORT, '127.0.0.1', () => {
      log.info(
        `[OAuth] Callback server listening on http://127.0.0.1:${DEFAULT_OAUTH_SERVER_PORT}${callbackPath}`
      )
      resolve(server)
    })
  })
}

export function openOAuthWindow(authUrl: string, redirectUri?: string) {
  let callbackPath = '/callback'
  if (redirectUri) {
    try {
      const parsedRedirect = new URL(redirectUri)
      callbackPath = parsedRedirect.pathname

      if (parsedRedirect.protocol === 'https:') {
        log.info('[OAuth] Using custom Electron window for HTTPS redirect')

        const authWindow = new BrowserWindow({
          width: 800,
          height: 600,
          show: true,
          webPreferences: {
            nodeIntegration: false,
            contextIsolation: true,
            webSecurity: true
          }
        })

        app.on('certificate-error', (event, webContents, _url, _error, _certificate, callback) => {
          if (webContents.id === authWindow.webContents.id) {
            log.info('[OAuth] Handling certificate error for auth window')
            event.preventDefault()
            callback(true)
          } else {
            callback(false)
          }
        })

        const filter = { urls: [parsedRedirect.origin + callbackPath + '*'] }

        authWindow.webContents.session.webRequest.onBeforeRequest(filter, (details, callback) => {
          log.info(`[OAuth] Intercepted request to: ${details.url}`)

          try {
            const parsedUrl = new URL(details.url)
            const code = parsedUrl.searchParams.get('code')
            const state = parsedUrl.searchParams.get('state')

            if (code && state && windowManager.mainWindow) {
              windowManager.mainWindow.webContents.send('oauth-callback', { code, state })

              authWindow.loadURL(
                'data:text/html,' +
                  encodeURIComponent(`
                <!DOCTYPE html>
                <html>
                  <head>
                    <title>Authentication Successful</title>
                    <style>
                      body { font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; text-align: center; padding: 40px; }
                      h1 { color: #333; }
                      p { color: #666; }
                      .success { color: #4CAF50; font-weight: bold; }
                    </style>
                  </head>
                  <body>
                    <h1>Authentication Successful</h1>
                    <p class="success">You have successfully authenticated!</p>
                    <p>You can close this window and return to the application.</p>
                  </body>
                </html>
              `)
              )

              setTimeout(() => {
                try {
                  if (!authWindow.isDestroyed()) {
                    authWindow.close()
                  }
                } catch (err) {
                  log.error('[OAuth] Error closing auth window:', err)
                }
              }, 3000)
            }
          } catch (err) {
            log.error('[OAuth] Error parsing callback URL:', err)
          }

          callback({})
        })

        authWindow.on('closed', () => {
          log.info('[OAuth] Auth window closed')
          app.removeAllListeners('certificate-error')
        })

        authWindow.webContents.on(
          'did-fail-load',
          (_, errorCode, errorDescription, validatedURL) => {
            log.error(
              `[OAuth] Failed to load URL: ${validatedURL}. Error: ${errorCode} - ${errorDescription}`
            )
          }
        )

        log.info(`[OAuth] Loading auth URL in BrowserWindow: ${authUrl}`)
        authWindow.loadURL(authUrl).catch((err) => {
          log.error('[OAuth] Error loading auth URL:', err)
        })

        return
      }

      startOAuthCallbackServer(callbackPath)
        .then((server) => {
          oauthServer = server
          log.info('[OAuth] Server started, opening auth URL in browser')
          return shell.openExternal(authUrl)
        })
        .then(() => {
          log.info('[OAuth] Opened auth URL in default browser')
        })
        .catch((err) => {
          log.error('[OAuth] Error in OAuth flow:', err)
        })
    } catch (err) {
      log.error('[OAuth] Failed to parse redirectUri', err)

      shell
        .openExternal(authUrl)
        .then(() => log.info('[OAuth] Opened auth URL in default browser'))
        .catch((err) => log.error('[OAuth] Failed to open auth URL in default browser', err))
    }
  } else {
    shell
      .openExternal(authUrl)
      .then(() => log.info('[OAuth] Opened auth URL in default browser'))
      .catch((err) => log.error('[OAuth] Failed to open auth URL in default browser', err))
  }
}

export function cleanupOAuthServer() {
  if (oauthServer) {
    log.info('Closing OAuth callback server...')
    oauthServer.close()
    oauthServer = null
  }
}
