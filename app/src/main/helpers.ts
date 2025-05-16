import { BrowserWindow } from 'electron'
import log from 'electron-log/main'
import { nativeTheme } from 'electron/main'
import waitOn from 'wait-on'

export async function waitForBackend(port: number) {
  log.info(`Waiting for backend on tcp:127.0.0.1:${port} …`)
  await waitOn({ resources: [`tcp:127.0.0.1:${port}`], timeout: 15000, delay: 100 })
  log.info('Backend is ready.')
}

export function createErrorWindow(errorMessage: string) {
  const errorWindow = new BrowserWindow({
    width: 800,
    height: 600,
    resizable: false,
    minimizable: false,
    maximizable: false,
    title: 'Application Error',
    webPreferences: {
      nodeIntegration: true
    }
  })

  errorWindow.loadURL(`data:text/html,
      <html>
        <body>
          <h2>Application Error</h2>
          <pre>${errorMessage}</pre>
          <button onclick="window.close()">Close</button>
        </body>
      </html>`)
}

export function createSplashWindow(): BrowserWindow {
  const splashWidth = 450
  const splashHeight = 320

  const computePalette = (dark: boolean) => ({
    bg: dark ? '#000000' : '#f9f9f9',
    text: dark ? '#ffffff' : '#000000',
    track: dark ? '#323232' : '#f0f0f0',
    accent: dark ? '#ffffff' : '#000000'
  })

  let palette = computePalette(nativeTheme.shouldUseDarkColors)

  const splash = new BrowserWindow({
    width: splashWidth,
    height: splashHeight,
    frame: false,
    resizable: false,
    transparent: false,
    backgroundColor: palette.bg,
    center: true,
    show: false,
    alwaysOnTop: true,
    webPreferences: { sandbox: false }
  })

  const html = `<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <title>Loading…</title>
    <style>
      @keyframes spin { to { transform: rotate(360deg); } }

      html, body {
        height: 100%;
        margin: 0;
        font-family: Sen, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
        background: ${palette.bg};
        color: ${palette.text};
        display: flex;
        align-items: center;
        justify-content: center;
        -webkit-app-region: drag;
      }

      .container {
        text-align: center;
        display: flex;
        align-items: center;
        flex-direction: column;
        gap: 20px;
        -webkit-app-region: no-drag;
      }

      .spinner {
        width: 64px;
        height: 64px;
        border: 8px solid ${palette.track};
        border-top-color: ${palette.accent};
        border-radius: 50%;
        animation: spin 1s linear infinite;
      }

      h1 {
        font-size: 1.1rem;
        font-weight: 500;
        margin: 0;
        letter-spacing: 0.02em;
      }
    </style>
  </head>
  <body>
    <div class="container">
      <div class="spinner"></div>
      <h1>Starting&nbsp;Enchanted&nbsp;Twin…</h1>
    </div>
  </body>
</html>`

  splash.loadURL('data:text/html;charset=utf-8,' + encodeURIComponent(html))
  splash.once('ready-to-show', () => splash.show())

  // live‑update palette when OS theme flips
  nativeTheme.on('updated', () => {
    if (splash.isDestroyed()) return
    palette = computePalette(nativeTheme.shouldUseDarkColors)
    splash.webContents.executeJavaScript(`
      document.body.style.background='${palette.bg}';
      document.body.style.color='${palette.text}';
      const sp=document.querySelector('.spinner');
      if(sp){ 
        sp.style.borderColor='${palette.track}'; 
        sp.style.borderTopColor='${palette.accent}'; 
      }
    `)
  })

  return splash
}
