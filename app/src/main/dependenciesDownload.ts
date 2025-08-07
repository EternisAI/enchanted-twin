import { app } from 'electron'
import path from 'path'
import fs from 'fs'
import axios from 'axios'
import extract from 'extract-zip'
import * as tar from 'tar'
import { spawn } from 'child_process'
import log from 'electron-log'
import lzma from 'lzma-native'

// Helper type for dependency configuration (matching the readonly generated structure)
type DependencyConfig = {
  readonly backend_download_enabled: boolean
  readonly url: string | Record<string, string>
  readonly name: string
  readonly display_name?: string
  readonly description?: string
  readonly category: string
  readonly dir: string
  readonly type: string
  readonly platform_url_key?: boolean
  readonly validation_condition?: 'has_both_models' | 'platform_specific_binary'
  readonly validation_files?: readonly string[] | Record<string, readonly string[]>
  readonly install_script?: string
  readonly files?: {
    readonly binaries?: readonly string[]
    readonly libraries?: readonly string[]
    readonly dataFiles?: readonly string[]
  }
  readonly platform_config?: Record<string, {
    readonly type: string
    readonly files?: {
      readonly binaries?: readonly string[]
      readonly libraries?: readonly string[]
      readonly dataFiles?: readonly string[]
    }
  }>
  readonly post_download?: {
    readonly chmod?: {
      readonly files: readonly string[]
      readonly mode: string
    }
    readonly cleanup?: readonly string[]
    readonly copy_from_global?: boolean
  }
}
import { windowManager } from './windows'
import { DependencyName } from './types/dependencies'
import { EMBEDDED_RUNTIME_DEPS_CONFIG } from './embeddedDepsConfig'

const DEPENDENCIES_DIR = path.join(app.getPath('appData'), 'enchanted')

// Use embedded config that was generated at build time
const RUNTIME_DEPS_CONFIG = EMBEDDED_RUNTIME_DEPS_CONFIG

// Helper function to get platform-specific key
function getPlatformKey(): string {
  if (process.platform === 'darwin' && process.arch === 'arm64') {
    return 'darwin-arm64'
  } else if (process.platform === 'linux' && process.arch === 'x64') {
    return 'linux-x64'
  } else if (process.platform === 'win32') {
    return 'win32'
  }
  return 'linux-x64' // fallback default
}

