import { ChildProcess, spawn } from 'child_process'
import log from 'electron-log/main'
import { join } from 'path'
import fs from 'fs'
import { is } from '@electron-toolkit/utils'

let podmanProcess: ChildProcess | null = null
const IS_PRODUCTION = process.env.IS_PROD_BUILD === 'true' || !is.dev

/**
 * Checks if Podman is installed on the system
 */
export async function isPodmanInstalled(): Promise<boolean> {
  return new Promise((resolve) => {
    log.info('Checking for Podman in standard locations...')

    // Common paths where podman might be installed
    const podmanPaths = [
      '/usr/local/bin/podman',
      '/usr/bin/podman',
      '/opt/podman/bin/podman',
      '/opt/homebrew/bin/podman'
    ]

    // First check known paths
    for (const path of podmanPaths) {
      if (fs.existsSync(path)) {
        log.info(`Found Podman at ${path}`)
        resolve(true)
        return
      }
    }

    // Fall back to checking via command
    const checkProcess = spawn('which', ['podman'])

    checkProcess.stdout.on('data', (data) => {
      const path = data.toString().trim()
      log.info(`Podman found at: ${path}`)
      if (path) {
        resolve(true)
      }
    })

    checkProcess.on('close', (code) => {
      if (code === 0) {
        resolve(true)
      } else {
        // Try running podman directly as last resort
        const podmanCheck = spawn('podman', ['--version'])

        podmanCheck.on('close', (innerCode) => {
          resolve(innerCode === 0)
        })

        podmanCheck.on('error', () => {
          log.info('Podman command not found')
          resolve(false)
        })
      }
    })

    checkProcess.on('error', () => {
      log.info('Error checking for podman path')
      // Try direct command as fallback
      const podmanCheck = spawn('podman', ['--version'])

      podmanCheck.on('close', (innerCode) => {
        resolve(innerCode === 0)
      })

      podmanCheck.on('error', () => {
        log.info('Podman command not found')
        resolve(false)
      })
    })
  })
}

/**
 * Checks if a Podman machine already exists
 */
export async function machineExists(): Promise<boolean> {
  return new Promise((resolve) => {
    const checkProcess = spawn('podman', ['machine', 'list'])
    let output = ''

    checkProcess.stdout.on('data', (data) => {
      output += data.toString().trim()
    })

    checkProcess.on('close', (code) => {
      if (code === 0) {
        // Check if output contains a machine that's not just a header row
        const lines = output.split('\n').filter((line) => line.trim().length > 0)
        // If we have more than just the header row, a machine exists
        resolve(lines.length > 1)
      } else {
        resolve(false)
      }
    })

    checkProcess.on('error', () => {
      resolve(false)
    })
  })
}

/**
 * Initializes a Podman machine if one doesn't exist
 */
export async function initMachine(): Promise<boolean> {
  const exists = await machineExists()

  if (exists) {
    log.info('Podman machine already exists')
    return true
  }

  log.info('Initializing Podman machine')

  return new Promise((resolve) => {
    const initProcess = spawn('podman', ['machine', 'init'])

    initProcess.stdout?.on('data', (data) => {
      log.info(`Machine init stdout: ${data.toString().trim()}`)
    })

    initProcess.stderr?.on('data', (data) => {
      log.error(`Machine init stderr: ${data.toString().trim()}`)
    })

    initProcess.on('close', (code) => {
      const success = code === 0
      log.info(
        `Podman machine initialization ${success ? 'succeeded' : 'failed'} with code ${code}`
      )
      resolve(success)
    })

    initProcess.on('error', (err) => {
      log.error(`Podman machine init error: ${err.message}`)
      resolve(false)
    })
  })
}

/**
 * Installs Podman if not already installed
 */
