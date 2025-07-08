import { app } from 'electron'
import path from 'path'
import fs from 'fs'
import axios from 'axios'
import extract from 'extract-zip'
import { windowManager } from './windows'

const MODELS_DIR = path.join(app.getPath('appData'), 'Enchanted', 'models')

type ModelName = 'embeddings' | 'anonymizer'

const MODEL_CONFIGS: Record<ModelName, { url: string; name: string }> = {
  embeddings: {
    url: 'https://d3o88a4htgfnky.cloudfront.net/models/jina-embeddings-v2-base-en.zip',
    name: 'embeddings'
  },
  anonymizer: {
    url: 'https://d3o88a4htgfnky.cloudfront.net/models/jina-embeddings-v2-base-en.zip',
    name: 'anonymizer'
  }
}

export function hasModelsDownloaded(): {
  embeddings: boolean
  anonymizer: boolean
} {
  const embeddingsDir = path.join(MODELS_DIR, 'embeddings')
  const anonymizerDir = path.join(MODELS_DIR, 'anonymizer')

  const embeddingsExists = fs.existsSync(embeddingsDir) && fs.readdirSync(embeddingsDir).length > 0
  const anonymizerExists = fs.existsSync(anonymizerDir) && fs.readdirSync(anonymizerDir).length > 0

  return {
    embeddings: embeddingsExists,
    anonymizer: anonymizerExists
  }
}

export async function downloadModels(modelName: ModelName) {
  const cfg = MODEL_CONFIGS[modelName]
  if (!cfg) {
    console.error(`[downloadModels] Unknown model: ${modelName}`)
    throw new Error(`Unknown model: ${modelName}`)
  }

  console.log(`[downloadModels] Model config found:`, { name: cfg.name, url: cfg.url })

  const modelDir = path.join(MODELS_DIR, cfg.name)

  if (!fs.existsSync(modelDir)) {
    fs.mkdirSync(modelDir, { recursive: true })
  }
  const tmpZip = path.join(modelDir, `${modelName}.zip`)

  const resp = await axios.get(cfg.url, {
    responseType: 'stream'
  })

  const total = Number(resp.headers['content-length'] || 0)
  let downloaded = 0

  console.log(
    `[downloadModels] Total file size: ${total} bytes (${(total / 1024 / 1024).toFixed(2)} MB)`
  )

  windowManager.mainWindow?.webContents.send('models:progress', {
    modelName,
    pct: 0,
    totalBytes: total,
    downloadedBytes: 0
  })

  await new Promise<void>((resolve, reject) => {
    console.log(`[downloadModels] Creating write stream to: ${tmpZip}`)
    const ws = fs.createWriteStream(tmpZip)

    resp.data.on('data', (chunk: Buffer) => {
      downloaded += chunk.length
      const pct = total > 0 ? Math.round((downloaded / total) * 100) : 0

      // Send progress every 10% or every 50MB
      if (pct % 10 === 0 || downloaded % (50 * 1024 * 1024) < chunk.length) {
        windowManager.mainWindow?.webContents.send('models:progress', {
          modelName,
          pct,
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
      console.error(`[downloadModels] Write stream error:`, error)
      reject(error)
    })
  })

  console.log(`[downloadModels] Starting ZIP extraction to: ${modelDir}`)
  await extract(tmpZip, { dir: modelDir })
  console.log(`[downloadModels] ZIP extraction completed`)

  console.log(`[downloadModels] Removing temporary ZIP file: ${tmpZip}`)
  fs.unlinkSync(tmpZip)

  console.log(`[downloadModels] Model download completed successfully: ${modelName}`)

  windowManager.mainWindow?.webContents.send('models:progress', {
    modelName,
    pct: 100,
    totalBytes: total,
    downloadedBytes: total
  })

  return { success: true, path: modelDir }
}
