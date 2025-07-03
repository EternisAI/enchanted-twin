import { app, shell, BrowserWindow } from 'electron'
import http from 'http'
import log from 'electron-log/main'
import { windowManager } from './windows'

const DEFAULT_OAUTH_SERVER_PORT = 8080

let oauthServer: http.Server | null = null

const getLoginPageHTML = () => `
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Sign in to Enchanted</title>
  <style>
    * {
      margin: 0;
      padding: 0;
      box-sizing: border-box;
    }
    
    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
      background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
      min-height: 100vh;
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 20px;
    }
    
    .login-container {
      background: white;
      border-radius: 16px;
      padding: 48px;
      box-shadow: 0 20px 40px rgba(0, 0, 0, 0.1);
      max-width: 400px;
      width: 100%;
      text-align: center;
    }
    
    .logo {
      width: 64px;
      height: 64px;
      background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
      border-radius: 16px;
      margin: 0 auto 24px;
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 24px;
      color: white;
      font-weight: bold;
    }
    
    h1 {
      font-size: 28px;
      font-weight: 600;
      color: #1a1a1a;
      margin-bottom: 8px;
    }
    
    p {
      color: #666;
      margin-bottom: 32px;
      font-size: 16px;
      line-height: 1.5;
    }
    
    .google-btn {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 100%;
      padding: 16px 24px;
      border: 2px solid #e5e7eb;
      border-radius: 12px;
      background: white;
      color: #374151;
      font-size: 16px;
      font-weight: 500;
      text-decoration: none;
      transition: all 0.2s ease;
      cursor: pointer;
    }
    
    .google-btn:hover {
      border-color: #d1d5db;
      box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
      transform: translateY(-1px);
    }
    
    .google-icon {
      width: 20px;
      height: 20px;
      margin-right: 12px;
    }
    
    .loading {
      opacity: 0.7;
      cursor: not-allowed;
    }
    
    .footer {
      margin-top: 32px;
      padding-top: 24px;
      border-top: 1px solid #e5e7eb;
      font-size: 14px;
      color: #9ca3af;
    }
  </style>
</head>
<body>
  <div class="login-container">
    <div class="logo">E</div>
    <h1>Welcome to Enchanted</h1>
    <p>Sign in with your Google account to continue</p>
    
    <button id="google-signin" class="google-btn">
      <svg class="google-icon" viewBox="0 0 24 24">
        <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
        <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
        <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"/>
        <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/>
      </svg>
      Continue with Google
    </button>
    
    <div class="footer">
      Secure authentication powered by Firebase
    </div>
  </div>
  
  <script type="module">
    import { initializeApp } from 'https://www.gstatic.com/firebasejs/11.10.0/firebase-app.js'
    import { getAuth, GoogleAuthProvider, signInWithPopup } from 'https://www.gstatic.com/firebasejs/11.10.0/firebase-auth.js'
    
    // Firebase config - these will be replaced with actual values
    const firebaseConfig = {
      apiKey: "{{FIREBASE_API_KEY}}",
      authDomain: "{{FIREBASE_AUTH_DOMAIN}}",
      projectId: "{{FIREBASE_PROJECT_ID}}"
    }
    
    const app = initializeApp(firebaseConfig)
    const auth = getAuth(app)
    const provider = new GoogleAuthProvider()
    
    // Add Gmail scope
    provider.addScope('https://www.googleapis.com/auth/gmail.readonly')
    
    document.getElementById('google-signin').addEventListener('click', async () => {
      const button = document.getElementById('google-signin')
      button.classList.add('loading')
      button.textContent = 'Signing in...'
      
      try {
        const result = await signInWithPopup(auth, provider)
        const user = result.user
        const credential = GoogleAuthProvider.credentialFromResult(result)
        
        // Send success back to Electron main process
        const params = new URLSearchParams({
          success: 'true',
          user: JSON.stringify({
            uid: user.uid,
            email: user.email,
            displayName: user.displayName,
            photoURL: user.photoURL,
            accessToken: credential?.accessToken || '',
            idToken: credential?.idToken || ''
          })
        })
        
        window.location.href = '/auth-success?' + params.toString()
        
      } catch (error) {
        console.error('Authentication failed:', error)
        button.classList.remove('loading')
        button.innerHTML = \`
          <svg class="google-icon" viewBox="0 0 24 24">
            <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
            <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
            <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"/>
            <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/>
          </svg>
          Try again
        \`
        
        const params = new URLSearchParams({
          error: error.message
        })
        
        window.location.href = '/auth-error?' + params.toString()
      }
    })
  </script>
</body>
</html>
`

