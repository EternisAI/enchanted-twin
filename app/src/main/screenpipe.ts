import { ChildProcess, exec, execSync, spawn } from 'child_process'
import { platform, homedir } from 'os'
import path from 'path'
import log from 'electron-log/main'
import { ipcMain } from 'electron'
import { screenpipeStore } from './stores'

let screenpipeProcess: ChildProcess | null = null
let isScreenpipeCurrentlyInstalled: boolean = false
let isStarting: boolean = false
let healthCheckInterval: NodeJS.Timeout | null = null

async function killExistingScreenpipeProcesses(): Promise<void> {
  const isWindows = platform() === 'win32'

  try {
    if (isWindows) {
      // Kill all screenpipe.exe processes on Windows
      await new Promise((resolve) => {
        exec('taskkill /F /IM screenpipe.exe', (err) => {
          if (err && !err.message.includes('not found')) {
            log.warn('Error killing existing screenpipe processes:', err)
          }
          resolve(undefined)
        })
      })
    } else {
      // Kill all screenpipe processes on macOS/Linux
      // First try pkill
      await new Promise((resolve) => {
        exec('pkill -f screenpipe', (err) => {
          if (err && err.code !== 1) {
            // Exit code 1 means no processes found
            log.warn('Error killing existing screenpipe processes with pkill:', err)
          }
          resolve(undefined)
        })
      })

      // On macOS, also try killall as a fallback
      if (platform() === 'darwin') {
        await new Promise((resolve) => {
          exec('killall -9 screenpipe 2>/dev/null', (err) => {
            // Ignore errors as process might not exist
            log.warn('Error killing existing screenpipe processes with killall:', err)
            resolve(undefined)
          })
        })
      }
    }
  } catch (error) {
    log.error('Failed to kill existing screenpipe processes:', error)
  }
}

// Synchronous version for immediate cleanup
function killExistingScreenpipeProcessesSync(): void {
  const isWindows = platform() === 'win32'

  try {
    if (isWindows) {
      // Kill all screenpipe.exe processes on Windows
      try {
        execSync('taskkill /F /IM screenpipe.exe', { stdio: 'ignore' })
      } catch (error) {
        // Ignore errors
        log.warn('Error killing existing screenpipe processes:', error)
      }
    } else {
      // Kill all screenpipe processes on macOS/Linux
      try {
        execSync('pkill -9 -f screenpipe', { stdio: 'ignore' })
      } catch (error) {
        // Ignore errors
        log.warn('Error killing existing screenpipe processes with pkill:', error)
      }

      // On macOS, also try killall as a fallback
      if (platform() === 'darwin') {
        try {
          execSync('killall -9 screenpipe 2>/dev/null', { stdio: 'ignore' })
        } catch (error) {
          log.warn('Error killing existing screenpipe processes with killall:', error)
          // Ignore errors as process might not exist
        }
      }
    }
  } catch (error) {
    log.warn('Error killing existing screenpipe processes:', error)
    // Ignore errors in sync cleanup
  }
}

export function startScreenpipe(): Promise<{ success: boolean; error?: string }> {
  console.log('Starting screenpipe!!')

  // Check preconditions first
  if (isStarting) {
    log.warn('Screenpipe is already being started.')
    return Promise.resolve({ success: false, error: 'Screenpipe is already being started.' })
  }

  if (screenpipeProcess) {
    log.warn('Screenpipe is already running.')
    return Promise.resolve({ success: false, error: 'Screenpipe is already running.' })
  }

  isStarting = true

  // Kill any existing screenpipe processes first
  log.info('Checking for existing screenpipe processes...')

  return killExistingScreenpipeProcesses().then(() => {
    return new Promise((resolve) => {
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
            isStarting = false
            startHealthCheck() // Start monitoring the process
            resolve({ success: true })
          })

          screenpipeProcess.on('exit', (code) => {
            log.info('Screenpipe process exited with code:', code)
            screenpipeProcess = null
            isStarting = false
            stopHealthCheck() // Stop monitoring
          })

          screenpipeProcess.on('error', (err) => {
            log.error('Screenpipe process error:', err)
            screenpipeProcess = null
            isStarting = false
            resolve({ success: false, error: err.message })
          })
        }
      } catch (error) {
        log.error('Error starting screenpipe:', error)
        isStarting = false
        resolve({
          success: false,
          error: error instanceof Error ? error.message : 'Unknown error'
        })
      }
    })
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
      isScreenpipeCurrentlyInstalled = true
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
    isScreenpipeCurrentlyInstalled = true
    return true
  } catch (error) {
    log.warn(`Screenpipe not found: ${error}`)
    isScreenpipeCurrentlyInstalled = false
    return false
  }
}

function isScreenpipeRunning(): boolean {
  // First check our internal state
  if (!screenpipeProcess || !screenpipeProcess.pid) {
    return false
  }

  // Then verify the process is actually alive
  try {
    // Sending signal 0 checks if process exists without affecting it
    process.kill(screenpipeProcess.pid, 0)
    return true
  } catch (error) {
    // Process doesn't exist, clean up our reference
    // ESRCH is expected when the process is gone
    if ((error as NodeJS.ErrnoException).code !== 'ESRCH') {
      log.warn('Error checking screenpipe process:', error)
    }
    screenpipeProcess = null
    return false
  }
}

