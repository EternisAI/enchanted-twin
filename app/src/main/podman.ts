import { ChildProcess, spawn, execSync } from 'child_process'
import log from 'electron-log/main'
import { join } from 'path'
import * as fs from 'fs'
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
    const initProcess = spawn('podman', ['machine', 'init', '--now'])
    const downloadRegex = /Copying blob .+? \[(.+?)\] (.+?) \/ (.+?) \| (.+?) MiB\/s/
    let currentProgress = 0

    initProcess.stdout?.on('data', (data) => {
      const output = data.toString().trim()
      log.info(`Machine init stdout: ${output}`)
    })

    initProcess.stderr?.on('data', (data) => {
      const output = data.toString().trim()
      log.info(`Machine init stderr: ${output}`)

      // Try to parse progress information
      if (output.includes('Copying blob')) {
        const match = output.match(downloadRegex)
        if (match) {
          const [, progressBar, current, total, speed] = match
          log.info(`Download progress: ${progressBar} | ${current} of ${total} at ${speed}MB/s`)

          // Extract percentage from progress bar if possible
          if (progressBar.includes('%')) {
            try {
              const percentage = progressBar.match(/(\d+)%/)?.[1]
              if (percentage) {
                currentProgress = parseInt(percentage, 10)
                log.info(`Download percentage: ${currentProgress}%`)

                // Notify the renderer process of the progress if needed
                // mainWindow?.webContents.send('podman-download-progress', currentProgress)
              }
            } catch {
              // Ignore parsing errors
            }
          }
        }
      }
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

/**
 * Pulls a Docker image using Podman
 * @param imageUrl The full image URL to pull (e.g., 'docker.io/library/alpine:latest')
 * @param options Additional options like retry, authenticated pulls, etc.
 * @returns Promise that resolves to true if pull succeeded, false otherwise
 */
export async function pullImage(
  imageUrl: string,
  options: {
    retry?: number
    retryDelay?: number
    timeout?: number
  } = {}
): Promise<boolean> {
  const { retry = 3, retryDelay = 2000, timeout = 300000 } = options // Default 5 minute timeout

  log.info(
    `Pulling image: ${imageUrl} (retries: ${retry}, delay: ${retryDelay}ms, timeout: ${timeout}ms)`
  )

  // Check if the machine is running
  const machineRunning = await isMachineRunning()
  if (!machineRunning) {
    log.error('Cannot pull image: Podman machine is not running')
    return false
  }

  const pullImageWithRetry = async (attemptsLeft: number): Promise<boolean> => {
    try {
      const args = ['pull']

      // Only use retry flags if they're applicable to this version of Podman
      if (retry > 0) {
        args.push('--retry', retry.toString())
      }

      if (retryDelay > 0) {
        args.push('--retry-delay', `${Math.floor(retryDelay / 1000)}s`)
      }

      args.push(imageUrl)

      log.info(`Running podman command: podman ${args.join(' ')}`)

      return new Promise((resolve) => {
        const pullProcess = spawn('podman', args)
        let error = ''
        let lastOutputTime = Date.now()
        let timeoutId: NodeJS.Timeout | null = null

        const checkProgress = () => {
          const currentTime = Date.now()
          if (currentTime - lastOutputTime > timeout) {
            log.error(`Pull operation timed out after ${timeout}ms of inactivity`)
            if (pullProcess && !pullProcess.killed) {
              pullProcess.kill()
            }
            resolve(false)
          }
        }

        // Set up interval to check for timeout
        const intervalId = setInterval(checkProgress, 10000) // Check every 10 seconds

        pullProcess.stdout?.on('data', (data) => {
          const output = data.toString().trim()
          log.info(`Pull stdout: ${output}`)
          lastOutputTime = Date.now() // Reset timeout counter on output
        })

        pullProcess.stderr?.on('data', (data) => {
          const output = data.toString().trim()
          error += output
          log.info(`Pull stderr: ${output}`) // Log as info to see progress, not just errors
          lastOutputTime = Date.now() // Reset timeout counter on output
        })

        pullProcess.on('close', (code) => {
          clearInterval(intervalId)
          if (timeoutId) clearTimeout(timeoutId)

          const success = code === 0
          log.info(`Image pull ${success ? 'succeeded' : 'failed'} with code ${code}`)

          if (
            !success &&
            attemptsLeft > 1 &&
            (error.includes('connection refused') || error.includes('timeout'))
          ) {
            log.info(`Network issue detected. Retrying in ${retryDelay}ms...`)
            setTimeout(() => {
              resolve(pullImageWithRetry(attemptsLeft - 1))
            }, retryDelay)
          } else {
            resolve(success)
          }
        })

        pullProcess.on('error', (err) => {
          clearInterval(intervalId)
          if (timeoutId) clearTimeout(timeoutId)

          log.error(`Pull error: ${err.message}`)

          if (attemptsLeft > 1) {
            log.info(`Error pulling image. Retrying in ${retryDelay}ms...`)
            setTimeout(() => {
              resolve(pullImageWithRetry(attemptsLeft - 1))
            }, retryDelay)
          } else {
            resolve(false)
          }
        })

        // Set overall timeout
        timeoutId = setTimeout(() => {
          log.error(`Pull operation exceeded maximum time of ${timeout}ms`)
          clearInterval(intervalId)
          if (pullProcess && !pullProcess.killed) {
            pullProcess.kill()
          }
          resolve(false)
        }, timeout)
      })
    } catch (error) {
      log.error('Error executing podman pull:', error)
      return false
    }
  }

  // Start with the maximum number of retries
  return pullImageWithRetry(retry)
}

/**
 * Checks if a Podman machine is running
 */
export async function isMachineRunning(): Promise<boolean> {
  return new Promise((resolve) => {
    const checkProcess = spawn('podman', ['machine', 'list', '--format', '{{.Running}}'])
    let output = ''

    checkProcess.stdout.on('data', (data) => {
      output += data.toString().trim()
    })

    checkProcess.on('close', (code) => {
      if (code === 0) {
        // If output contains "true", at least one machine is running
        resolve(output.includes('true'))
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
 * Runs a diagnostic check on Podman
 * Returns information about the Podman environment
 */
export async function runPodmanDiagnostics(): Promise<{
  success: boolean
  info: Record<string, string>
  existingImages: string[]
}> {
  const info: Record<string, string> = {}
  const existingImages: string[] = []
  let success = false

  try {
    // Check Podman version
    try {
      const version = execSync('podman --version').toString().trim()
      info.version = version
      log.info(`Podman version: ${version}`)
    } catch (error) {
      info.version = 'Error getting version'
      log.error('Error getting Podman version', error)
    }

    // Check machine status
    try {
      const machineList = execSync('podman machine list --format json').toString().trim()
      info.machineList = machineList
      log.info(`Podman machine list: ${machineList}`)
    } catch (error) {
      info.machineList = 'Error getting machine list'
      log.error('Error getting Podman machine list', error)
    }

    // Check if Podman can list images
    try {
      const images = execSync('podman image ls --format json').toString().trim()
      info.imageList = images
      log.info(`Podman image list: ${images}`)

      try {
        // Parse the JSON output if available
        const parsedImages = JSON.parse(images)
        if (Array.isArray(parsedImages)) {
          for (const img of parsedImages) {
            if (img.Names && Array.isArray(img.Names)) {
              existingImages.push(...img.Names)
            } else if (img.Repository) {
              const tagName = img.Tag ? `${img.Repository}:${img.Tag}` : img.Repository
              existingImages.push(tagName)
            }
          }
        }
      } catch (jsonError) {
        log.warn('Error parsing image list JSON', jsonError)
      }
    } catch (error) {
      info.imageList = 'Error listing images'
      log.error('Error listing Podman images', error)
    }

    // Check network connectivity (can we ping a public DNS?)
    try {
      execSync('ping -c 1 8.8.8.8')
      info.networkConnectivity = 'Network is reachable'
      log.info('Network connectivity check passed')
    } catch (error) {
      info.networkConnectivity = 'Network unreachable'
      log.error('Network connectivity check failed', error)
    }

    // Check if we can access registry
    try {
      const registryCheck = execSync('podman search alpine --limit 1').toString().trim()
      info.registryAccess = 'Registry accessible'
      log.info(`Registry access check: ${registryCheck}`)
      success = true // If we can search the registry, basic connectivity is working
    } catch (error) {
      info.registryAccess = 'Registry unreachable'
      log.error('Registry access check failed', error)
    }

    return { success, info, existingImages }
  } catch (error) {
    log.error('Error running Podman diagnostics', error)
    return { success: false, info, existingImages }
  }
}

/**
 * Pulls an image using execSync for more direct control
 */
export function pullImageSync(imageUrl: string): boolean {
  try {
    log.info(`Pulling image synchronously: ${imageUrl}`)
    const result = execSync(`podman pull ${imageUrl}`, { timeout: 60000 }).toString().trim()
    log.info(`Pull result: ${result}`)
    return true
  } catch (error) {
    log.error(
      `Error pulling image synchronously: ${error instanceof Error ? error.message : String(error)}`
    )
    return false
  }
}