interface FirebaseConfig {
  apiKey?: string
  authDomain?: string
  projectId?: string
}

function startOAuthServer(firebaseConfig?: FirebaseConfig): Promise<http.Server> {
  return new Promise((resolve, reject) => {
    if (oauthServer) {
      oauthServer.close()
      oauthServer = null
    }

    const server = http.createServer((req, res) => {
      log.info(`[OAuth] Received request: ${req.url}`)

      if (req.url === '/' || req.url === '/login') {
        // Serve the login page
        res.writeHead(200, { 'Content-Type': 'text/html' })
        let html = getLoginPageHTML()

        // Replace Firebase config placeholders
        if (firebaseConfig) {
          html = html.replace('{{FIREBASE_API_KEY}}', firebaseConfig.apiKey || '')
          html = html.replace('{{FIREBASE_AUTH_DOMAIN}}', firebaseConfig.authDomain || '')
          html = html.replace('{{FIREBASE_PROJECT_ID}}', firebaseConfig.projectId || '')
        }

        res.end(html)
      } else if (req.url && req.url.startsWith('/auth-success')) {
        // Handle successful authentication
        log.info(`[OAuth] Received auth success: ${req.url}`)

        try {
          const parsedUrl = new URL(`http://localhost:${DEFAULT_OAUTH_SERVER_PORT}${req.url}`)
          const userDataString = parsedUrl.searchParams.get('user')

          if (userDataString && windowManager.mainWindow) {
            const userData = JSON.parse(userDataString)
            log.info(
              '[OAuth] 📡 Sending firebase-auth-success to renderer with user:',
              userData.email
            )
            windowManager.mainWindow.webContents.send('firebase-auth-success', userData)

            res.writeHead(200, { 'Content-Type': 'text/html' })
            res.end(`
              <!DOCTYPE html>
              <html>
                <head>
                  <title>Authentication Successful</title>
                  <style>
                    body { 
                      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; 
                      text-align: center; 
                      padding: 40px; 
                      background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
                      min-height: 100vh;
                      display: flex;
                      align-items: center;
                      justify-content: center;
                      margin: 0;
                    }
                    .container {
                      background: white;
                      padding: 48px;
                      border-radius: 16px;
                      box-shadow: 0 20px 40px rgba(0, 0, 0, 0.1);
                    }
                    h1 { color: #333; margin-bottom: 16px; }
                    p { color: #666; }
                    .success { color: #4CAF50; font-weight: bold; font-size: 18px; }
                  </style>
                </head>
                <body>
                  <div class="container">
                    <h1>Welcome to Enchanted!</h1>
                    <p class="success">You have successfully signed in with Google!</p>
                    <p>You can close this window and return to the application.</p>
                  </div>
                  <script>
                    setTimeout(() => {
                      window.close()
                    }, 3000)
                  </script>
                </body>
              </html>
            `)

            setTimeout(() => {
              if (oauthServer) {
                oauthServer.close()
                oauthServer = null
                log.info('[OAuth] Server closed after successful authentication')
              }
            }, 5000)
          }
        } catch (err) {
          log.error('[OAuth] Failed to parse auth success data', err)
          res.writeHead(500, { 'Content-Type': 'text/plain' })
          res.end('Internal Server Error')
        }
      } else if (req.url && req.url.startsWith('/auth-error')) {
        // Handle authentication error
        log.info(`[OAuth] Received auth error: ${req.url}`)

        try {
          const parsedUrl = new URL(`http://localhost:${DEFAULT_OAUTH_SERVER_PORT}${req.url}`)
          const error = parsedUrl.searchParams.get('error')

          if (windowManager.mainWindow) {
            log.info('[OAuth] 📡 Sending firebase-auth-error to renderer:', error)
            windowManager.mainWindow.webContents.send('firebase-auth-error', { error })
          }

          res.writeHead(200, { 'Content-Type': 'text/html' })
          res.end(`
            <!DOCTYPE html>
            <html>
              <head>
                <title>Authentication Error</title>
                <style>
                  body { 
                    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; 
                    text-align: center; 
                    padding: 40px; 
                    background: linear-gradient(135deg, #ff6b6b 0%, #ee5a24 100%);
                    min-height: 100vh;
                    display: flex;
                    align-items: center;
                    justify-content: center;
                    margin: 0;
                  }
                  .container {
                    background: white;
                    padding: 48px;
                    border-radius: 16px;
                    box-shadow: 0 20px 40px rgba(0, 0, 0, 0.1);
                  }
                  h1 { color: #333; margin-bottom: 16px; }
                  p { color: #666; }
                  .error { color: #e74c3c; font-weight: bold; }
                </style>
              </head>
              <body>
                <div class="container">
                  <h1>Authentication Failed</h1>
                  <p class="error">There was an error signing in: ${error || 'Unknown error'}</p>
                  <p>Please close this window and try again.</p>
                </div>
              </body>
            </html>
          `)
        } catch (err) {
          log.error('[OAuth] Failed to parse auth error', err)
          res.writeHead(500, { 'Content-Type': 'text/plain' })
          res.end('Internal Server Error')
        }
      } else {
        // Handle other requests
        res.writeHead(404, { 'Content-Type': 'text/plain' })
        res.end('Not Found')
      }
    })

    server.on('error', (err) => {
      log.error(`[OAuth] Server error on port ${DEFAULT_OAUTH_SERVER_PORT}:`, err)
      reject(err)
    })

    server.listen(DEFAULT_OAUTH_SERVER_PORT, 'localhost', () => {
      log.info(`[OAuth] OAuth server listening on http://localhost:${DEFAULT_OAUTH_SERVER_PORT}`)
      resolve(server)
    })
  })
}

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

