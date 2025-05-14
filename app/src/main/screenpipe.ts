import { ChildProcess, exec, execSync, spawn } from 'child_process'
import { platform, homedir } from 'os'
import path from 'path'
import log from 'electron-log/main'
import { ipcMain } from 'electron'

let screenpipeProcess: ChildProcess | null = null

export function startScreenpipe(): Promise<{ success: boolean; error?: string }> {
  console.log('Starting screenpipe!!')
  return new Promise((resolve) => {
    if (screenpipeProcess) {
      log.warn('Screenpipe is already running.')
      resolve({ success: false, error: 'Screenpipe is already running.' })
      return
    }

    const isWindows = platform() === 'win32'
    const homeDir = homedir()
    log.info(`Screenpipe running from ${homeDir}`)

    try {
      const screenpipeBinaryPath = isWindows
        ? path.join(homeDir, 'screenpipe', 'bin', 'screenpipe.exe')
        : path.join(homeDir, '.local', 'bin', 'screenpipe')

      const screenpipeArgs = [
        '--disable-audio'
        // '--monitor-id',
        // '1',
      ]

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
          resolve({ success: true })
        })

        screenpipeProcess.on('exit', (code) => {
          log.info('Screenpipe process exited with code:', code)
          screenpipeProcess = null
        })

        screenpipeProcess.on('error', (err) => {
          log.error('Screenpipe process error:', err)
          screenpipeProcess = null
          resolve({ success: false, error: err.message })
        })
      }
    } catch (error) {
      log.error('Error starting screenpipe:', error)
      resolve({
        success: false,
        error: error instanceof Error ? error.message : 'Unknown error'
      })
    }
  })
}

export function installScreenpipe(): Promise<{ success: boolean; error?: string }> {
  const isWindows = platform() === 'win32'
  const installCommand = isWindows
    ? 'powershell -Command "iwr get.screenpi.pe/cli.ps1 | iex"'
    : 'curl -A "ScreenpipeInstaller/1.0" -fsSL get.screenpi.pe/cli | zsh'

  return new Promise((resolve) => {
    exec(installCommand, (err, stdout, stderr) => {
      if (err) {
        log.error(`Failed to install screenpipe: ${err}`)
        resolve({ success: false, error: `Failed to install screenpipe: ${err.message}` })
        return
      }

      if (stderr) {
        log.error(`Install stderr: ${stderr}`)
      }

      if (stdout) {
        log.info(`Install stdout: ${stdout}`)
      }

      log.info('Screenpipe installation complete')
      resolve({ success: true })
    })
  })
}

function isScreenpipeInstalled(): boolean {
  const isWindows = platform() === 'win32'
  const checkCommand = isWindows ? 'where screenpipe' : 'which screenpipe'

  try {
    const home = homedir()
    const localBin = path.join(home, '.local', 'bin')
    process.env.PATH = `${process.env.PATH}:${localBin}`
    log.info('Application PATH', process.env.PATH)
    execSync(checkCommand)
    log.info('Screenpipe already installed')
    return true
  } catch (error) {
    log.warn(`Screenpipe not found: ${error}`)
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

export function cleanupScreenpipe(): void {
  if (screenpipeProcess) {
    log.info('Shutting down screenpipe process...')
    stopScreenpipe()
  }
}

export function registerScreenpipeIpc(): void {
  ipcMain.handle('screenpipe:get-status', () => {
    return {
      isRunning: isScreenpipeRunning(),
      isInstalled: isScreenpipeInstalled()
    }
  })

  ipcMain.handle('screenpipe:install', async () => {
    return await installScreenpipe()
  })

  ipcMain.handle('screenpipe:start', async () => {
    return await startScreenpipe()
  })

  ipcMain.handle('screenpipe:stop', () => {
    return stopScreenpipe()
  })
}
