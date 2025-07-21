import { app } from 'electron'
import path from 'path'
import fs from 'fs'
import axios from 'axios'
import extract from 'extract-zip'
import * as tar from 'tar'

import { windowManager } from './windows'
import { DependencyName, PostInstallHook } from './types/dependencies'

const DEPENDENCIES_DIR = path.join(app.getPath('appData'), 'enchanted')

function getUvDownloadUrl(): string {
  if (process.platform === 'darwin') {
    if (process.arch === 'arm64') {
      return 'https://github.com/astral-sh/uv/releases/latest/download/uv-aarch64-apple-darwin.tar.gz'
    } else {
      return 'https://github.com/astral-sh/uv/releases/latest/download/uv-x86_64-apple-darwin.tar.gz'
    }
  } else if (process.platform === 'linux') {
    return 'https://github.com/astral-sh/uv/releases/latest/download/uv-x86_64-unknown-linux-gnu.tar.gz'
  } else if (process.platform === 'win32') {
    return 'https://github.com/astral-sh/uv/releases/latest/download/uv-x86_64-pc-windows-msvc.zip'
  }
  throw new Error(`Unsupported platform: ${process.platform} ${process.arch}`)
}

const DEPENDENCIES_CONFIGS: Record<
  DependencyName,
  {
    url: string
    name: string
    dir: string
    needsExtraction: boolean
    postInstallHook?: PostInstallHook
  }
> = {
  embeddings: {
    url: 'https://d3o88a4htgfnky.cloudfront.net/models/jina-embeddings-v2-base-en.zip',
    name: 'embeddings',
    dir: path.join(DEPENDENCIES_DIR, 'models', 'jina-embeddings-v2-base-en'),
    needsExtraction: true
  },
  anonymizer: {
    url: 'https://huggingface.co/eternis/eternis_anonymizer_merge_Qwen3-0.6B_9jul_30k_gguf/resolve/main/qwen3-0.6b-q4_k_m.gguf?download=true',
    name: 'anonymizer',
    dir: path.join(DEPENDENCIES_DIR, 'models', 'anonymizer'),
    needsExtraction: false
  },
  LLAMACCP: {
    //@TODO: Add different versions for linux, windows and intel mac
    url: 'https://github.com/ggml-org/llama.cpp/releases/download/b5916/llama-b5916-bin-macos-arm64.zip',
    name: 'llamacpp',
    dir: path.join(DEPENDENCIES_DIR, 'shared', 'lib', 'llamacpp'),
    needsExtraction: true
  },
  onnx: {
    url:
      process.platform === 'darwin' && process.arch === 'arm64'
        ? 'https://d3o88a4htgfnky.cloudfront.net/assets/onnxruntime-osx-arm64-1.22.0.tgz'
        : 'https://d3o88a4htgfnky.cloudfront.net/assets/onnxruntime-linux-x64-1.22.0.tgz',
    name: 'onnx',
    dir: path.join(DEPENDENCIES_DIR, 'shared', 'lib'),
    needsExtraction: true
  },
  uv: {
    url: getUvDownloadUrl(),
    name: 'uv',
    dir: path.join(DEPENDENCIES_DIR, 'shared', 'bin'),
    needsExtraction: true,
    postInstallHook: async (dependencyDir: string) => {
      await processUvInstallation(dependencyDir)

      try {
        await createVenvAfterUvInstallation()
        console.log(
          `[downloadDependencies] Virtual environment created successfully after UV installation`
        )
      } catch (error) {
        console.error(
          `[downloadDependencies] Failed to create virtual environment after UV installation:`,
          error
        )
      }
    }
  }
}

export function getDependencyPath(dependencyName: DependencyName): string {
  const cfg = DEPENDENCIES_CONFIGS[dependencyName]
  if (!cfg) {
    throw new Error(`Unknown dependency: ${dependencyName}`)
  }
  return cfg.dir
}

