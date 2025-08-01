import { app } from 'electron'
import path from 'path'
import fs from 'fs'
import axios from 'axios'
import extract from 'extract-zip'
import * as tar from 'tar'
import { windowManager } from './windows'
import { DependencyName } from './types/dependencies'
import { LiveKitAgentBootstrap } from './livekitAgent'

const DEPENDENCIES_DIR = path.join(app.getPath('appData'), 'enchanted')

// PostgreSQL files to download
const POSTGRES_FILES = {
  binaries: [
    'bin/postgres',
    'bin/initdb', 
    'bin/pg_ctl'
  ],
  libraries: [
    'lib/libpq.5.dylib',
    'lib/libpq.dylib',
    'lib/postgresql/vector.dylib',
    'lib/postgresql/dict_snowball.dylib',
    'lib/postgresql/plpgsql.dylib',
    // Essential system libraries that PostgreSQL binaries depend on
    'lib/libicuuc.75.dylib',
    'lib/libicui18n.75.dylib',
    'lib/libicudata.75.dylib',
    'lib/libssl.3.dylib',
    'lib/libcrypto.3.dylib',
    'lib/libxml2.2.dylib',
    'lib/libzstd.1.dylib',
    'lib/liblz4.1.dylib'
  ],
  dataFiles: [
    'share/postgresql/postgres.bki',
    'share/postgresql/errcodes.txt',
    'share/postgresql/information_schema.sql',
    // Configuration sample files required by initdb
    'share/postgresql/pg_hba.conf.sample',
    'share/postgresql/pg_ident.conf.sample',
    'share/postgresql/postgresql.conf.sample',
    'share/postgresql/pg_service.conf.sample',
    'share/postgresql/psqlrc.sample',
    // System catalog and function definitions
    'share/postgresql/system_constraints.sql',
    'share/postgresql/system_functions.sql', 
    'share/postgresql/system_views.sql',
    'share/postgresql/sql_features.txt',
    'share/postgresql/snowball_create.sql',
    // Core PostgreSQL extension (required by initdb)
    'share/postgresql/extension/plpgsql.control',
    'share/postgresql/extension/plpgsql--1.0.sql',
    // pgvector extension files
    'share/postgresql/extension/vector.control',
    'share/postgresql/extension/vector--0.8.0.sql',
    // Essential timezone files - just UTC
    'share/postgresql/timezone/UTC',
    // Timezone sets - required directory structure for PostgreSQL
    'share/postgresql/timezonesets/Africa.txt',
    'share/postgresql/timezonesets/America.txt',
    'share/postgresql/timezonesets/Antarctica.txt',
    'share/postgresql/timezonesets/Asia.txt',
    'share/postgresql/timezonesets/Atlantic.txt',
    'share/postgresql/timezonesets/Australia',
    'share/postgresql/timezonesets/Australia.txt',
    'share/postgresql/timezonesets/Default',
    'share/postgresql/timezonesets/Etc.txt',
    'share/postgresql/timezonesets/Europe.txt',
    'share/postgresql/timezonesets/India',
    'share/postgresql/timezonesets/Indian.txt',
    'share/postgresql/timezonesets/Pacific.txt',
    // Text search data - just English
    'share/postgresql/tsearch_data/english.stop'
  ]
}

const DEPENDENCIES_CONFIGS: Record<
  DependencyName,
  {
    url: string
    name: string
    dir: string
    install: () => Promise<void>
    isDownloaded: () => boolean
  }
