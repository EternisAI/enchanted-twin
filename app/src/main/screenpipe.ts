import { ChildProcess, exec, execSync, spawn } from 'child_process'
import { platform, homedir } from 'os'
import path from 'path'
import log from 'electron-log/main'
import { ipcMain } from 'electron'
import { createErrorWindow } from './helpers'

let screenpipeProcess: ChildProcess | null = null

export function startScreenpipe(): void {
  const isWindows = platform() === 'win32'
  const homeDir = homedir()
  log.info(`Screenpipe running from ${homeDir}`)

  try {
    const screenpipeBinaryPath = isWindows
      ? path.join(homeDir, 'screenpipe', 'bin', 'screenpipe.exe')
      : path.join(homeDir, '.local', 'bin', 'screenpipe')

    const screenpipeArgs = [`--disable-audio`]

    screenpipeProcess = spawn(screenpipeBinaryPath, screenpipeArgs, {
      stdio: 'pipe',
      env: process.env
    })

    log.info(`Screenpipe process spawned with PID: ${screenpipeProcess?.pid}`)

    if (screenpipeProcess) {
      screenpipeProcess.stdout?.on('data', (data) => {
        log.info('Screenpipe stdout:', data.toString())
      })

      screenpipeProcess.stderr?.on('data', (data) => {
        log.error('Screenpipe stderr:', data.toString())
      })

      screenpipeProcess.on('spawn', () => {
        log.info('Screenpipe process spawned with PID:', screenpipeProcess?.pid)
      })

      screenpipeProcess.on('exit', (code) => {
        log.info('Screenpipe process exited with code:', code)
        screenpipeProcess = null
      })

      screenpipeProcess.on('error', (err) => {
        log.error('Screenpipe process error:', err)
        screenpipeProcess = null
      })
    }
  } catch (error) {
    log.error('Error starting screenpipe:', error)
    createErrorWindow(
      `Error starting screenpipe: ${error instanceof Error ? error.message : 'Unknown error'}`
    )
  }
}

export function installAndStartScreenpipe(): Promise<void> {
  const isWindows = platform() === 'win32'
  const installCommand = isWindows
    ? 'powershell -Command "iwr get.screenpi.pe/cli.ps1 | iex"'
    : 'curl -fsSL get.screenpi.pe/cli | sh'

  return new Promise((resolve, reject) => {
    exec(installCommand, (err, stdout, stderr) => {
      if (err && stderr) {
        log.error(`Failed to install screenpipe: ${err} ${stderr}`)
        reject(new Error(`Failed to install screenpipe: ${err} ${stderr}`))
      } else {
        log.info('Screenpipe installed successfully')
        startScreenpipe()
        resolve()
      }
    })
  })
}

function isScreenpipeInstalled(): boolean {
  const isWindows = platform() === 'win32'
  const checkCommand = isWindows ? 'where screenpipe' : 'which screenpipe'

  try {
    execSync(checkCommand)
    log.info('Screenpipe already installed')
    return true
  } catch (error) {
    log.error(`Failed to check for screenpipe: ${error}`)
    return false
  }
}

function isScreenpipeRunning(): boolean {
  return screenpipeProcess !== null
}

function stopScreenpipe(): boolean {
  if (screenpipeProcess) {
    screenpipeProcess.kill()
    screenpipeProcess = null
    return true
  }
  return false
}

export async function startAndInstallScreenpipe(): Promise<void> {
  if (!isScreenpipeInstalled()) {
    await installAndStartScreenpipe()
  } else if (!isScreenpipeRunning()) {
    startScreenpipe()
  }
}

export function cleanupScreenpipe(): void {
  if (screenpipeProcess) {
    log.info('Shutting down screenpipe process...')
    stopScreenpipe()
  }
}

export function registerScreenpipeIpc(): void {
  ipcMain.handle('screenpipe:get-status', () => {
    return isScreenpipeRunning()
  })

  ipcMain.handle('screenpipe:start', async () => {
    await startScreenpipe()
    return true
  })

  ipcMain.handle('screenpipe:stop', () => {
    return stopScreenpipe()
  })
}
