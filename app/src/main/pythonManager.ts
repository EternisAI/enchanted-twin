import { app } from 'electron'
import path from 'node:path'
import fs, { constants as fsc } from 'node:fs'
import { pipeline } from 'node:stream/promises'
import { tmpdir } from 'node:os'
import http from 'node:http'
import https from 'node:https'
import { spawn, SpawnOptions } from 'node:child_process'
import unzipper from 'unzipper'
import log from 'electron-log/main'

export interface DependencyProgress {
  dependency: string
  progress: number
  status: string
  error?: string
}

type RunOptions = SpawnOptions & { label: string }

/* ─────────────────────────────────────────────────────────────────────────── */

export class KokoroBootstrap {
  private readonly USER_DIR = app.getPath('userData')
  private readonly USER_BIN = path.join(this.USER_DIR, 'bin')
  private readonly UV_PATH = path.join(
    this.USER_BIN,
    process.platform === 'win32' ? 'uv.exe' : 'uv'
  )

  private readonly KOKORO_DIR = path.join(this.USER_DIR, 'dependencies', 'kokoro')
  private readonly VENV_DIR = path.join(this.KOKORO_DIR, '.venv')

  private readonly ZIP_URL =
    'https://github.com/remsky/Kokoro-FastAPI/archive/refs/heads/master.zip'

  /** absolute path to the python executable in the venv */
  private pythonBin(): string {
    const sub =
      process.platform === 'win32' ? path.join('Scripts', 'python.exe') : path.join('bin', 'python')
    return path.join(this.VENV_DIR, sub)
  }

  private kokoroProc: import('child_process').ChildProcess | null = null
  private onProgress?: (data: DependencyProgress) => void
  private latestProgress: DependencyProgress = {
    dependency: 'Kokoro',
    progress: 0,
    status: 'Not started'
  }

  constructor(onProgress?: (data: DependencyProgress) => void) {
    this.onProgress = onProgress
  }

  getLatestProgress() {
    return this.latestProgress
  }

  /* ── helpers ────────────────────────────────────────────────────────────── */
  private async exists(p: string, mode = fsc.F_OK) {
    try {
      await fs.promises.access(p, mode)
      return true
    } catch {
      return false
    }
  }

  private uvEnv(venv: string) {
    const bin = process.platform === 'win32' ? path.join(venv, 'Scripts') : path.join(venv, 'bin')
    return { ...process.env, VIRTUAL_ENV: venv, PATH: `${bin}${path.delimiter}${process.env.PATH}` }
  }

  private run(cmd: string, args: readonly string[], opts: RunOptions) {
    return new Promise<void>((resolve, reject) => {
      log.info(`[Kokoro] [${opts.label}] → ${cmd} ${args.join(' ')}`)
      const p = spawn(cmd, args, { ...opts, stdio: 'pipe' })

      p.stdout?.on('data', (data) => {
        const output = data.toString().trim()
        if (output) {
          log.info(`[Kokoro] [${opts.label}] ${output}`)
        }
      })

      p.stderr?.on('data', (data) => {
        const output = data.toString().trim()
        if (output) {
          log.error(`[Kokoro] [${opts.label}] ${output}`)
        }
      })

      p.once('error', reject)
      p.once('exit', (c) => (c === 0 ? resolve() : reject(new Error(`${opts.label} exit ${c}`))))
    })
  }

  private download(url: string, dest: string, redirects = 5): Promise<void> {
    return new Promise((resolve, reject) => {
      const req = (u: string, r: number) =>
        https
          .get(u, { headers: { 'User-Agent': 'kokoro-installer' } }, (res) => {
            const { statusCode, headers } = res
            if (statusCode && statusCode >= 300 && statusCode < 400 && headers.location) {
              return r ? req(headers.location, r - 1) : reject(new Error('Too many redirects'))
            }
            if (statusCode !== 200) {
              res.resume()
              return reject(new Error(`HTTP ${statusCode} on ${u}`))
            }
            pipeline(res, fs.createWriteStream(dest)).then(resolve).catch(reject)
          })
          .on('error', reject)
      req(url, redirects)
    })
  }

  private async extractZipFlattened(src: string, dest: string) {
    const zip = await unzipper.Open.file(src)
    for (const e of zip.files) {
      if (e.type === 'Directory') continue
      const rel = e.path.split(/[/\\]/).slice(1)
      if (!rel.length) continue
      const out = path.join(dest, ...rel)
      await fs.promises.mkdir(path.dirname(out), { recursive: true })
      await new Promise<void>((res, rej) =>
        e.stream().pipe(fs.createWriteStream(out)).on('finish', res).on('error', rej)
      )
    }
  }