function isDependencyProperlyDownloaded(dependencyName: DependencyName): boolean {
  const cfg = DEPENDENCIES_CONFIGS[dependencyName]
  if (!cfg) {
    return false
  }

  const dependencyPath = cfg.dir

  if (!cfg.needsExtraction) {
    return fs.existsSync(dependencyPath)
  }

  // For extracted dependencies, check if directory exists and has extracted content
  if (!fs.existsSync(dependencyPath)) {
    return false
  }

  try {
    const files = fs.readdirSync(dependencyPath)

    // Directory must have files
    if (files.length === 0) {
      return false
    }

    // Filter out archive files - we want extracted content, not just downloaded archives
    const nonArchiveFiles = files.filter((file) => {
      const ext = path.extname(file).toLowerCase()
      return !ext.match(/\.(zip|tar|tgz|tar\.gz)$/)
    })

    if (nonArchiveFiles.length === 0) {
      return false
    }

    const validFiles = nonArchiveFiles.filter((file) => {
      const filePath = path.join(dependencyPath, file)
      const stat = fs.statSync(filePath)

      if (stat.isDirectory()) {
        return true
      }

      return stat.size > 0
    })

    return validFiles.length > 0
  } catch (error) {
    console.error(`[Dependencies] Error checking ${dependencyName}:`, error)
    return false
  }
}

export function hasDependenciesDownloaded(): Record<DependencyName, boolean> {
  return {
    embeddings: isDependencyProperlyDownloaded('embeddings'),
    anonymizer: isDependencyProperlyDownloaded('anonymizer'),
    onnx: isDependencyProperlyDownloaded('onnx'),
    LLAMACCP: isDependencyProperlyDownloaded('LLAMACCP'),
    uv: isDependencyProperlyDownloaded('uv')
  }
}

export function getVirtualEnvironmentPath(): string {
  return path.join(DEPENDENCIES_DIR, 'python', 'venv')
}

export function hasVirtualEnvironment(): boolean {
  const venvPath = path.join(DEPENDENCIES_DIR, 'python', 'venv')

  if (!fs.existsSync(venvPath)) {
    return false
  }

  try {
    const files = fs.readdirSync(venvPath)
    const hasBinOrScripts = files.some((file) => file === 'bin' || file === 'Scripts')
    const hasLib = files.some((file) => file === 'lib')

    return hasBinOrScripts && hasLib
  } catch (error) {
    console.error(`[hasVirtualEnvironment] Error checking venv:`, error)
    return false
  }
}

