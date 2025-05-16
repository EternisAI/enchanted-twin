import { app } from 'electron'
import path from 'node:path'
import fs, { constants as fsc } from 'node:fs'
import { pipeline } from 'node:stream/promises'
import { tmpdir } from 'node:os'
import https from 'node:https'
import { spawn, SpawnOptions } from 'node:child_process'
import unzipper from 'unzipper'
import log from 'electron-log/main'
import http from 'node:http'

type RunOptions = SpawnOptions & { label: string }

export class KokoroBootstrap {
  private readonly USER_DIR: string
  private readonly USER_BIN: string
  private readonly UV_EXE: string
  private readonly KOKORO_DIR: string // project root
  private readonly VENV_DIR: string // dedicated venv
  private readonly ZIP_URL: string =
    'https://github.com/remsky/Kokoro-FastAPI/archive/refs/heads/master.zip'
  private readonly onProgress?: (progress: number, status?: string) => void

  constructor(onProgress?: (progress: number, status?: string) => void) {
    this.USER_DIR = app.getPath('userData')
    this.USER_BIN = path.join(this.USER_DIR, 'bin')
    this.UV_EXE = path.join(this.USER_BIN, process.platform === 'win32' ? 'uv.exe' : 'uv')
    this.KOKORO_DIR = path.join(this.USER_DIR, 'kokoro')
    this.VENV_DIR = path.join(this.USER_DIR, 'uv_envs', 'kokoro')
    this.onProgress = onProgress
  }

  private async fileExists(p: string, mode = fsc.F_OK): Promise<boolean> {
    try {
      await fs.promises.access(p, mode)
      return true
    } catch {
      return false
    }
  }

  private uvEnv(venvPath: string): Record<string, string | undefined> {
    const binDir =
      process.platform === 'win32' ? path.join(venvPath, 'Scripts') : path.join(venvPath, 'bin')
    return {
      ...process.env,
      VIRTUAL_ENV: venvPath,
      PATH: `${binDir}${path.delimiter}${process.env.PATH ?? ''}`
    }
  }

  private run(cmd: string, args: readonly string[], opts: RunOptions): Promise<void> {
    return new Promise((resolve, reject) => {
      const t0 = Date.now()
      log.info(`[${opts.label}] → ${cmd} ${args.join(' ')}`)
      const proc = spawn(cmd, args, { ...opts, stdio: 'inherit' })
      proc.once('error', (err) => {
        log.error(`[${opts.label}] ✖ process error`, err)
        reject(err)
      })
      proc.once('exit', (code) => {
        const dt = Date.now() - t0
        if (code === 0) {
          log.info(`[${opts.label}] ✓ finished in ${dt} ms`)
          resolve()
        } else {
          log.error(`[${opts.label}] ✖ exited ${code} after ${dt} ms`)
          reject(new Error(`${opts.label} failed (exit ${code})`))
        }
      })
    })
  }

  private download(url: string, dest: string, redirects = 5): Promise<void> {
    return new Promise((resolve, reject) => {
      const requestFn = (currentUrl: string, remainingRedirects: number) => {
        https
          .get(currentUrl, { headers: { 'User-Agent': 'kokoro-installer' } }, (res) => {
            const { statusCode, headers } = res
            if (statusCode && statusCode >= 300 && statusCode < 400 && headers.location) {
              if (remainingRedirects === 0) {
                return reject(new Error('Too many redirects'))
              }
              return requestFn(headers.location, remainingRedirects - 1)
            }
            if (statusCode !== 200) {
              res.resume() // Consume data to free up resources
              return reject(new Error(`HTTP ${statusCode} on ${currentUrl}`))
            }
            pipeline(res, fs.createWriteStream(dest)).then(resolve).catch(reject)
          })
          .on('error', reject)
      }
      requestFn(url, redirects)
    })
  }

  private async extractZipFlattened(zipFile: string, destDir: string): Promise<void> {
    const zip = await unzipper.Open.file(zipFile)
    for (const entry of zip.files) {
      if (entry.type === 'Directory') continue

      const relativePathParts = entry.path.split(/[/\\]/).slice(1)

      if (relativePathParts.length === 0 || relativePathParts.join('') === '') {
        log.warn(
          `[extractZipFlattened] Skipping entry with unusual path after flattening: ${entry.path}`
        )
        continue
      }

      const outPath = path.join(destDir, ...relativePathParts)
      await fs.promises.mkdir(path.dirname(outPath), { recursive: true })
      await new Promise<void>((res, rej) =>
        entry.stream().pipe(fs.createWriteStream(outPath)).on('finish', res).on('error', rej)
      )
    }
  }

  private async ensureUv(): Promise<void> {
    if (await this.fileExists(this.UV_EXE, fsc.X_OK)) return
    await fs.promises.mkdir(this.USER_BIN, { recursive: true })
    await this.run('sh', ['-c', 'curl -LsSf https://astral.sh/uv/install.sh | sh'], {
      label: 'uv-install',
      env: { ...process.env, UV_INSTALL_DIR: this.USER_BIN }
    })
    if (!(await this.fileExists(this.UV_EXE, fsc.X_OK))) {
      throw new Error(`uv binary missing at ${this.UV_EXE} after installation attempt.`)
    }
  }