> = {
  embeddings: {
    url: 'https://d3o88a4htgfnky.cloudfront.net/models/jina-embeddings-v2-base-en.zip',
    name: 'embeddings',
    dir: path.join(DEPENDENCIES_DIR, 'models', 'jina-embeddings-v2-base-en'),
    install: async function () {
      const file = await downloadFile(
        this.url,
        this.dir,
        'embeddings.zip',
        (pct, total, downloaded) => {
          windowManager.mainWindow?.webContents.send('models:progress', {
            modelName: this.name,
            pct,
            totalBytes: total,
            downloadedBytes: downloaded
          })
        }
      )
      await extractZip(file, this.dir)
    },
    isDownloaded: function () {
      return isExtractedDirValid(this.dir)
    }
  },
  uv: {
    url: '',
    name: 'uv',
    dir: path.join(DEPENDENCIES_DIR, 'shared', 'bin'),
    install: async function () {
      if (process.env.VITE_DISABLE_VOICE === 'true') {
        return
      }

      const livekitAgent = new LiveKitAgentBootstrap({
        onProgress: (data) => {
          windowManager.mainWindow?.webContents.send('models:progress', {
            modelName: this.name,
            pct: data.progress,
            status: data.status,
            error: data.error,
            totalBytes: 0,
            downloadedBytes: 0
          })
        }
      })
      await livekitAgent.setup()
    },
    isDownloaded: function () {
      if (process.env.VITE_DISABLE_VOICE === 'true') {
        return true
      }

      const uvBin = process.platform === 'win32' ? 'uv.exe' : 'uv'
      return fs.existsSync(path.join(this.dir, uvBin))
    }
  },
  anonymizer: {
    url: 'https://d3o88a4htgfnky.cloudfront.net/models/qwen3-4b_q4_k_m.zip',
    name: 'anonymizer',
    dir: path.join(DEPENDENCIES_DIR, 'models', 'anonymizer'),
    install: async function () {
      const file = await downloadFile(
        this.url,
        this.dir,
        'anonymizer.zip',
        (pct, total, downloaded) => {
          windowManager.mainWindow?.webContents.send('models:progress', {
            modelName: this.name,
            pct,
            totalBytes: total,
            downloadedBytes: downloaded
          })
        }
      )
      await extractZip(file, this.dir)
    },
    isDownloaded: function () {
      // If ANONYMIZER_TYPE is set to "no-op", consider it downloaded
      if (process.env.ANONYMIZER_TYPE === 'no-op') {
        return true
      }

      // For anonymizer, we need both 4b and 0.6b models
      if (!isExtractedDirValid(this.dir)) {
        return false
      }

      try {
        const files = fs.readdirSync(this.dir)
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
  },
  LLAMACCP: {
    url: 'https://github.com/ggml-org/llama.cpp/releases/download/b5916/llama-b5916-bin-macos-arm64.zip',
    name: 'llamacpp',
    dir: path.join(DEPENDENCIES_DIR, 'shared', 'lib', 'llamacpp'),
    install: async function () {
      const file = await downloadFile(
        this.url,
        this.dir,
        'llamacpp.zip',
        (pct, total, downloaded) => {
          windowManager.mainWindow?.webContents.send('models:progress', {
            modelName: this.name,
            pct,
            totalBytes: total,
            downloadedBytes: downloaded
          })
        }
      )
      await extractZip(file, this.dir)
    },
    isDownloaded: function () {
      return isExtractedDirValid(this.dir)
    }
  },
  onnx: {
    url:
      process.platform === 'darwin' && process.arch === 'arm64'
        ? 'https://d3o88a4htgfnky.cloudfront.net/assets/onnxruntime-osx-arm64-1.22.0.tgz'
        : 'https://d3o88a4htgfnky.cloudfront.net/assets/onnxruntime-linux-x64-1.22.0.tgz',
    name: 'onnx',
    dir: path.join(DEPENDENCIES_DIR, 'shared', 'lib'),
    install: async function () {
      const file = await downloadFile(this.url, this.dir, 'onnx.tgz', (pct, total, downloaded) => {
        windowManager.mainWindow?.webContents.send('models:progress', {
          modelName: this.name,
          pct,
          totalBytes: total,
          downloadedBytes: downloaded
        })
      })
      await extractTarGz(file, this.dir)
    },
    isDownloaded: function () {
      // Check for the extracted onnxruntime folder (platform-specific)
      const onnxDir =
        process.platform === 'darwin' && process.arch === 'arm64'
          ? path.join(this.dir, 'onnxruntime-osx-arm64-1.22.0')
          : path.join(this.dir, 'onnxruntime-linux-x64-1.22.0')
      return isExtractedDirValid(onnxDir)
    }
  },
  postgres: {
    url: 'https://d1vu5azmz7om3b.cloudfront.net/enchanted_data/postgres',
    name: 'postgres',
    dir: path.join(DEPENDENCIES_DIR, 'postgres'),
    install: async function () {
      const baseUrl = this.url
      const allFiles = [...POSTGRES_FILES.binaries, ...POSTGRES_FILES.libraries, ...POSTGRES_FILES.dataFiles]
      let downloadedFiles = 0
      const totalFiles = allFiles.length
      
      for (const filePath of allFiles) {
        const fileUrl = `${baseUrl}/${filePath}`
        const destPath = path.join(this.dir, filePath)
        const destDir = path.dirname(destPath)
        
        // Create directory if it doesn't exist
        if (!fs.existsSync(destDir)) {
          fs.mkdirSync(destDir, { recursive: true })
        }
        
        try {
          await downloadSingleFile(fileUrl, destPath)
          downloadedFiles++
          
          // Report progress
          const pct = Math.round((downloadedFiles / totalFiles) * 100)
          windowManager.mainWindow?.webContents.send('models:progress', {
            modelName: this.name,
            pct,
            totalBytes: totalFiles,
            downloadedBytes: downloadedFiles
          })
        } catch (error) {
          console.error(`Failed to download ${filePath}:`, error)
          // Continue with other files even if one fails
        }
      }
      
      // Set executable permissions for binaries on Unix-like systems
      if (process.platform !== 'win32') {
        for (const binaryPath of POSTGRES_FILES.binaries) {
          const fullPath = path.join(this.dir, binaryPath)
          if (fs.existsSync(fullPath)) {
            fs.chmodSync(fullPath, 0o755)
          }
        }
      }
    },
    isDownloaded: function () {
      // Check for essential PostgreSQL files including required libraries and config files
      const essentialFiles = [
        'bin/postgres',
        'bin/initdb',
        'bin/pg_ctl',
        'lib/libpq.5.dylib',
        'lib/postgresql/vector.dylib',
        'lib/libicuuc.75.dylib',
        'lib/libssl.3.dylib',
        'lib/libcrypto.3.dylib',
        'share/postgresql/postgres.bki',
        'share/postgresql/pg_hba.conf.sample',
        'share/postgresql/postgresql.conf.sample',
        'share/postgresql/extension/vector.control',
        'share/postgresql/extension/plpgsql.control'
      ]
      
      return essentialFiles.every(filePath => {
        const fullPath = path.join(this.dir, filePath)
        return fs.existsSync(fullPath)
      })
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
  return {
    embeddings: DEPENDENCIES_CONFIGS.embeddings.isDownloaded(),
    anonymizer: DEPENDENCIES_CONFIGS.anonymizer.isDownloaded(),
    onnx: DEPENDENCIES_CONFIGS.onnx.isDownloaded(),
    LLAMACCP: DEPENDENCIES_CONFIGS.LLAMACCP.isDownloaded(),
    uv: DEPENDENCIES_CONFIGS.uv.isDownloaded(),
    postgres: DEPENDENCIES_CONFIGS.postgres.isDownloaded()
  }
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