// Generic dependency installer functions
function createGenericInstaller(depName: DependencyName) {
  const config = RUNTIME_DEPS_CONFIG?.dependencies?.[depName] as DependencyConfig | undefined
  if (!config) {
    console.warn(`No config found for dependency: ${String(depName)}`)
    return null
  }

  return {
    install: async function () {
      const depDir = config.dir.replace('{DEPENDENCIES_DIR}', DEPENDENCIES_DIR)

      switch (config.type) {
        case 'individual_files': {
          const baseUrl = config.url
          const allFiles = [
            ...(config.files?.binaries || []),
            ...(config.files?.libraries || []),
            ...(config.files?.dataFiles || [])
          ]
          let downloadedFiles = 0
          const totalFiles = allFiles.length
          const failedDownloads: string[] = []

          const downloadPromises = allFiles.map(async (filePath) => {
            const fileUrl = `${baseUrl}/${filePath}`
            const destPath = path.join(depDir, filePath)
            const destDir = path.dirname(destPath)

            if (!fs.existsSync(destDir)) {
              fs.mkdirSync(destDir, { recursive: true })
            }

            try {
              await downloadSingleFile(fileUrl, destPath)
              downloadedFiles++

              const pct = Math.round((downloadedFiles / totalFiles) * 100)
              windowManager.mainWindow?.webContents.send('models:progress', {
                modelName: config.name,
                pct,
                totalBytes: totalFiles,
                downloadedBytes: downloadedFiles
              })
            } catch (error) {
              console.error(`Failed to download ${filePath}:`, error)
              failedDownloads.push(filePath)
            }
          })

          await Promise.all(downloadPromises)

          // Set executable permissions based on post_download config
          if (config.post_download?.chmod && process.platform !== 'win32') {
            const mode = config.post_download.chmod.mode
            const files = config.post_download.chmod.files || []
            for (const filePath of files) {
              const fullPath = path.join(depDir, filePath)
              if (fs.existsSync(fullPath)) {
                fs.chmodSync(fullPath, parseInt(mode, 8))
              }
            }
          }
          break
        }

        case 'platform_mixed': {
          const platformKey = getPlatformKey()
          const platformConfig = config.platform_config?.[platformKey]
          
          if (!platformConfig) {
            throw new Error(`No platform configuration found for ${platformKey} in dependency ${config.name}`)
          }

          const platformType = platformConfig.type
          let url: string = typeof config.url === 'string' ? config.url : ''
          
          if (typeof config.url === 'object' && config.url) {
            url = config.url[platformKey] || ''
          }

          if (!url) {
            throw new Error(`No URL found for platform ${platformKey} in dependency ${config.name}`)
          }

          if (platformType === 'individual_files') {
            // Handle individual files download for macOS
            const baseUrl = url
            const allFiles = [
              ...(platformConfig.files?.binaries || []),
              ...(platformConfig.files?.libraries || []),
              ...(platformConfig.files?.dataFiles || [])
            ]
            let downloadedFiles = 0
            const totalFiles = allFiles.length

            const downloadPromises = allFiles.map(async (filePath) => {
              const fileUrl = `${baseUrl}/${filePath}`
              const destPath = path.join(depDir, filePath)
              const destDir = path.dirname(destPath)

              if (!fs.existsSync(destDir)) {
                fs.mkdirSync(destDir, { recursive: true })
              }

              try {
                await downloadSingleFile(fileUrl, destPath)
                downloadedFiles++

                const pct = Math.round((downloadedFiles / totalFiles) * 100)
                windowManager.mainWindow?.webContents.send('models:progress', {
                  modelName: config.name,
                  pct,
                  totalBytes: totalFiles,
                  downloadedBytes: downloadedFiles
                })
              } catch (error) {
                console.error(`Failed to download ${filePath}:`, error)
                throw error
              }
            })

            await Promise.all(downloadPromises)
          } else if (platformType === 'tar.gz') {
            // Handle archive download for Linux
            const file = await downloadFile(url, depDir, 'temp.tgz', (pct, total, downloaded) => {
              windowManager.mainWindow?.webContents.send('models:progress', {
                modelName: config.name,
                pct,
                totalBytes: total,
                downloadedBytes: downloaded
              })
            })
            
            try {
              await extractTarGz(file, depDir)
            } catch (error) {
              log.error(`Failed to extract TAR.GZ archive for ${config.name}:`, error)
              windowManager.mainWindow?.webContents.send('models:progress', {
                modelName: config.name,
                pct: 0,
                error: `Failed to extract TAR.GZ archive: ${error instanceof Error ? error.message : 'Unknown error'}`
              })
              throw error
            }
          } else if (platformType === 'tar.xz') {
            // Handle XZ archive download for Linux
            const file = await downloadFile(url, depDir, 'temp.txz', (pct, total, downloaded) => {
              windowManager.mainWindow?.webContents.send('models:progress', {
                modelName: config.name,
                pct,
                totalBytes: total,
                downloadedBytes: downloaded
              })
            })
            
            try {
              await extractTarXz(file, depDir)
            } catch (error) {
              log.error(`Failed to extract TAR.XZ archive for ${config.name}:`, error)
              windowManager.mainWindow?.webContents.send('models:progress', {
                modelName: config.name,
                pct: 0,
                error: `Failed to extract TAR.XZ archive: ${error instanceof Error ? error.message : 'Unknown error'}`
              })
              throw error
            }
          } else {
            throw new Error(`Unsupported platform type '${platformType}' for platform_mixed dependency ${config.name}`)
          }

          // Set executable permissions based on post_download config
          if (config.post_download?.chmod && process.platform !== 'win32') {
            const mode = config.post_download.chmod.mode
            const files = config.post_download.chmod.files || []
            for (const filePath of files) {
              const fullPath = path.join(depDir, filePath)
              if (fs.existsSync(fullPath)) {
                fs.chmodSync(fullPath, parseInt(mode, 8))
              }
            }
          }
          break
        }

        case 'zip': {
          const file = await downloadFile(
            config.url as string,
            depDir,
            'temp.zip',
            (pct, total, downloaded) => {
              windowManager.mainWindow?.webContents.send('models:progress', {
                modelName: config.name,
                pct,
                totalBytes: total,
                downloadedBytes: downloaded
              })
            }
          )
          try {
            await extractZip(file, depDir)
          } catch (error) {
            log.error(`Failed to extract ZIP file for ${config.name}:`, error)
            windowManager.mainWindow?.webContents.send('models:progress', {
              modelName: config.name,
              pct: 0,
              error: `Failed to extract ZIP file: ${error instanceof Error ? error.message : 'Unknown error'}`
            })
            throw error
          }
          break
        }

        case 'tar.gz': {
          let url: string = typeof config.url === 'string' ? config.url : ''
          if (config.platform_url_key && typeof config.url === 'object') {
            const urlObj = config.url as Record<string, string>
            const platformKey = getPlatformKey()
            url = urlObj[platformKey] || ''
          }

          const file = await downloadFile(url, depDir, 'temp.tgz', (pct, total, downloaded) => {
            windowManager.mainWindow?.webContents.send('models:progress', {
              modelName: config.name,
              pct,
              totalBytes: total,
              downloadedBytes: downloaded
            })
          })
          try {
            await extractTarGz(file, depDir)
          } catch (error) {
            log.error(`Failed to extract TAR.GZ file for ${config.name}:`, error)
            windowManager.mainWindow?.webContents.send('models:progress', {
              modelName: config.name,
              pct: 0,
              error: `Failed to extract TAR.GZ file: ${error instanceof Error ? error.message : 'Unknown error'}`
            })
            throw error
          }
          break
        }

        case 'tar.xz': {
          let url: string = typeof config.url === 'string' ? config.url : ''
          if (config.platform_url_key && typeof config.url === 'object') {
            const urlObj = config.url as Record<string, string>
            const platformKey = getPlatformKey()
            url = urlObj[platformKey] || ''
          }

          const file = await downloadFile(url, depDir, 'temp.txz', (pct, total, downloaded) => {
            windowManager.mainWindow?.webContents.send('models:progress', {
              modelName: config.name,
              pct,
              totalBytes: total,
              downloadedBytes: downloaded
            })
          })
          try {
            await extractTarXz(file, depDir)
          } catch (error) {
            log.error(`Failed to extract TAR.XZ file for ${config.name}:`, error)
            windowManager.mainWindow?.webContents.send('models:progress', {
              modelName: config.name,
              pct: 0,
              error: `Failed to extract TAR.XZ file: ${error instanceof Error ? error.message : 'Unknown error'}`
            })
            throw error
          }
          break
        }

        case 'curl_script': {
          if (!config.install_script) {
            throw new Error(`No install_script defined for ${config.name}`)
          }

          // Execute the shell script
          const process = spawn('sh', ['-c', config.install_script], {
            stdio: ['pipe', 'pipe', 'pipe']
          })

          let progress = 0
          const progressInterval = setInterval(() => {
            progress = Math.min(progress + 10, 90) // Increment progress up to 90%
            windowManager.mainWindow?.webContents.send('models:progress', {
              modelName: config.name,
              pct: progress,
              totalBytes: 0,
              downloadedBytes: 0
            })
          }, 1000)

          await new Promise<void>((resolve, reject) => {
            let stdout = ''
            let stderr = ''

            process.stdout.on('data', (data) => {
              stdout += data.toString()
            })

            process.stderr.on('data', (data) => {
              stderr += data.toString()
            })

            process.on('close', (code) => {
              clearInterval(progressInterval)

              if (code === 0) {
                windowManager.mainWindow?.webContents.send('models:progress', {
                  modelName: config.name,
                  pct: 100,
                  totalBytes: 0,
                  downloadedBytes: 0
                })
                resolve(undefined)
              } else {
                const errorMsg = `Script failed with code ${code}: ${stderr || stdout}`
                windowManager.mainWindow?.webContents.send('models:progress', {
                  modelName: config.name,
                  pct: 0,
                  error: errorMsg
                })
                reject(new Error(errorMsg))
              }
            })

            process.on('error', (error) => {
              clearInterval(progressInterval)
              windowManager.mainWindow?.webContents.send('models:progress', {
                modelName: config.name,
                pct: 0,
                error: `Failed to execute script: ${error.message}`
              })
              reject(error)
            })
          })
          break
        }
      }
    },

    isDownloaded: function () {
      const depDir = config.dir.replace('{DEPENDENCIES_DIR}', DEPENDENCIES_DIR)

      // Handle special validation conditions first
      if (config.validation_condition === 'has_both_models') {
        if (process.env.ANONYMIZER_TYPE === 'no-op') {
          return true
        }

        if (!isExtractedDirValid(depDir)) {
          return false
        }

        try {
          const files = fs.readdirSync(depDir)
          const ggufs = files.filter((file) => file.endsWith('.gguf'))
          const has4bModel = ggufs.some(
            (file) =>
              file.toLowerCase().includes('qwen') &&
              (file.toLowerCase().includes('4b') || file.toLowerCase().includes('4-b'))
          )
          const has06bModel = ggufs.some(
            (file) =>
              file.toLowerCase().includes('qwen') &&
              (file.toLowerCase().includes('0.6b') || file.toLowerCase().includes('0.6-b'))
          )
          return has4bModel && has06bModel
        } catch {
          return false
        }
      }

      if (config.validation_condition === 'platform_specific_binary') {
        if (process.env.VITE_DISABLE_VOICE === 'true') {
          return true
        }

        // Get platform-specific validation files
        let platformValidationFiles = config.validation_files
        if (
          typeof platformValidationFiles === 'object' &&
          !Array.isArray(platformValidationFiles)
        ) {
          const platformKey = getPlatformKey()
          platformValidationFiles =
            platformValidationFiles[platformKey] || platformValidationFiles['default'] || []
        }

        if (Array.isArray(platformValidationFiles)) {
          return platformValidationFiles.some((filePath) => {
            const fullPath = path.join(depDir, filePath)
            return fs.existsSync(fullPath)
          })
        }

        return false
      }

      // Handle platform-specific validation files (like ONNX)
      let validationFiles = config.validation_files || []
      if (typeof validationFiles === 'object' && !Array.isArray(validationFiles)) {
        // Platform-specific validation files
        const platformKey = getPlatformKey()
        validationFiles = validationFiles[platformKey] || []
      }

      // Standard validation using validation_files (array)
      if (Array.isArray(validationFiles)) {
        return validationFiles.every((filePath) => {
          const fullPath = path.join(depDir, filePath)
          return fs.existsSync(fullPath)
        })
      }

      return false
    }
  }
}