export async function downloadDependency(dependencyName: DependencyName) {
  const cfg = DEPENDENCIES_CONFIGS[dependencyName]
  if (!cfg) {
    console.error(`[downloadDependencies] Unknown dependency: ${dependencyName}`)
    throw new Error(`Unknown dependency: ${dependencyName}`)
  }

  console.log(`[downloadDependencies] Dependency config found:`, { name: cfg.name, url: cfg.url })

  const { capture } = await import('./analytics')
  const startTime = Date.now()
  capture('dependency_installation_started', {
    dependency: dependencyName
  })

  const dependencyDir = cfg.dir

  if (!fs.existsSync(dependencyDir)) {
    fs.mkdirSync(dependencyDir, { recursive: true })
  }
  const cleanUrl = cfg.url.split('?')[0] // Remove query parameters
  let urlExtension = path.extname(cleanUrl)

  if (cleanUrl.endsWith('.tar.gz')) {
    urlExtension = '.tar.gz'
  } else if (cleanUrl.endsWith('.tgz')) {
    urlExtension = '.tgz'
  }

  const isTarGz = urlExtension === '.tgz' || urlExtension === '.tar.gz'
  const fileName = cfg.needsExtraction
    ? `${dependencyName}${urlExtension}`
    : `${dependencyName}${urlExtension}`
  const tmpFile = path.join(dependencyDir, fileName)
  let total = 0

  try {
    const resp = await axios.get(cfg.url, {
      responseType: 'stream'
    })

    total = Number(resp.headers['content-length'] || 0)
    let downloaded = 0

    console.log(
      `[downloadDependencies] Total file size: ${total} bytes (${(total / 1024 / 1024).toFixed(2)} MB)`
    )

    windowManager.mainWindow?.webContents.send('models:progress', {
      modelName: dependencyName,
      pct: 0,
      totalBytes: total,
      downloadedBytes: 0
    })

    await new Promise<void>((resolve, reject) => {
      console.log(`[downloadDependencies] Creating write stream to: ${tmpFile}`)
      const ws = fs.createWriteStream(tmpFile)

      resp.data.on('data', (chunk: Buffer) => {
        downloaded += chunk.length
        const pct = total > 0 ? Math.round((downloaded / total) * 100) : 0

        // Send progress every 10% or every 50MB
        if (pct % 10 === 0 || downloaded % (50 * 1024 * 1024) < chunk.length) {
          windowManager.mainWindow?.webContents.send('models:progress', {
            modelName: dependencyName,
            pct: pct === 100 ? 99 : pct, // Workaround for waiting until file is extracted
            totalBytes: total,
            downloadedBytes: downloaded
          })
        }
      })

      resp.data.pipe(ws)

      ws.on('finish', () => {
        resolve()
      })

      ws.on('error', (error) => {
        console.error(`[downloadDependencies] Write stream error:`, error)
        reject(error)
      })
    })
  } catch (error) {
    console.error(`[downloadDependencies] Download failed for ${dependencyName}:`, error)

    capture('dependency_installation_failed', {
      dependency: dependencyName,
      duration: Date.now() - startTime,
      size_bytes: total,
      success: false,
      error: error instanceof Error ? error.message : 'Download failed'
    })

    if (fs.existsSync(tmpFile)) {
      try {
        fs.unlinkSync(tmpFile)
      } catch (cleanupError) {
        console.error(`[downloadDependencies] Failed to cleanup temp file:`, cleanupError)
      }
    }

    windowManager.mainWindow?.webContents.send('models:progress', {
      modelName: dependencyName,
      pct: 0,
      totalBytes: 0,
      downloadedBytes: 0,
      error: error instanceof Error ? error.message : 'Download failed'
    })

    throw error
  }

  try {
    if (cfg.needsExtraction) {
      if (isTarGz) {
        console.log(`[downloadDependencies] Starting TAR extraction to: ${dependencyDir}`)
        await tar.extract({
          file: tmpFile,
          cwd: dependencyDir
        })
        console.log(`[downloadDependencies] TAR extraction completed`)
      } else {
        console.log(`[downloadDependencies] Starting ZIP extraction to: ${dependencyDir}`)
        await extract(tmpFile, { dir: dependencyDir })
        console.log(`[downloadDependencies] ZIP extraction completed`)
      }

      // Remove the temporary file after extraction
      fs.unlinkSync(tmpFile)
    } else {
      console.log(
        `[downloadDependencies] No extraction needed for ${dependencyName}, file ready to use`
      )
    }

    console.log(
      `[downloadDependencies] ${dependencyName} download ${cfg.needsExtraction ? 'and extraction' : ''} completed successfully:`
    )

    if (cfg.postInstallHook) {
      try {
        await cfg.postInstallHook(dependencyDir)
        console.log(`[downloadDependencies] Post-install hook completed for ${dependencyName}`)
      } catch (error) {
        console.error(
          `[downloadDependencies] Post-install hook failed for ${dependencyName}:`,
          error
        )
      }
    }

    capture('dependency_installation_completed', {
      dependency: dependencyName,
      duration: Date.now() - startTime,
      size_bytes: total,
      success: true
    })

    windowManager.mainWindow?.webContents.send('models:progress', {
      modelName: dependencyName,
      pct: 100,
      totalBytes: total,
      downloadedBytes: total
    })

    return { success: true, path: dependencyDir }
  } catch (error) {
    console.error(
      `[downloadDependencies] ${cfg.needsExtraction ? 'Extraction' : 'Processing'} failed for ${dependencyName}:`,
      error
    )

    if (fs.existsSync(tmpFile)) {
      try {
        fs.unlinkSync(tmpFile)
      } catch (cleanupError) {
        console.error(`[downloadDependencies] Failed to cleanup temp file:`, cleanupError)
      }
    }

    windowManager.mainWindow?.webContents.send('models:progress', {
      modelName: dependencyName,
      pct: 0,
      totalBytes: total,
      downloadedBytes: 0,
      error:
        error instanceof Error
          ? error.message
          : `${cfg.needsExtraction ? (isTarGz ? 'TAR' : 'ZIP') + ' extraction' : 'Processing'} failed`
    })

    throw error
  }
}

