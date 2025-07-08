import { app } from 'electron'
import path from 'path'
import fs from 'fs'
import axios from 'axios'
import extract from 'extract-zip'
import tar from 'tar'

import { windowManager } from './windows'
import { DependencyName, DependencyStatus } from './types/dependencies'

const DEPENDENCIES_DIR = path.join(app.getPath('appData'), 'Enchanted', 'models')

const DEPENDENCIES_CONFIGS: Record<DependencyName, { url: string; name: string }> = {
  embeddings: {
    url: 'https://d3o88a4htgfnky.cloudfront.net/models/jina-embeddings-v2-base-en.zip',
    name: 'embeddings'
  },
  anonymizer: {
    url: 'https://d3o88a4htgfnky.cloudfront.net/models/jina-embeddings-v2-base-en.zip',
    name: 'anonymizer'
  },
  onnx: {
    url:
      process.platform === 'darwin' && process.arch === 'arm64'
        ? 'https://d3o88a4htgfnky.cloudfront.net/assets/onnxruntime-osx-arm64-1.22.0.tgz'
        : 'https://d3o88a4htgfnky.cloudfront.net/assets/onnxruntime-linux-x64-1.22.0.tgz',
    name: 'onnx'
  }
}

export function hasDependenciesDownloaded(): DependencyStatus {
  const embeddingsDir = path.join(DEPENDENCIES_DIR, 'embeddings')
  const anonymizerDir = path.join(DEPENDENCIES_DIR, 'anonymizer')
  const onnxDir = path.join(DEPENDENCIES_DIR, 'onnx')

  const embeddingsExists = fs.existsSync(embeddingsDir) && fs.readdirSync(embeddingsDir).length > 0
  const anonymizerExists = fs.existsSync(anonymizerDir) && fs.readdirSync(anonymizerDir).length > 0
  const onnxExists = fs.existsSync(onnxDir) && fs.readdirSync(onnxDir).length > 0

  return {
    embeddings: embeddingsExists,
    anonymizer: anonymizerExists,
    onnx: onnxExists
  }
}

export async function downloadDependency(dependencyName: DependencyName) {
  const cfg = DEPENDENCIES_CONFIGS[dependencyName]
  if (!cfg) {
    console.error(`[downloadDependencies] Unknown dependency: ${dependencyName}`)
    throw new Error(`Unknown dependency: ${dependencyName}`)
  }

  console.log(`[downloadDependencies] Dependency config found:`, { name: cfg.name, url: cfg.url })

  const dependencyDir = path.join(DEPENDENCIES_DIR, cfg.name)

  if (!fs.existsSync(dependencyDir)) {
    fs.mkdirSync(dependencyDir, { recursive: true })
  }
  // Determine file extension from URL
  const urlExtension = path.extname(cfg.url)
  const isTarGz = urlExtension === '.tgz' || urlExtension === '.tar.gz'
  const tmpFile = path.join(dependencyDir, `${dependencyName}${urlExtension}`)
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

    fs.unlinkSync(tmpFile)

    console.log(
      `[downloadDependencies] ${dependencyName} download and extraction completed successfully:`
    )

    windowManager.mainWindow?.webContents.send('models:progress', {
      modelName: dependencyName,
      pct: 100,
      totalBytes: total,
      downloadedBytes: total
    })

    return { success: true, path: dependencyDir }
  } catch (error) {
    console.error(`[downloadDependencies] Extraction failed for ${dependencyName}:`, error)

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
      error: error instanceof Error ? error.message : `${isTarGz ? 'TAR' : 'ZIP'} extraction failed`
    })

    throw error
  }
}