// Create JSON-driven dependency configs
const DEPENDENCIES_CONFIGS: Record<
  DependencyName,
  {
    url: string
    name: string
    dir: string
    install: () => Promise<void>
    isDownloaded: () => boolean
  }
> = (() => {
  const configs: Record<
    string,
    {
      url: string
      name: string
      dir: string
      install: () => Promise<void>
      isDownloaded: () => boolean
    }
  > = {}
  const dependencyNames = Object.keys(
    RUNTIME_DEPS_CONFIG?.dependencies || {}
  ) as string[] as DependencyName[]

  for (const depName of dependencyNames) {
    const genericInstaller = createGenericInstaller(depName)
    if (genericInstaller) {
      const config = RUNTIME_DEPS_CONFIG?.dependencies?.[depName] as DependencyConfig | undefined
      configs[depName] = {
        url: (() => {
          if (typeof config?.url === 'string') {
            return config.url
          } else if (typeof config?.url === 'object' && config?.url) {
            const platformKey = getPlatformKey()
            return config.url[platformKey] || ''
          }
          return ''
        })(),
        name: config?.name || depName,
        dir:
          config?.dir?.replace('{DEPENDENCIES_DIR}', DEPENDENCIES_DIR) ||
          path.join(DEPENDENCIES_DIR, depName),
        install: genericInstaller.install,
        isDownloaded: genericInstaller.isDownloaded
      }
    } else {
      // Fallback for missing config
      console.warn(`Using fallback config for ${depName}`)
      configs[depName] = {
        url: '',
        name: depName,
        dir: path.join(DEPENDENCIES_DIR, depName),
        install: async () => {
          console.warn(`No installer available for ${depName}`)
        },
        isDownloaded: () => false
      }
    }
  }

  return configs as Record<
    DependencyName,
    {
      url: string
      name: string
      dir: string
      install: () => Promise<void>
      isDownloaded: () => boolean
    }
  >
})()

