import { app } from 'electron'
import path from 'path'
import fs from 'fs'
import axios from 'axios'
import extract from 'extract-zip'
import * as tar from 'tar'
import { windowManager } from './windows'
import { DependencyName } from './types/dependencies'
import { LiveKitAgentBootstrap } from './livekitAgent'
import { EMBEDDED_RUNTIME_DEPS_CONFIG } from './embeddedDepsConfig'

const DEPENDENCIES_DIR = path.join(app.getPath('appData'), 'enchanted')

// Use embedded config that was generated at build time
const RUNTIME_DEPS_CONFIG = EMBEDDED_RUNTIME_DEPS_CONFIG

// Generic dependency installer functions
function createGenericInstaller(depName: DependencyName) {
  const config = RUNTIME_DEPS_CONFIG?.dependencies?.[depName]
  if (!config) {
    console.warn(`No config found for dependency: ${depName}`)
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

          for (const filePath of allFiles) {
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
            config.url,
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
          await extractZip(file, depDir)
          break
        }

        case 'tar.gz': {
          let url = config.url
          if (config.platform_url_key && typeof url === 'object') {
            if (process.platform === 'darwin' && process.arch === 'arm64') {
              url = url['darwin-arm64']
            } else {
              url = url['linux-x64']
            }
          }

          const file = await downloadFile(
            url as string,
            depDir,
            'temp.tgz',
            (pct, total, downloaded) => {
              windowManager.mainWindow?.webContents.send('models:progress', {
                modelName: config.name,
                pct,
                totalBytes: total,
                downloadedBytes: downloaded
              })
            }
          )
          await extractTarGz(file, depDir)
          break
        }

        case 'curl_script': {
          if (process.env.VITE_DISABLE_VOICE === 'true') {
            return
          }

          const livekitAgent = new LiveKitAgentBootstrap({
            onProgress: (data) => {
              windowManager.mainWindow?.webContents.send('models:progress', {
                modelName: config.name,
                pct: data.progress,
                status: data.status,
                error: data.error,
                totalBytes: 0,
                downloadedBytes: 0
              })
            }
          })
          await livekitAgent.setup()
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
        } catch (error) {
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
          const platformKey = process.platform === 'win32' ? 'win32' : 'default'
          platformValidationFiles = platformValidationFiles[platformKey] || []
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
        const platform = process.platform
        const arch = process.arch
        let platformKey: string

        if (platform === 'darwin' && arch === 'arm64') {
          platformKey = 'darwin-arm64'
        } else {
          platformKey = 'linux-x64'
        }

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
  const configs: Record<string, any> = {}
  const dependencyNames = Object.keys(RUNTIME_DEPS_CONFIG?.dependencies || {}) as DependencyName[]

  for (const depName of dependencyNames) {
    const genericInstaller = createGenericInstaller(depName)
    if (genericInstaller) {
      const config = RUNTIME_DEPS_CONFIG?.dependencies?.[depName]
      configs[depName] = {
        url: typeof config?.url === 'string' ? config.url : '',
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

  return configs
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
  const dependencyNames = Object.keys(RUNTIME_DEPS_CONFIG?.dependencies || {}) as DependencyName[]

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