  /* ── install steps ──────────────────────────────────────────────────────── */
  private async ensureUv() {
    if (await this.exists(this.UV_PATH, fsc.X_OK)) {
      log.info('[Kokoro] UV already installed')
      return
    }
    log.info('[Kokoro] Installing UV package manager')
    await fs.promises.mkdir(this.USER_BIN, { recursive: true })
    await this.run('sh', ['-c', 'curl -LsSf https://astral.sh/uv/install.sh | sh'], {
      label: 'uv-install',
      env: { ...process.env, UV_INSTALL_DIR: this.USER_BIN }
    })
  }

  private async ensurePython312() {
    await this.run(this.UV_PATH, ['python', 'install', '3.12', '--quiet'], { label: 'py312' })
  }

  private async ensureRepo() {
    if (await this.exists(path.join(this.KOKORO_DIR, 'api'))) {
      log.info('[Kokoro] Repository already exists')
      return
    }
    log.info('[Kokoro] Downloading Kokoro repository')
    await fs.promises.rm(this.KOKORO_DIR, { recursive: true, force: true }).catch(() => {})
    await fs.promises.mkdir(this.KOKORO_DIR, { recursive: true })

    const zipTmp = path.join(tmpdir(), `kokoro-${Date.now()}.zip`)
    try {
      log.info('[Kokoro] Downloading repository archive')
      await this.download(this.ZIP_URL, zipTmp)
      log.info('[Kokoro] Extracting repository archive')
      await this.extractZipFlattened(zipTmp, this.KOKORO_DIR)
    } finally {
      await fs.promises.unlink(zipTmp).catch(() => {})
    }
  }

  private async ensureVenv() {
    const cfg = path.join(this.VENV_DIR, 'pyvenv.cfg')

    let venvIs312 = false
    if (await this.exists(cfg)) {
      const txt = await fs.promises.readFile(cfg, 'utf8')
      venvIs312 = /^version = 3\.12\./m.test(txt)
    }

    if (venvIs312) {
      log.info('[Kokoro] Virtual environment already exists with Python 3.12')
      return
    }

    log.info('[Kokoro] Creating Python 3.12 virtual environment')
    if (await this.exists(this.VENV_DIR)) {
      log.info('[Kokoro] Removing existing virtual environment')
      await fs.promises.rm(this.VENV_DIR, { recursive: true, force: true }).catch(() => {})
    }

    await this.run(this.UV_PATH, ['venv', '--python', '3.12', this.VENV_DIR], {
      label: 'uv-venv'
    })
  }

  private async ensureDeps() {
    const stamp = path.join(this.VENV_DIR, '.kokoro-installed')
    if (await this.exists(stamp)) {
      log.info('[Kokoro] Dependencies already installed')
      return
    }

    log.info('[Kokoro] Installing Python dependencies')
    await this.run(this.UV_PATH, ['pip', 'install', '-e', '.'], {
      cwd: this.KOKORO_DIR,
      env: this.uvEnv(this.VENV_DIR),
      label: 'uv-pip'
    })

    await fs.promises.writeFile(stamp, '')
    log.info('[Kokoro] Dependencies installation completed')
  }

