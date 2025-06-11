import { spawn, ChildProcess } from 'child_process'
import log from 'electron-log/main'
import { existsSync, mkdirSync } from 'fs'
import { join } from 'path'
import { app } from 'electron'
import { createErrorWindow, waitForBackend, createSplashWindow } from './helpers'
import { BrowserWindow } from 'electron'
import split2 from 'split2'

let goServerProcess: ChildProcess | null = null
let splashWindow: BrowserWindow | null = null

export async function initializeGoServer(IS_PRODUCTION: boolean, DEFAULT_BACKEND_PORT: number) {
  // Create and show splash screen
  splashWindow = createSplashWindow()

  const userDataPath = app.getPath('userData')
  const dbDir = join(userDataPath, 'db')

  if (!existsSync(dbDir)) {
    try {
      mkdirSync(dbDir, { recursive: true })
      log.info(`Created database directory: ${dbDir}`)
    } catch (err) {
      log.error(`Failed to create database directory: ${err}`)
    }
  }

  const dbPath = join(dbDir, 'enchanted-twin.db')
  log.info(`Database path: ${dbPath}`)

  const executable = process.platform === 'win32' ? 'enchanted-twin.exe' : 'enchanted-twin'
  const goBinaryPath = !IS_PRODUCTION
    ? join(__dirname, '..', '..', 'resources', executable)
    : join(process.resourcesPath, 'resources', executable)

  if (IS_PRODUCTION) {
    const success = await startGoServer(goBinaryPath, userDataPath, dbPath, DEFAULT_BACKEND_PORT)
    // Close splash screen after Go server is ready
    if (splashWindow && !splashWindow.isDestroyed()) {
      splashWindow.close()
      splashWindow = null
    }
    return success
  } else {
    log.info('Running in development mode - packaged Go server not started')
    // Close splash screen in development mode
    if (splashWindow && !splashWindow.isDestroyed()) {
      splashWindow.close()
      splashWindow = null
    }
    return true
  }
}

async function startGoServer(
  goBinaryPath: string,
  userDataPath: string,
  dbPath: string,
  backendPort: number
) {
  if (!existsSync(goBinaryPath)) {
    log.error(`Go binary not found at: ${goBinaryPath}`)
    createErrorWindow(`Go binary not found at: ${goBinaryPath}`)
    return false
  }
  log.info(`Attempting to start Go server at: ${goBinaryPath}`)

  try {
    goServerProcess = spawn(goBinaryPath, [], {
      env: {
        ...process.env,
        APP_DATA_PATH: userDataPath,
        DB_PATH: dbPath,
        COMPLETIONS_API_URL: process.env.COMPLETIONS_API_URL,
        COMPLETIONS_MODEL: process.env.COMPLETIONS_MODEL,
        REASONING_MODEL: process.env.REASONING_MODEL,
        EMBEDDINGS_API_URL: process.env.EMBEDDINGS_API_URL,
        EMBEDDINGS_MODEL: process.env.EMBEDDINGS_MODEL,
        TELEGRAM_TOKEN: process.env.TELEGRAM_TOKEN,
        TELEGRAM_CHAT_SERVER: process.env.TELEGRAM_CHAT_SERVER,
        ENCHANTED_MCP_URL: process.env.ENCHANTED_MCP_URL,
        INVITE_SERVER_URL: process.env.INVITE_SERVER_URL
      }
    })

    if (goServerProcess) {
      goServerProcess.on('error', (err) => {
        log.error('Failed to start Go server, on error:', err)
        createErrorWindow(
          `Failed to start Go server: ${err instanceof Error ? err.message : 'Unknown error'}`
        )
      })

      goServerProcess.on('close', (code) => {
        log.info(`Go server process exited with code ${code}`)
        goServerProcess = null
      })

      if (goServerProcess.stderr) {
        forward(goServerProcess.stderr, 'stderr')
      }

      goServerProcess.stdout?.on('data', (data) => {
        log.info(`Go Server stdout: ${data.toString().trim()}`)
      })
      goServerProcess.stderr?.on('data', (data) => {
        log.error(`Go Server stderr: ${data.toString().trim()}`)
      })

      log.info('Go server process spawned. Waiting until it listens …')
      await waitForBackend(backendPort)
      return true
    } else {
      log.error('Failed to spawn Go server process.')
      return false
    }
  } catch (error: unknown) {
    log.error('Error spawning Go server:', error)
    createErrorWindow(
      `Failed to start Go server: ${error instanceof Error ? error.message : 'Unknown error'}`
    )
    return false
  }
}

export function cleanupGoServer() {
  if (goServerProcess) {
    log.info('Attempting to kill Go server process...')
    const killed = goServerProcess.kill()
    if (killed) {
      log.info('Go server process killed successfully.')
    } else {
      log.warn('Failed to kill Go server process. It might have already exited.')
    }
    goServerProcess = null
  }
}

function forward(stream: NodeJS.ReadableStream, source: 'stdout' | 'stderr') {
  stream.pipe(split2()).on('data', (line: string) => {
    if (!line) return
    // const clean = stripAnsi(line)   // zap ANSI color ESCs
    log[source === 'stdout' ? 'info' : 'error'](`Go ${source}: ${line}`)

    BrowserWindow.getAllWindows().forEach((win) =>
      win.webContents.send('go-log', { source, line: line })
    )
  })
}