export function getDependencyPath(dependencyName: DependencyName): string {
  const cfg = DEPENDENCIES_CONFIGS[dependencyName]
  if (!cfg) {
    throw new Error(`Unknown dependency: ${dependencyName}`)
  }
  return cfg.dir
}

export async function downloadDependency(dependencyName: DependencyName) {
  const cfg = DEPENDENCIES_CONFIGS[dependencyName]
  if (!cfg) {
    throw new Error(`Unknown dependency: ${dependencyName}`)
  }
  await cfg.install()

  await new Promise((resolve) => setTimeout(resolve, 1000)) // Small delay
  windowManager.mainWindow?.webContents.send('models:progress', {
    modelName: dependencyName,
    pct: 100,
    totalBytes: 0,
    downloadedBytes: 0
  })

  return { success: true, path: cfg.dir }
}

export function hasDependenciesDownloaded(): Record<DependencyName, boolean> {
  const result: Record<string, boolean> = {}
  const dependencyNames = Object.keys(
    RUNTIME_DEPS_CONFIG?.dependencies || {}
  ) as string[] as DependencyName[]

  for (const depName of dependencyNames) {
    const config = DEPENDENCIES_CONFIGS[depName]
    result[depName] = config ? config.isDownloaded() : false
  }

  return result as Record<DependencyName, boolean>
}