export function startFirebaseOAuth(firebaseConfig: FirebaseConfig): Promise<string> {
  return new Promise((resolve, reject) => {
    startOAuthServer(firebaseConfig)
      .then((server) => {
        oauthServer = server
        const loginUrl = `http://localhost:${DEFAULT_OAUTH_SERVER_PORT}/login`
        log.info(`[OAuth] Firebase OAuth server started at ${loginUrl}`)
        resolve(loginUrl)
      })
      .catch(reject)
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
        .catch((err) => {
          log.error('[OAuth] Failed to start OAuth callback server:', err)
        })
    } catch (err) {
      log.error('[OAuth] Failed to parse redirect URI:', err)
      shell
        .openExternal(authUrl)
        .then(() => log.info('[OAuth] Opened auth URL in default browser'))
        .catch((err) => log.error('[OAuth] Failed to open auth URL in default browser', err))
    }
  } else {
    log.error('[OAuth] No redirect URI provided')
    shell
      .openExternal(authUrl)
      .then(() => log.info('[OAuth] Opened auth URL in default browser'))
      .catch((err) => log.error('[OAuth] Failed to open auth URL in default browser', err))
  }
}

export function cleanupOAuthServer() {
  if (oauthServer) {
    log.info('[OAuth] Cleaning up OAuth server')
    oauthServer.close()
    oauthServer = null
  }
}
