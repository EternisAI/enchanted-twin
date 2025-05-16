import { app } from 'electron'
import path from 'node:path'
import fs, { constants as fsc } from 'node:fs'
import { pipeline } from 'node:stream/promises'
import { tmpdir } from 'node:os'
import https from 'node:https'
import { spawn, SpawnOptions } from 'node:child_process'
import unzipper from 'unzipper'
import log from 'electron-log/main'

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
  private onProgress?: (progress: number, status?: string) => void
  private latestProgress: { progress: number; status: string } = {
    progress: 0,
    status: 'Not started'
  }

  constructor(onProgress?: (progress: number, status?: string) => void) {
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
      log.info(`[${opts.label}] → ${cmd} ${args.join(' ')}`)
      const p = spawn(cmd, args, { ...opts, stdio: 'inherit' })
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
    if (await this.exists(this.UV_PATH, fsc.X_OK)) return
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
    if (await this.exists(path.join(this.KOKORO_DIR, 'api'))) return
    await fs.promises.rm(this.KOKORO_DIR, { recursive: true, force: true }).catch(() => {})
    await fs.promises.mkdir(this.KOKORO_DIR, { recursive: true })

    const zipTmp = path.join(tmpdir(), `kokoro-${Date.now()}.zip`)
    try {
      await this.download(this.ZIP_URL, zipTmp)
      await this.extractZipFlattened(zipTmp, this.KOKORO_DIR)
    } finally {
      await fs.promises.unlink(zipTmp).catch(() => {})
    }
  }

  private async ensureVenv() {
    const cfg = path.join(this.VENV_DIR, 'pyvenv.cfg')
    if (await this.exists(cfg)) {
      const txt = await fs.promises.readFile(cfg, 'utf8')
      if (/^version = 3\.12\./m.test(txt)) return
      await fs.promises.rm(this.VENV_DIR, { recursive: true, force: true })
    }
    await fs.promises.mkdir(path.dirname(this.VENV_DIR), { recursive: true })
    await this.run(this.UV_PATH, ['venv', '--python', '3.12', this.VENV_DIR], { label: 'uv-venv' })
  }

  private async ensureDeps() {
    const stamp = path.join(this.VENV_DIR, '.kokoro-installed')
    if (await this.exists(stamp)) return

    await this.run(this.UV_PATH, ['pip', 'install', '-e', '.'], {
      cwd: this.KOKORO_DIR,
      env: this.uvEnv(this.VENV_DIR),
      label: 'uv-pip'
    })

    await fs.promises.writeFile(stamp, '')
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
    this.kokoroProc = spawn(
      this.pythonBin(),
      ['-m', 'uvicorn', 'api.src.main:app', '--host', '0.0.0.0', '--port', '45000'],
      { cwd: this.KOKORO_DIR, env, stdio: 'inherit' }
    )
    this.kokoroProc.on('exit', () => (this.kokoroProc = null))
  }

  async setup() {
    try {
      this.onProgress?.(10, 'Setting up dependency manager')
      this.latestProgress = { progress: 10, status: 'Setting up dependency manager' }
      await this.ensureUv()
      this.onProgress?.(20, 'Installing Python')
      this.latestProgress = { progress: 20, status: 'Installing Python' }
      await this.ensurePython312()
      this.onProgress?.(30, 'Downloading Kokoro')
      this.latestProgress = { progress: 30, status: 'Downloading Kokoro' }
      await this.ensureRepo()
      this.onProgress?.(45, 'Creating virtual environment')
      this.latestProgress = { progress: 45, status: 'Creating virtual environment' }
      await this.ensureVenv()
      this.onProgress?.(60, 'Installing dependencies')
      this.latestProgress = { progress: 60, status: 'Installing dependencies' }
      await this.ensureDeps()
      this.onProgress?.(90, 'Starting speech model')
      this.latestProgress = { progress: 90, status: 'Starting speech model' }
      this.startTts()
      this.onProgress?.(100, 'Completed')
      this.latestProgress = { progress: 100, status: 'Completed' }
    } catch (e) {
      log.error('[KokoroBootstrap] failed', e)
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