async function downloadSingleFile(url: string, destPath: string): Promise<void> {
  const resp = await axios.get(url, { responseType: 'stream' })
  return new Promise<void>((resolve, reject) => {
    const ws = fs.createWriteStream(destPath)
    resp.data.pipe(ws)
    ws.on('finish', resolve)
    ws.on('error', reject)
  })
}

async function downloadFile(
  url: string,
  destDir: string,
  fileName: string,
  onProgress?: (pct: number, total: number, downloaded: number) => void
): Promise<string> {
  if (!fs.existsSync(destDir)) {
    fs.mkdirSync(destDir, { recursive: true })
  }
  const tmpFile = path.join(destDir, fileName)
  let total = 0
  let downloaded = 0
  const resp = await axios.get(url, { responseType: 'stream' })
  total = Number(resp.headers['content-length'] || 0)
  await new Promise<void>((resolve, reject) => {
    const ws = fs.createWriteStream(tmpFile)
    resp.data.on('data', (chunk: Buffer) => {
      downloaded += chunk.length
      if (onProgress && total > 0) {
        // Cap progress at 99% during download, 100% will be sent after extraction
        const pct = Math.min(99, Math.round((downloaded / total) * 100))
        onProgress(pct, total, downloaded)
      }
    })
    resp.data.pipe(ws)
    ws.on('finish', resolve)
    ws.on('error', reject)
  })
  return tmpFile
}

async function extractZip(file: string, destDir: string) {
  await extract(file, { dir: destDir })
  fs.unlinkSync(file)
}

async function extractTarGz(file: string, destDir: string) {
  await tar.extract({ file, cwd: destDir })
  fs.unlinkSync(file)
}

async function extractTarXz(file: string, destDir: string) {
  return new Promise<void>((resolve, reject) => {
    const readStream = fs.createReadStream(file)
    const decompressStream = lzma.createDecompressor()
    
    const chunks: Buffer[] = []
    
    readStream
      .pipe(decompressStream)
      .on('data', (chunk: Buffer) => {
        chunks.push(chunk)
      })
      .on('end', async () => {
        try {
          const decompressedData = Buffer.concat(chunks)
          const tempTarFile = file.replace(/\.txz$|\.tar\.xz$/, '.tar')
          
          fs.writeFileSync(tempTarFile, decompressedData)
          await tar.extract({ file: tempTarFile, cwd: destDir })
          
          fs.unlinkSync(tempTarFile)
          fs.unlinkSync(file)
          resolve()
        } catch (error) {
          reject(error)
        }
      })
      .on('error', (error) => {
        reject(error)
      })
  })
}

function isExtractedDirValid(dir: string): boolean {
  if (!fs.existsSync(dir)) return false
  try {
    const files = fs.readdirSync(dir)
    if (files.length === 0) return false
    const nonArchiveFiles = files.filter((file) => {
      const ext = path.extname(file).toLowerCase()
      return !ext.match(/\.(zip|tar|tgz|tar\.gz)$/)
    })
    if (nonArchiveFiles.length === 0) return false
    const validFiles = nonArchiveFiles.filter((file) => {
      const filePath = path.join(dir, file)
      const stat = fs.statSync(filePath)
      return stat.isDirectory() || stat.size > 0
    })
    return validFiles.length > 0
  } catch {
    return false
  }
}
