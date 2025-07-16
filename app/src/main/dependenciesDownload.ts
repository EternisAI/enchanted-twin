import { app } from 'electron'
import path from 'path'
import fs from 'fs'
import axios from 'axios'
import extract from 'extract-zip'
import * as tar from 'tar'

import { windowManager } from './windows'
import { DependencyName } from './types/dependencies'

const DEPENDENCIES_DIR = path.join(app.getPath('appData'), 'enchanted')

const DEPENDENCIES_CONFIGS: Record<
  DependencyName,
  { url: string; name: string; dir: string; needsExtraction: boolean }
> = {
  embeddings: {
    url: 'https://d3o88a4htgfnky.cloudfront.net/models/jina-embeddings-v2-base-en.zip',
    name: 'embeddings',
    dir: path.join(DEPENDENCIES_DIR, 'models', 'jina-embeddings-v2-base-en'),
    needsExtraction: true
  },
  anonymizer: {
    url: 'https://d3o88a4htgfnky.cloudfront.net/models/eternis_eternis_anonymizer_merge_Qwen3-0.6B_9jul_30k.zip',
    name: 'anonymizer',
    dir: path.join(
      DEPENDENCIES_DIR,
      'models',
      'eternis_eternis_anonymizer_merge_Qwen3-0.6B_9jul_30k'
    ),
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
  LLMCLI: {
    url: 'https://d3o88a4htgfnky.cloudfront.net/assets/LLMCLI',
    name: 'LLMCLI',
    dir: path.join(DEPENDENCIES_DIR, 'shared', 'lib'),
    needsExtraction: false
  }
}

export function getDependencyPath(dependencyName: DependencyName): string {
  const cfg = DEPENDENCIES_CONFIGS[dependencyName]
  if (!cfg) {
    throw new Error(`Unknown dependency: ${dependencyName}`)
  }
  return cfg.dir
}

export function hasDependenciesDownloaded(): Record<DependencyName, boolean> {
  const embeddingsDir = getDependencyPath('embeddings')
  const anonymizerDir = getDependencyPath('anonymizer')
  const onnxDir = getDependencyPath('onnx')
  const LLMCLIFile = getDependencyPath('LLMCLI')

  console.log('onnxDir', onnxDir)
  console.log('LLMCLIFile', LLMCLIFile)

  const embeddingsExists = fs.existsSync(embeddingsDir) && fs.readdirSync(embeddingsDir).length > 0
  const anonymizerExists = fs.existsSync(anonymizerDir) && fs.readdirSync(anonymizerDir).length > 0
  const onnxExists = fs.existsSync(onnxDir) && fs.readdirSync(onnxDir).length > 0
  const LLMCLIExists = fs.existsSync(LLMCLIFile)

  return {
    embeddings: embeddingsExists,
    anonymizer: anonymizerExists,
    onnx: onnxExists,
    LLMCLI: LLMCLIExists
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
  // Determine file extension from URL
  const urlExtension = path.extname(cfg.url)
  const isTarGz = urlExtension === '.tgz' || urlExtension === '.tar.gz'
  const fileName = cfg.needsExtraction ? `${dependencyName}${urlExtension}` : dependencyName
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
      // For files that don't need extraction, make sure they're executable if they're binary files
      if (dependencyName === 'LLMCLI') {
        fs.chmodSync(tmpFile, '755')
      }
    }

    console.log(
      `[downloadDependencies] ${dependencyName} download ${cfg.needsExtraction ? 'and extraction' : ''} completed successfully:`
    )

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