export async function installPodman(): Promise<boolean> {
  const isInstalled = await isPodmanInstalled()

  if (isInstalled) {
    log.info('Podman is already installed')
    // Check if we need to initialize a machine
    const machineInit = await initMachine()
    return machineInit
  }

  log.info('Attempting to install Podman')

  try {
    const installerPath = getPodmanInstallerPath()

    log.info(`Full installer path: ${installerPath}`)
    log.info(`Checking if installer exists: ${fs.existsSync(installerPath)}`)

    // Log file stats if the file exists
    if (fs.existsSync(installerPath)) {
      try {
        const stats = fs.statSync(installerPath)
        log.info(`Installer file stats: size=${stats.size}, permissions=${stats.mode.toString(8)}`)
      } catch (err) {
        log.error('Error getting file stats:', err)
      }
    }

    if (!fs.existsSync(installerPath)) {
      log.error(`Podman installer not found at: ${installerPath}`)
      // Show error dialog
      if (process.platform === 'darwin') {
        log.info('Attempting to download Podman installer directly...')
        // For macOS, try to download directly
        const downloadProcess = spawn('curl', [
          '-L',
          'https://github.com/containers/podman/releases/download/v5.4.2/podman-installer-5.4.2-darwin-arm64.pkg',
          '-o',
          installerPath
        ])

        await new Promise((resolve) => {
          downloadProcess.on('close', (code) => {
            log.info(`Download process exited with code ${code}`)
            resolve(null)
          })
        })

        if (fs.existsSync(installerPath)) {
          log.info('Successfully downloaded Podman installer')
        } else {
          return false
        }
      } else {
        return false
      }
    }

    log.info(`Installing Podman from: ${installerPath}`)

    // Platform-specific installation
    let installProcess

    if (process.platform === 'darwin') {
      // macOS - Direct execution of pkg installer without sudo
      // The installer should prompt for admin privileges if needed
      installProcess = spawn('open', [installerPath])

      log.info('Launched macOS installer. Please complete the installation when prompted.')
      log.info('If the installer does not appear, please install manually with:')
      log.info(`open "${installerPath}"`)

      return new Promise((resolve) => {
        installProcess.on('close', (code) => {
          log.info(`Installer launcher exited with code ${code}`)

          // Wait longer to give time for manual installation
          log.info('Waiting for installation to complete (60 seconds)...')
          setTimeout(async () => {
            const installed = await isPodmanInstalled()
            log.info(`After installation check: Podman installed = ${installed}`)

            if (installed) {
              // If installed, initialize the machine
              const machineInit = await initMachine()
              resolve(machineInit)
            } else {
              log.info(
                'Podman not detected after waiting. If you completed the installation, you may need to:'
              )
              log.info('1. Restart your terminal/command prompt')
              log.info('2. Restart the application')
              log.info('3. Verify Podman is in your PATH')
              resolve(false)
            }
          }, 60000) // Check after 60 seconds
        })

        installProcess.on('error', (err) => {
          log.error(`Error launching installer: ${err.message}`)
          resolve(false)
        })
      })
    } else if (process.platform === 'linux') {
      // Linux - assuming .rpm or .deb package
      const isRpm = installerPath.endsWith('.rpm')

      if (isRpm) {
        installProcess = spawn('sudo', ['rpm', '-i', installerPath])
      } else {
        installProcess = spawn('sudo', ['dpkg', '-i', installerPath])
      }
    } else if (process.platform === 'win32') {
      // Windows - assuming .msi installer
      installProcess = spawn('msiexec', ['/i', installerPath, '/quiet'])
    } else {
      log.error(`Unsupported platform: ${process.platform}`)
      return false
    }

    return new Promise((resolve) => {
      installProcess.stdout?.on('data', (data) => {
        log.info(`Podman installer stdout: ${data.toString().trim()}`)
      })

      installProcess.stderr?.on('data', (data) => {
        log.error(`Podman installer stderr: ${data.toString().trim()}`)
      })

      installProcess.on('close', async (code) => {
        const success = code === 0
        log.info(
          `Podman installation ${success ? 'completed successfully' : 'failed'} with code ${code}`
        )

        if (success) {
          // If installation succeeded, initialize the machine
          const machineInit = await initMachine()
          resolve(machineInit)
        } else {
          resolve(false)
        }
      })

      installProcess.on('error', (err) => {
        log.error(`Podman installation error: ${err.message}`)
        resolve(false)
      })
    })
  } catch (error) {
    log.error('Error installing Podman:', error)
    return false
  }
}

/**
 * Gets the path to the Podman installer package
 */