export async function createVenvAfterUvInstallation(): Promise<void> {
  console.log(`[createVenvAfterUvInstallation] Checking if UV is ready and creating venv`)

  try {
    if (!isDependencyProperlyDownloaded('uv')) {
      throw new Error('UV must be installed before creating virtual environment')
    }

    await createVirtualEnvironment()

    console.log(`[createVenvAfterUvInstallation] Virtual environment created successfully`)
  } catch (error) {
    console.error(`[createVenvAfterUvInstallation] Failed to create virtual environment:`, error)
    throw error
  }
}

async function processUvInstallation(uvDir: string): Promise<void> {
  console.log(`[processUvInstallation] Processing UV installation in: ${uvDir}`)

  try {
    const files = fs.readdirSync(uvDir)
    const uvBinary = files.find((file) => file === 'uv' || file === 'uv.exe')

    if (!uvBinary) {
      throw new Error('UV binary not found in extracted directory')
    }

    const uvBinaryPath = path.join(uvDir, uvBinary)

    if (process.platform !== 'win32') {
      fs.chmodSync(uvBinaryPath, 0o755)
      console.log(`[processUvInstallation] Made UV binary executable: ${uvBinaryPath}`)
    }

    console.log(`[processUvInstallation] UV installation completed successfully`)
  } catch (error) {
    console.error(`[processUvInstallation] Failed to process UV installation:`, error)
    throw error
  }
}

async function createVirtualEnvironment(): Promise<void> {
  console.log(`[createVirtualEnvironment] Creating Python virtual environment`)

  try {
    const uvPath = getDependencyPath('uv')
    const venvPath = path.join(DEPENDENCIES_DIR, 'python', 'venv')

    if (!isDependencyProperlyDownloaded('uv')) {
      throw new Error('UV must be installed before creating virtual environment')
    }

    const uvFiles = fs.readdirSync(uvPath)
    const uvBinary = uvFiles.find((file) => file === 'uv' || file === 'uv.exe')
    if (!uvBinary) {
      throw new Error('UV binary not found')
    }

    const uvBinaryPath = path.join(uvPath, uvBinary)

    const { spawn } = await import('child_process')

    return new Promise((resolve, reject) => {
      const uvProcess = spawn(uvBinaryPath, ['venv', venvPath], {
        stdio: 'pipe'
      })

      let stderr = ''

      uvProcess.stdout?.on('data', (data) => {
        console.log(`[createVirtualEnvironment] UV stdout: ${data.toString()}`)
      })

      uvProcess.stderr?.on('data', (data) => {
        stderr += data.toString()
        console.log(`[createVirtualEnvironment] UV stderr: ${data.toString()}`)
      })

      uvProcess.on('close', (code) => {
        if (code === 0) {
          console.log(
            `[createVirtualEnvironment] Virtual environment created successfully at: ${venvPath}`
          )
          resolve()
        } else {
          const error = new Error(`UV venv creation failed with code ${code}: ${stderr}`)
          console.error(`[createVirtualEnvironment] Failed:`, error)
          reject(error)
        }
      })

      uvProcess.on('error', (error) => {
        console.error(`[createVirtualEnvironment] UV process error:`, error)
        reject(error)
      })
    })
  } catch (error) {
    console.error(`[createVirtualEnvironment] Failed to create virtual environment:`, error)
    throw error
  }
}