  private async ensureKokoroRepo(): Promise<void> {
    if (await this.fileExists(path.join(this.KOKORO_DIR, 'api'))) return
    await fs.promises.rm(this.KOKORO_DIR, { recursive: true, force: true }).catch(() => {})
    await fs.promises.mkdir(this.KOKORO_DIR, { recursive: true })
    const zipFileName = `kokoro-${Date.now()}.zip`
    const zipTempPath = path.join(tmpdir(), zipFileName)
    try {
      await this.download(this.ZIP_URL, zipTempPath)
      await this.extractZipFlattened(zipTempPath, this.KOKORO_DIR)
    } finally {
      await fs.promises.unlink(zipTempPath).catch((err) => {
        log.warn(`[ensureKokoroRepo] Failed to delete temp zip: ${zipTempPath}`, err)
      })
    }
  }

  private async ensureVenv(): Promise<void> {
    if (await this.fileExists(path.join(this.VENV_DIR, 'pyvenv.cfg'))) return
    await fs.promises.mkdir(path.dirname(this.VENV_DIR), { recursive: true })
    await this.run(this.UV_EXE, ['venv', '--python', '3.12', this.VENV_DIR], { label: 'uv-venv' })
  }

  private async ensureDeps(): Promise<void> {
    const stampFile = path.join(this.VENV_DIR, '.kokoro-installed')
    if (await this.fileExists(stampFile)) return

    const baseInstallArgs: string[] = ['pip', 'install', '-e', this.KOKORO_DIR]
    const requirementsPath = path.join(this.KOKORO_DIR, 'requirements.txt')

    if (await this.fileExists(requirementsPath)) {
      baseInstallArgs.push('-r', requirementsPath)
    }

    await this.run(this.UV_EXE, baseInstallArgs, {
      label: 'uv-pip-dependencies',
      env: this.uvEnv(this.VENV_DIR)
    })

    await this.run(
      this.UV_EXE,
      ['pip', 'install', '--upgrade', 'torch', 'torchaudio', 'torchvision'],
      {
        label: 'uv-pip-torch',
        env: this.uvEnv(this.VENV_DIR)
      }
    )

    await fs.promises.writeFile(stampFile, '')
  }

  private async checkTorch(): Promise<void> {
    await this.run(
      this.UV_EXE,
      [
        'run',
        'python',
        '-c',
        "import torch, sys; print(f'torch OK: {torch.__version__}, Python: {sys.version}')"
      ],
      { label: 'torch-check', env: this.uvEnv(this.VENV_DIR) }
    )
  }

  private async startTts(): Promise<void> {
    const kokoroEnv = {
      ...this.uvEnv(this.VENV_DIR),
      USE_GPU: 'true',
      USE_ONNX: 'false',
      PYTHONPATH: `${this.KOKORO_DIR}${path.delimiter}${path.join(this.KOKORO_DIR, 'api')}`,
      MODEL_DIR: 'src/models',
      VOICES_DIR: 'src/voices/v1_0',
      WEB_PLAYER_PATH: this.KOKORO_DIR,
      DEVICE_TYPE: 'mps',
      PYTORCH_ENABLE_MPS_FALLBACK: '1'
    }

    await this.run(
      this.UV_EXE,
      [
        'run',
        '--no-sync',
        'python',
        'docker/scripts/download_model.py',
        '--output',
        'api/src/models/v1_0'
      ],
      {
        cwd: this.KOKORO_DIR,
        env: kokoroEnv,
        label: 'kokoro-download-model'
      }
    )

    const proc = spawn(
      this.UV_EXE,
      ['run', '--no-sync', 'uvicorn', 'api.src.main:app', '--host', '0.0.0.0', '--port', '8765'],
      {
        cwd: this.KOKORO_DIR,
        env: kokoroEnv,
        stdio: 'inherit'
      }
    )
    proc.on('error', (err) => {
      log.error('[kokoro-tts] Failed to start:', err)
    })
    proc.on('exit', (code) => {
      log.info(`[kokoro-tts] exited with code ${code}`)
    })

    await this.waitForServer('http://127.0.0.1:8765/docs', 10000)
  }

  private async waitForServer(url: string, timeoutMs: number): Promise<void> {
    const start = Date.now()
    while (Date.now() - start < timeoutMs) {
      try {
        await new Promise<void>((resolve, reject) => {
          http
            .get(url, (res) => {
              if (res.statusCode && res.statusCode < 500) {
                resolve()
              } else {
                reject(new Error('Server not ready'))
              }
            })
            .on('error', reject)
        })
        log.info(`[waitForServer] Server is up at ${url}`)
        return
      } catch {
        await new Promise((r) => setTimeout(r, 500))
      }
    }
    throw new Error(`Server at ${url} did not become ready in ${timeoutMs}ms`)
  }

  private emitProgress(progress: number, status?: string) {
    const safeStatus = status ?? ''
    log.info(`[KokoroBootstrap] Emitting progress: ${progress}, status: ${safeStatus}`)
    if (this.onProgress) {
      this.onProgress(progress, safeStatus)
    }
  }

  public async setup(): Promise<void> {
    try {
      this.emitProgress(10, 'Ensuring UV')
      await this.ensureUv()
      this.emitProgress(25, 'Cloning Kokoro Repo')
      await this.ensureKokoroRepo()
      this.emitProgress(40, 'Creating Virtual Env')
      await this.ensureVenv()
      this.emitProgress(60, 'Installing Dependencies')
      await this.ensureDeps()
      this.emitProgress(80, 'Checking Torch')
      await this.checkTorch()
      this.emitProgress(95, 'Starting TTS')
      await this.startTts()
      this.emitProgress(100, 'Setup Complete')
    } catch (error) {
      log.error('─── PythonBootstrap: Python setup failed ❌', error)
      throw error
    }
  }
}