function getPodmanInstallerPath(): string {
  const resourcesPath = !IS_PRODUCTION
    ? join(__dirname, '..', '..', 'resources') // Path in development
    : join(process.resourcesPath, 'resources') // Path in production

  // First, check for the universal installer
  const universalInstallerPath = join(resourcesPath, 'podman-installer-macos-universal.pkg')
  if (fs.existsSync(universalInstallerPath)) {
    log.info(`Found universal Podman installer: ${universalInstallerPath}`)
    return universalInstallerPath
  }

  // Look for any podman installer files in the resources directory
  const files = fs.readdirSync(resourcesPath)

  // Platform-specific patterns to match installer files
  let filePattern: RegExp
  if (process.platform === 'darwin') {
    filePattern =
      process.arch === 'arm64'
        ? /podman.*darwin.*arm64.*\.pkg$|podman.*macos.*arm64.*\.pkg$/i
        : /podman.*darwin.*amd64.*\.pkg$|podman.*macos.*amd64.*\.pkg$/i
  } else if (process.platform === 'linux') {
    filePattern = /podman.*linux.*\.rpm$|podman.*linux.*\.deb$/i
  } else if (process.platform === 'win32') {
    filePattern = /podman.*windows.*\.exe$|podman.*windows.*\.msi$/i
  } else {
    throw new Error(`Unsupported platform: ${process.platform}`)
  }

  // Find matching installer file
  const matchedFile = files.find((file) => filePattern.test(file))

  if (matchedFile) {
    log.info(`Found Podman installer: ${matchedFile}`)
    return join(resourcesPath, matchedFile)
  }

  // Fallback to original naming if no match found
  let installerName

  if (process.platform === 'darwin') {
    if (process.arch === 'arm64') {
      installerName = 'podman-installer-macos-arm64.pkg'
    } else {
      installerName = 'podman-installer-macos-amd64.pkg'
    }
  } else if (process.platform === 'linux') {
    installerName = 'podman-installer-linux-amd64.rpm'
  } else if (process.platform === 'win32') {
    installerName = 'podman-installer-windows-amd64.msi'
  } else {
    throw new Error(`Unsupported platform: ${process.platform}`)
  }

  return join(resourcesPath, installerName)
}

/**
 * Starts the Podman service
 */
export async function startPodman(): Promise<boolean> {
  if (podmanProcess) {
    log.info('Podman is already running')
    return true
  }

  try {
    const isInstalled = await isPodmanInstalled()

    if (!isInstalled) {
      log.error('Cannot start Podman: Not installed')
      return false
    }

    log.info('Starting Podman service')

    if (process.platform === 'darwin' || process.platform === 'linux') {
      // On macOS and Linux, start the podman machine
      podmanProcess = spawn('podman', ['machine', 'start'])
    } else if (process.platform === 'win32') {
      // On Windows, start the podman machine
      podmanProcess = spawn('podman', ['machine', 'start'])
    } else {
      log.error(`Unsupported platform: ${process.platform}`)
      return false
    }

    podmanProcess.stdout?.on('data', (data) => {
      log.info(`Podman stdout: ${data.toString().trim()}`)
    })

    podmanProcess.stderr?.on('data', (data) => {
      log.error(`Podman stderr: ${data.toString().trim()}`)
    })

    podmanProcess.on('close', (code) => {
      log.info(`Podman process exited with code ${code}`)
      podmanProcess = null
    })

    podmanProcess.on('error', (err) => {
      log.error(`Podman start error: ${err.message}`)
      podmanProcess = null
      return false
    })

    return true
  } catch (error) {
    log.error('Error starting Podman:', error)
    return false
  }
}

/**
 * Stops the Podman service
 */
export function stopPodman(): Promise<boolean> {
  return new Promise((resolve) => {
    if (!podmanProcess) {
      log.info('Podman is not running')
      resolve(true)
      return
    }

    log.info('Stopping Podman service')

    // First try to stop the process gracefully
    podmanProcess.kill()
    podmanProcess = null

    // Then ensure the machine is also stopped
    const stopProcess = spawn('podman', ['machine', 'stop'])

    stopProcess.stdout?.on('data', (data) => {
      log.info(`Podman stop stdout: ${data.toString().trim()}`)
    })

    stopProcess.stderr?.on('data', (data) => {
      log.error(`Podman stop stderr: ${data.toString().trim()}`)
    })

    stopProcess.on('close', (code) => {
      log.info(`Podman machine stop exited with code ${code}`)
      resolve(code === 0)
    })

    stopProcess.on('error', (err) => {
      log.error(`Error stopping Podman machine: ${err.message}`)
      resolve(false)
    })
  })
}