function stopScreenpipe(): boolean {
  if (screenpipeProcess) {
    const pid = screenpipeProcess.pid
    log.info(`Stopping screenpipe process with PID: ${pid}`)

    stopHealthCheck() // Stop monitoring

    try {
      // First try SIGTERM for graceful shutdown
      screenpipeProcess.kill('SIGTERM')

      // Set a timeout to force kill if it doesn't exit gracefully
      setTimeout(() => {
        if (screenpipeProcess && !screenpipeProcess.killed) {
          log.warn('Screenpipe did not exit gracefully, force killing...')
          screenpipeProcess.kill('SIGKILL')
        }
      }, 2000)

      // On macOS/Linux, also try to kill the entire process group
      if (pid && process.platform !== 'win32') {
        try {
          // Kill the process group (negative PID kills the entire group)
          process.kill(-pid, 'SIGTERM')
          setTimeout(() => {
            try {
              process.kill(-pid, 'SIGKILL')
            } catch (error) {
              // ESRCH means the process doesn't exist, which is fine
              if ((error as NodeJS.ErrnoException).code !== 'ESRCH') {
                log.warn('Failed to kill process group:', error)
              }
            }
          }, 2000)
        } catch (error) {
          // ESRCH means the process doesn't exist, which is fine - it's already dead
          if ((error as NodeJS.ErrnoException).code !== 'ESRCH') {
            log.error('Failed to kill process group:', error)
          }
        }
      }

      // On Windows, use taskkill to ensure all child processes are terminated
      if (pid && process.platform === 'win32') {
        exec(`taskkill /F /T /PID ${pid}`, (err) => {
          if (err) {
            log.error('Failed to kill process tree on Windows:', err)
          }
        })
      }
    } catch (error) {
      log.error('Error stopping screenpipe:', error)
    }

    screenpipeProcess = null
    return true
  }
  return false
}

export function cleanupScreenpipe(): void {
  stopHealthCheck()
  if (screenpipeProcess) {
    log.info('Shutting down screenpipe process...')
    stopScreenpipe()
  }
  // As a fallback, also kill any screenpipe processes by name
  // This ensures cleanup even if we lost track of the process
  killExistingScreenpipeProcesses().catch((error) => {
    log.error('Error killing existing screenpipe processes during cleanup:', error)
  })
}

// Synchronous cleanup for immediate termination
export function cleanupScreenpipeSync(): void {
  stopHealthCheck()
  if (screenpipeProcess) {
    log.info('Shutting down screenpipe process (sync)...')
    try {
      // Try to kill our tracked process
      if (screenpipeProcess.pid) {
        process.kill(screenpipeProcess.pid, 'SIGKILL')
      }
    } catch (error) {
      // ESRCH means the process doesn't exist, which is fine
      if ((error as NodeJS.ErrnoException).code !== 'ESRCH') {
        log.warn('Error killing screenpipe process (sync):', error)
      }
    }
  }
  // Always kill by name as fallback
  killExistingScreenpipeProcessesSync()
}

// Start health check when process starts
function startHealthCheck() {
  if (healthCheckInterval) {
    clearInterval(healthCheckInterval)
  }

  healthCheckInterval = setInterval(() => {
    if (screenpipeProcess && !isScreenpipeRunning()) {
      log.warn('Screenpipe process died unexpectedly, cleaning up')
      screenpipeProcess = null
      stopHealthCheck()
    }
  }, 10000) // Check every 10 seconds
}

function stopHealthCheck() {
  if (healthCheckInterval) {
    clearInterval(healthCheckInterval)
    healthCheckInterval = null
  }
}

export function autoStartScreenpipeIfEnabled(): Promise<void> {
  return new Promise((resolve) => {
    const autoStart = screenpipeStore.get('autoStart') as boolean

    if (!autoStart) {
      log.info('[Screenpipe] auto-start is disabled')
      resolve()
      return
    }

    if (!isScreenpipeInstalled()) {
      log.warn('[Screenpipe] auto-start is enabled but screenpipe is not installed')
      resolve()
      return
    }

    log.info('[Screenpipe] Auto-starting screenpipe...')
    startScreenpipe()
      .then((result) => {
        if (result.success) {
          log.info('[Screenpipe] auto-started successfully')
        } else {
          log.error('[Screenpipe] Failed to auto-start:', result.error)
        }
        resolve()
      })
      .catch((error) => {
        log.error('[Screenpipe] Error during auto-start:', error)
        resolve()
      })
  })
}

export function registerScreenpipeIpc(): void {
  ipcMain.handle('screenpipe:get-status', async () => {
    // Double-check the actual running state
    const isRunning = isScreenpipeRunning()

    // If we think it's running but it's not, clean up
    if (!isRunning && screenpipeProcess) {
      screenpipeProcess = null
    }

    return {
      isRunning,
      isInstalled: isScreenpipeCurrentlyInstalled || isScreenpipeInstalled()
    }
  })

  ipcMain.handle('screenpipe:install', async () => {
    return await installScreenpipe()
  })

  ipcMain.handle('screenpipe:start', async () => {
    const result = await startScreenpipe()
    if (result.success) {
      screenpipeStore.set('autoStart', true)
      log.info('Screenpipe auto-start enabled after manual start')
    }
    return result
  })

  ipcMain.handle('screenpipe:stop', () => {
    const stopped = stopScreenpipe()
    if (stopped) {
      screenpipeStore.set('autoStart', false)
      log.info('Screenpipe auto-start disabled after manual stop')
    }
    return stopped
  })

  ipcMain.handle('screenpipe:get-auto-start', () => {
    return screenpipeStore.get('autoStart')
  })

  ipcMain.handle('screenpipe:set-auto-start', (_, enabled: boolean) => {
    screenpipeStore.set('autoStart', enabled)
    log.info(`[Screenpipe] auto-start ${enabled ? 'enabled' : 'disabled'}`)
    return enabled
  })

  ipcMain.handle('screenpipe:store-restart-intent', (_, route: string, showModal: boolean) => {
    screenpipeStore.set('restartIntent', { route, showModal })
    log.info(`[Screenpipe] stored restart intent: ${route}, modal: ${showModal}`)
    return true
  })
}
