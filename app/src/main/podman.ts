import { ChildProcess, spawn, execSync } from 'child_process'
import log from 'electron-log/main'
import { join } from 'path'
import * as fs from 'fs'
import { is } from '@electron-toolkit/utils'
import * as https from 'https'

let podmanProcess: ChildProcess | null = null
const IS_PRODUCTION = process.env.IS_PROD_BUILD === 'true' || !is.dev

// Early in your main process (e.g. before any spawn calls)
if (process.platform === 'darwin') {
  process.env.PATH = ['/opt/podman/bin', process.env.PATH!].join(':')
}

export async function isPodmanInstalled(): Promise<boolean> {
  return new Promise((resolve) => {
    log.info('Checking for Podman in standard locations...')

    const podmanPaths = [
      '/usr/local/bin/podman',
      '/usr/bin/podman',
      '/opt/podman/bin/podman',
      '/opt/homebrew/bin/podman'
    ]

    for (const path of podmanPaths) {
      if (fs.existsSync(path)) {
        log.info(`Found Podman at ${path}`)
        resolve(true)
        return
      }
    }

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

export async function machineExists(): Promise<boolean> {
  return new Promise((resolve) => {
    const checkProcess = spawn('podman', ['machine', 'list'])
    let output = ''

    checkProcess.stdout.on('data', (data) => {
      output += data.toString().trim()
    })

    checkProcess.on('close', (code) => {
      if (code === 0) {
        const lines = output.split('\n').filter((line) => line.trim().length > 0)
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

      if (output.includes('Copying blob')) {
        const match = output.match(downloadRegex)
        if (match) {
          const [, progressBar, current, total, speed] = match
          log.info(`Download progress: ${progressBar} | ${current} of ${total} at ${speed}MB/s`)

          if (progressBar.includes('%')) {
            try {
              const percentage = progressBar.match(/(\d+)%/)?.[1]
              if (percentage) {
                currentProgress = parseInt(percentage, 10)
                log.info(`Download percentage: ${currentProgress}%`)
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

    const machineInit = await initMachine()
    return machineInit
  }

  log.info('Attempting to install Podman')

  try {
    const installerPath = getPodmanInstallerPath()

    log.info(`Full installer path: ${installerPath}`)
    log.info(`Checking if installer exists: ${fs.existsSync(installerPath)}`)

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
      if (process.platform === 'darwin') {
        log.info('Attempting to download the latest Podman installer from GitHub releases...')

        const latest = await getLatestPodmanMacOSDownloadURL()
        if (!latest) {
          log.error('Could not determine latest Podman release for macOS')
          return false
        }

        const downloaded = await downloadWithCurl(latest.url, installerPath)
        if (!downloaded) {
          log.error('Failed to download the Podman installer')
          return false
        }
        log.info(`Successfully downloaded Podman ${latest.version} installer`)
      } else {
        return false
      }
    }

    log.info(`Installing Podman from: ${installerPath}`)

    let installProcess

    if (process.platform === 'darwin') {
      // Perform a silent installation on macOS using the command-line `installer` wrapped
      // in AppleScript. Using `osascript` with the `with administrator privileges` flag
      // triggers the native macOS authorisation dialog, which supports Touch ID and
      // does not require users to type their password in the terminal.

      // Craft an AppleScript command that quotes the pkg path safely.
      const escapedPath = installerPath.replace(/'/g, "'\\''")
      const appleScript = `do shell script "installer -pkg '${escapedPath}' -target /" with administrator privileges`
      installProcess = spawn('osascript', ['-e', appleScript])

      log.info(
        'Running macOS pkg through osascript; users will be prompted via the standard authorisation dialog (Touch ID supported).'
      )
    } else if (process.platform === 'linux') {
      const isRpm = installerPath.endsWith('.rpm')

      if (isRpm) {
        installProcess = spawn('sudo', ['rpm', '-i', installerPath])
      } else {
        installProcess = spawn('sudo', ['dpkg', '-i', installerPath])
      }
    } else if (process.platform === 'win32') {
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
          // Double-check that the CLI is now available on the PATH. On slower
          // systems the package post-install scripts can take a short while to
          // finish even after the installer exits.
          const installed = await waitForPodmanInstalled()

          if (!installed) {
            log.error('Podman was not detected on the system after the installer finished.')
            resolve(false)
            return
          }

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

  const universalInstallerPath = join(resourcesPath, 'podman-installer-macos-universal.pkg')
  if (fs.existsSync(universalInstallerPath)) {
    log.info(`Found universal Podman installer: ${universalInstallerPath}`)
    return universalInstallerPath
  }

  const files = fs.readdirSync(resourcesPath)

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

  const matchedFile = files.find((file) => filePattern.test(file))

  if (matchedFile) {
    log.info(`Found Podman installer: ${matchedFile}`)
    return join(resourcesPath, matchedFile)
  }

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
 * Sets memory allocation for a Podman machine
 * @param memory Memory in MB to allocate
 * @param machineName Name of the Podman machine (defaults to podman-machine-default)
 * @returns Promise that resolves to true if setting memory succeeded
 */
export async function setPodmanMachineMemory(
  memory: number = 4096,
  machineName: string = 'podman-machine-default'
): Promise<boolean> {
  log.info(`Setting Podman machine ${machineName} memory to ${memory}MB`)

  return new Promise((resolve) => {
    const setMemoryProcess = spawn('podman', [
      'machine',
      'set',
      '--memory',
      memory.toString(),
      machineName
    ])

    setMemoryProcess.stdout?.on('data', (data) => {
      log.info(`Podman memory set stdout: ${data.toString().trim()}`)
    })

    setMemoryProcess.stderr?.on('data', (data) => {
      log.error(`Podman memory set stderr: ${data.toString().trim()}`)
    })

    setMemoryProcess.on('close', (code) => {
      const success = code === 0
      log.info(`Podman machine memory set ${success ? 'succeeded' : 'failed'} with code ${code}`)
      resolve(success)
    })

    setMemoryProcess.on('error', (err) => {
      log.error(`Error setting Podman machine memory: ${err.message}`)
      resolve(false)
    })
  })
}

/**
 * Starts the Podman service
 */
export async function startPodman(
  options: { setMemory?: boolean; memoryMB?: number } = {}
): Promise<boolean> {
  const { setMemory = false, memoryMB = 4096 } = options

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

    // Set memory if requested
    if (setMemory) {
      const memorySet = await setPodmanMachineMemory(memoryMB)
      if (!memorySet) {
        log.warn(
          `Failed to set Podman machine memory to ${memoryMB}MB, continuing with start anyway`
        )
      }
    }

    log.info('Starting Podman service')

    if (process.platform === 'darwin' || process.platform === 'linux') {
      podmanProcess = spawn('podman', ['machine', 'start'])
    } else if (process.platform === 'win32') {
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

    podmanProcess.kill()
    podmanProcess = null

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
  const { retry = 3, retryDelay = 2000, timeout = 300000 } = options

  log.info(
    `Pulling image: ${imageUrl} (retries: ${retry}, delay: ${retryDelay}ms, timeout: ${timeout}ms)`
  )

  const machineRunning = await isMachineRunning()
  if (!machineRunning) {
    log.error('Cannot pull image: Podman machine is not running')
    return false
  }

  const pullImageWithRetry = async (attemptsLeft: number): Promise<boolean> => {
    try {
      const args = ['pull']

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

        const intervalId = setInterval(checkProgress, 10000) // Check every 10 seconds

        pullProcess.stdout?.on('data', (data) => {
          const output = data.toString().trim()
          log.info(`Pull stdout: ${output}`)
          lastOutputTime = Date.now() // Reset timeout counter on output
        })

        pullProcess.stderr?.on('data', (data) => {
          const output = data.toString().trim()
          error += output
          log.info(`Pull stderr: ${output}`)
          lastOutputTime = Date.now()
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
    try {
      const version = execSync('podman --version').toString().trim()
      info.version = version
      log.info(`Podman version: ${version}`)
    } catch (error) {
      info.version = 'Error getting version'
      log.error('Error getting Podman version', error)
    }

    try {
      const machineList = execSync('podman machine list --format json').toString().trim()
      info.machineList = machineList
      log.info(`Podman machine list: ${machineList}`)
    } catch (error) {
      info.machineList = 'Error getting machine list'
      log.error('Error getting Podman machine list', error)
    }

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

    try {
      execSync('ping -c 1 8.8.8.8')
      info.networkConnectivity = 'Network is reachable'
      log.info('Network connectivity check passed')
    } catch (error) {
      info.networkConnectivity = 'Network unreachable'
      log.error('Network connectivity check failed', error)
    }

    try {
      const registryCheck = execSync('podman search alpine --limit 1').toString().trim()
      info.registryAccess = 'Registry accessible'
      log.info(`Registry access check: ${registryCheck}`)
      success = true
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
export function pullImageSync(imageUrl: string, timeoutMs: number = 600_000): boolean {
  try {
    log.info(`Pulling image synchronously: ${imageUrl} (timeout: ${timeoutMs}ms)`)
    const resultBuffer = execSync(`podman pull ${imageUrl}`, { timeout: timeoutMs })
    const result = resultBuffer.toString().trim()
    log.info(`Pull result: ${result}`)
    return true
  } catch (error) {
    log.error(
      `Error pulling image synchronously: ${error instanceof Error ? error.message : String(error)}`
    )
    return false
  }
}

// Helper: fetch latest Podman release info for macOS
async function getLatestPodmanMacOSDownloadURL(): Promise<{ url: string; version: string } | null> {
  return new Promise((resolve) => {
    const apiUrl = 'https://api.github.com/repos/containers/podman/releases/latest'

    const request = https.get(
      apiUrl,
      {
        headers: {
          'User-Agent': 'enchanted-twin-podman-installer'
        }
      },
      (res) => {
        let data = ''
        res.on('data', (chunk) => {
          data += chunk
        })
        res.on('end', () => {
          try {
            const json = JSON.parse(data)
            const assets: Array<{ name: string; browser_download_url: string }> = json.assets || []
            const arch = process.arch === 'arm64' ? 'arm64' : 'amd64'
            const pattern = new RegExp(
              `podman-installer.*(darwin|macos).*${arch}.*\\.pkg$|podman.*(darwin|macos).*universal.*\\.pkg$`,
              'i'
            )
            const match = assets.find((a) => pattern.test(a.name))
            if (match) {
              resolve({ url: match.browser_download_url, version: json.tag_name })
              return
            }
          } catch (err) {
            log.error('Error parsing GitHub release JSON', err)
          }
          resolve(null)
        })
      }
    )

    request.on('error', (err) => {
      log.error('Failed to query GitHub releases', err)
      resolve(null)
    })
  })
}

// Helper: download a file with curl (shows progress automatically)
async function downloadWithCurl(url: string, destination: string): Promise<boolean> {
  return new Promise((resolve) => {
    log.info(`Downloading Podman installer from ${url} to ${destination}`)
    const download = spawn('curl', ['-L', url, '-o', destination])

    download.stderr?.on('data', (data) => {
      log.info(`curl: ${data.toString().trim()}`)
    })

    download.on('close', (code) => {
      const success = code === 0 && fs.existsSync(destination)
      if (!success) {
        log.error(`curl exited with code ${code}. Download failed.`)
      }
      resolve(success)
    })

    download.on('error', (err) => {
      log.error('Error spawning curl', err)
      resolve(false)
    })
  })
}

// Helper: poll until Podman is installed or timeout expires
async function waitForPodmanInstalled(maxWaitMs = 600_000, intervalMs = 15_000): Promise<boolean> {
  const start = Date.now()
  while (Date.now() - start < maxWaitMs) {
    if (await isPodmanInstalled()) {
      return true
    }
    await new Promise((r) => setTimeout(r, intervalMs))
  }
  return false
}
