import { spawn, ChildProcess } from 'child_process'
import log from 'electron-log/main'
import { existsSync } from 'fs'
import { createErrorWindow, waitForBackend } from './helpers'

let goServerProcess: ChildProcess | null = null

export async function startGoServer(
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
        OLLAMA_BASE_URL: process.env.OLLAMA_BASE_URL,
        TELEGRAM_CHAT_SERVER: process.env.TELEGRAM_CHAT_SERVER
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