  private async startTts() {
    /* print interpreter info */
    await this.run(
      this.pythonBin(),
      [
        '-c',
        'import sys;print("\\n=== Kokoro Runtime ===");' +
          'print("Python:",sys.version);print("Exec:",sys.executable);print("====================\\n")'
      ],
      { cwd: this.KOKORO_DIR, env: this.uvEnv(this.VENV_DIR), label: 'python-info' }
    )

    const env = {
      ...this.uvEnv(this.VENV_DIR),
      USE_GPU: 'true',
      USE_ONNX: 'false',
      PYTHONPATH: `${this.KOKORO_DIR}${path.delimiter}${path.join(this.KOKORO_DIR, 'api')}`,
      MODEL_DIR: 'src/models',
      VOICES_DIR: 'src/voices/v1_0',
      WEB_PLAYER_PATH: `${this.KOKORO_DIR}/web`,
      DEVICE_TYPE: 'mps',
      PYTORCH_ENABLE_MPS_FALLBACK: '1',
      TORCHVISION_DISABLE_META_REGISTRATION: '1'
    }

    /* download model */
    await this.run(
      this.pythonBin(),
      ['docker/scripts/download_model.py', '--output', 'api/src/models/v1_0'],
      { cwd: this.KOKORO_DIR, env, label: 'model-dl' }
    )

    /* start uvicorn */
    this.kokoroProc?.kill()
    log.info('[Kokoro] Starting uvicorn server on port 45000')
    this.kokoroProc = spawn(
      this.pythonBin(),
      ['-m', 'uvicorn', 'api.src.main:app', '--host', '0.0.0.0', '--port', '45000'],
      { cwd: this.KOKORO_DIR, env, stdio: 'pipe' }
    )

    this.kokoroProc.stdout?.on('data', (data) => {
      const output = data.toString().trim()
      if (output) {
        log.info(`[Kokoro] [uvicorn] ${output}`)
      }
    })

    this.kokoroProc.stderr?.on('data', (data) => {
      const output = data.toString().trim()
      if (output) {
        log.error(`[Kokoro] [uvicorn] ${output}`)
      }
    })

    this.kokoroProc.on('exit', (code) => {
      log.info(`[Kokoro] uvicorn server exited with code ${code}`)
      this.kokoroProc = null
    })

    const checkServer = () =>
      new Promise<boolean>((resolve) => {
        const req = http.get('http://localhost:45000/web', (res) => {
          res.resume()
          resolve(res.statusCode === 200 || res.statusCode === 307)
        })
        req.on('error', () => resolve(false))
      })

    log.info('[Kokoro] Waiting for server to become ready...')
    const start = Date.now()
    const timeout = 10 * 60 * 1000
    let checkCount = 0
    while (Date.now() - start < timeout) {
      if (await checkServer()) {
        log.info('[Kokoro] Server is ready and responding!')
        this.onProgress?.({ dependency: 'Kokoro', progress: 100, status: 'Completed' })
        this.latestProgress = { dependency: 'Kokoro', progress: 100, status: 'Completed' }
        return
      }
      checkCount++
      if (checkCount % 10 === 0) {
        log.info(
          `[Kokoro] Still waiting for server... (${Math.round((Date.now() - start) / 1000)}s elapsed)`
        )
      }
      await new Promise((r) => setTimeout(r, 1000))
    }

    log.error('[Kokoro] Timed out waiting for server to start')
    throw new Error('Timed out waiting for Kokoro server to start')
  }

  async setup() {
    log.info('[Kokoro] Starting Kokoro setup process')
    try {
      this.onProgress?.({
        dependency: 'Kokoro',
        progress: 10,
        status: 'Setting up dependency manager'
      })
      this.latestProgress = {
        dependency: 'Kokoro',
        progress: 10,
        status: 'Setting up dependency manager'
      }
      await this.ensureUv()
      this.onProgress?.({ dependency: 'Kokoro', progress: 20, status: 'Installing Python' })
      this.latestProgress = { dependency: 'Kokoro', progress: 20, status: 'Installing Python' }
      await this.ensurePython312()
      this.onProgress?.({ dependency: 'Kokoro', progress: 30, status: 'Downloading Kokoro' })
      this.latestProgress = { dependency: 'Kokoro', progress: 30, status: 'Downloading Kokoro' }
      await this.ensureRepo()
      this.onProgress?.({
        dependency: 'Kokoro',
        progress: 45,
        status: 'Creating virtual environment'
      })
      this.latestProgress = {
        dependency: 'Kokoro',
        progress: 45,
        status: 'Creating virtual environment'
      }
      await this.ensureVenv()
      this.onProgress?.({ dependency: 'Kokoro', progress: 60, status: 'Installing dependencies' })
      this.latestProgress = {
        dependency: 'Kokoro',
        progress: 60,
        status: 'Installing dependencies'
      }
      await this.ensureDeps()
      this.onProgress?.({ dependency: 'Kokoro', progress: 90, status: 'Starting speech model' })
      this.latestProgress = { dependency: 'Kokoro', progress: 90, status: 'Starting speech model' }
      await this.startTts()
    } catch (e) {
      const error = e instanceof Error ? e.message : 'Unknown error occurred'
      log.error('[Kokoro] KokoroBootstrap failed', e)
      this.latestProgress = {
        dependency: 'Kokoro',
        progress: this.latestProgress.progress,
        status: 'Failed',
        error
      }
      this.onProgress?.({ dependency: 'Kokoro', progress: 0, status: 'Failed', error })
      throw e
    }
  }

  async cleanup() {
    try {
      this.kokoroProc?.kill()
    } finally {
      this.kokoroProc = null
    }
  }
}
