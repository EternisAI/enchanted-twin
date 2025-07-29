import { spawn, ChildProcess } from 'child_process'
import log from 'electron-log/main'
import { existsSync, mkdirSync } from 'fs'
import { join } from 'path'
import { app } from 'electron'
import { BrowserWindow } from 'electron'
import split2 from 'split2'

import { createErrorWindow, waitForBackend } from './helpers'
import { capture } from './analytics'
import { startLlamaCppSetup, cleanupLlamaCpp } from './llamaCppServer'

let goServerProcess: ChildProcess | null = null
let isInitializing = false

export async function initializeGoServer(IS_PRODUCTION: boolean, DEFAULT_BACKEND_PORT: number) {
  console.log('IS_PRODUCTION', IS_PRODUCTION)

  if (goServerProcess && !goServerProcess.killed) {
    log.info('[GO] Go server is already running, skipping initialization')
    return true
  }

  if (isInitializing) {
    log.info('[GO] Go server is already initializing, skipping duplicate initialization')
    return true
  }

  isInitializing = true

  try {
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

    startLlamaCppSetup()

    if (IS_PRODUCTION) {
      const success = await startGoServer(goBinaryPath, userDataPath, dbPath, DEFAULT_BACKEND_PORT)
      return success
    } else {
      //TODO: Remove this once we have a production build
      const success = await startGoServer(goBinaryPath, userDataPath, dbPath, DEFAULT_BACKEND_PORT)
      return success
    }
  } finally {
    isInitializing = false
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

    capture('server_startup_error', {
      error_type: 'binary_not_found',
      binary_path: goBinaryPath
    })

    return false
  }
  log.info(`Attempting to start Go server at: ${goBinaryPath}`)

  const startTime = Date.now()

  try {
    goServerProcess = spawn(goBinaryPath, [], {
      env: {
        ...process.env,
        APP_DATA_PATH: userDataPath,
        DB_PATH: dbPath,
        COMPLETIONS_API_URL: process.env.COMPLETIONS_API_URL,
        COMPLETIONS_API_KEY: process.env.COMPLETIONS_API_KEY,
        COMPLETIONS_MODEL: process.env.COMPLETIONS_MODEL,
        REASONING_MODEL: process.env.REASONING_MODEL,
        EMBEDDINGS_API_URL: process.env.EMBEDDINGS_API_URL,
        EMBEDDINGS_API_KEY: process.env.EMBEDDINGS_API_KEY,
        EMBEDDINGS_MODEL: process.env.EMBEDDINGS_MODEL,
        TELEGRAM_CHAT_SERVER: process.env.TELEGRAM_CHAT_SERVER,
        ENCHANTED_MCP_URL: process.env.ENCHANTED_MCP_URL,
        PROXY_TEE_URL: process.env.PROXY_TEE_URL,
        HOLON_API_URL: process.env.HOLON_API_URL,
        ANONYMIZER_TYPE: process.env.ANONYMIZER_TYPE,
        USE_LOCAL_EMBEDDINGS: process.env.USE_LOCAL_EMBEDDINGS,
        DISABLE_ONBOARDING: process.env.VITE_DISABLE_ONBOARDING,
        DISABLE_HOLONS: process.env.VITE_DISABLE_HOLONS,
        DISABLE_TASKS: process.env.VITE_DISABLE_TASKS,
        DISABLE_CONNECTORS: process.env.VITE_DISABLE_CONNECTORS,
        DISABLE_VOICE: process.env.VITE_DISABLE_VOICE
      }
    })

    if (goServerProcess) {
      goServerProcess.on('error', (err) => {
        log.error('Failed to start Go server, on error:', err)

        capture('server_startup_error', {
          error_type: 'spawn_error',
          error_message: err instanceof Error ? err.message : 'Unknown error',
          duration: Date.now() - startTime
        })

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

      const duration = Date.now() - startTime
      log.info(`[GO] Go server process ready in ${duration}ms`)

      capture('server_startup_success', {
        duration: duration,
        port: backendPort
      })

      return true
    } else {
      log.error('Failed to spawn Go server process.')

      capture('server_startup_error', {
        error_type: 'spawn_failed',
        error_message: 'Failed to spawn Go server process',
        duration: Date.now() - startTime
      })

      return false
    }
  } catch (error: unknown) {
    log.error('Error spawning Go server:', error)

    capture('server_startup_error', {
      error_type: 'general_error',
      error_message: error instanceof Error ? error.message : 'Unknown error',
      duration: Date.now() - startTime
    })

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

  cleanupLlamaCpp()
}

export function isGoServerRunning(): boolean {
  return goServerProcess !== null && !goServerProcess.killed
}

export function getGoServerState() {
  return {
    isRunning: isGoServerRunning(),
    isInitializing
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
