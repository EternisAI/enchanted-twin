import { app } from 'electron'
import path from 'node:path'
import fs, { constants as fsc } from 'node:fs'
import { spawn } from 'node:child_process'
import log from 'electron-log/main'

import { DependencyProgress, RunOptions } from './types/pythonManager.types'
import { PYTHON_VERSION, UV_INSTALL_SCRIPT } from './constants/pythonManager.constants'
import { InstallationError } from './errors/pythonManager.errors'

export interface PythonEnvironmentCallbacks {
  onProgress?: (data: DependencyProgress) => void
}

export const PYTHON_ENV_PROGRESS_STEPS = {
  UV_SETUP: 25,
  PYTHON_INSTALL: 50,
  VENV_CREATION: 75,
  COMPLETE: 100
} as const

export class PythonEnvironmentManager {
  private readonly USER_DIR = app.getPath('userData')
  private readonly USER_BIN = path.join(this.USER_DIR, 'bin')
  private readonly UV_PATH = path.join(
    this.USER_BIN,
    process.platform === 'win32' ? 'uv.exe' : 'uv'
  )

  private latestProgress: DependencyProgress
  private onProgress?: (progress: DependencyProgress) => void

  constructor(callbacks: PythonEnvironmentCallbacks = {}) {
    this.onProgress = callbacks.onProgress
    this.latestProgress = {
      dependency: 'Python Environment',
      progress: 0,
      status: 'Not started'
    }
  }

  getLatestProgress(): DependencyProgress {
    return this.latestProgress
  }

  private updateProgress(progress: number, status: string, error?: string) {
    const progressData: DependencyProgress = {
      dependency: 'Python Environment',
      progress,
      status,
      error
    }
    this.latestProgress = progressData
    this.onProgress?.(progressData)
  }

  // Utility methods for path management
  getProjectDir(projectName: string): string {
    return path.join(this.USER_DIR, 'dependencies', projectName)
  }

  getVenvDir(projectName: string): string {
    return path.join(this.getProjectDir(projectName), '.venv')
  }

  getPythonBin(projectName: string): string {
    const venvDir = this.getVenvDir(projectName)
    const sub =
      process.platform === 'win32' ? path.join('Scripts', 'python.exe') : path.join('bin', 'python')
    return path.join(venvDir, sub)
  }

  getUvEnv(projectName: string): Record<string, string> {
    const venvDir = this.getVenvDir(projectName)
    const bin =
      process.platform === 'win32' ? path.join(venvDir, 'Scripts') : path.join(venvDir, 'bin')
    return {
      ...process.env,
      VIRTUAL_ENV: venvDir,
      PATH: `${bin}${path.delimiter}${process.env.PATH}`
    }
  }

  private async exists(p: string, mode = fsc.F_OK): Promise<boolean> {
    try {
      await fs.promises.access(p, mode)
      return true
    } catch {
      return false
    }
  }

  private run(cmd: string, args: readonly string[], opts: RunOptions): Promise<void> {
    return new Promise<void>((resolve, reject) => {
      log.info(`[Python Environment] [${opts.label}] → ${cmd} ${args.join(' ')}`)
      const p = spawn(cmd, args, { ...opts, stdio: 'pipe' })

      p.stdout?.on('data', (data) => {
        const output = data.toString().trim()
        if (output) {
          log.info(`[Python Environment] [${opts.label}] ${output}`)
        }
      })

      p.stderr?.on('data', (data) => {
        const output = data.toString().trim()
        if (output) {
          log.error(`[Python Environment] [${opts.label}] ${output}`)
        }
      })

      p.once('error', reject)
      p.once('exit', (c) => (c === 0 ? resolve() : reject(new Error(`${opts.label} exit ${c}`))))
    })
  }

  private async ensureUv(): Promise<void> {
    if (await this.exists(this.UV_PATH, fsc.X_OK)) {
      log.info('[Python Environment] UV already installed')
      return
    }
    log.info('[Python Environment] Installing UV package manager')
    await fs.promises.mkdir(this.USER_BIN, { recursive: true })
    await this.run('sh', ['-c', UV_INSTALL_SCRIPT], {
      label: 'uv-install',
      env: { ...process.env, UV_INSTALL_DIR: this.USER_BIN }
    })
  }

  private async ensurePython(pythonVersion: string = PYTHON_VERSION): Promise<void> {
    await this.run(this.UV_PATH, ['python', 'install', pythonVersion, '--quiet'], {
      label: 'python-install'
    })
  }

  private async ensureVenv(
    projectName: string,
    pythonVersion: string = PYTHON_VERSION
  ): Promise<void> {
    const venvDir = this.getVenvDir(projectName)
    if (await this.exists(venvDir)) {
      log.info(`[Python Environment] Virtual environment already exists for ${projectName}`)
      return
    }

    log.info(
      `[Python Environment] Creating virtual environment for ${projectName} with Python ${pythonVersion}`
    )
    await this.run(this.UV_PATH, ['venv', '--python', pythonVersion, venvDir], {
      label: 'uv-venv'
    })
  }

  async setupPythonEnvironment(): Promise<void> {
    log.info('[Python Environment] Starting Python environment setup')
    try {
      this.updateProgress(PYTHON_ENV_PROGRESS_STEPS.UV_SETUP, 'Setting up UV package manager')
      await this.ensureUv()

      this.updateProgress(PYTHON_ENV_PROGRESS_STEPS.PYTHON_INSTALL, 'Installing Python')
      await this.ensurePython()

      this.updateProgress(PYTHON_ENV_PROGRESS_STEPS.COMPLETE, 'Python environment ready')
      log.info('[Python Environment] Python environment setup completed successfully')
    } catch (e) {
      const error = e instanceof Error ? e.message : 'Unknown error occurred'
      log.error('[Python Environment] Python environment setup failed', e)
      this.updateProgress(this.latestProgress.progress, 'Failed', error)

      if (e instanceof InstallationError) {
        throw e
      }
      throw new InstallationError(error, 'python-environment-setup')
    }
  }

  async setupProjectVenv(
    projectName: string,
    pythonVersion: string = PYTHON_VERSION
  ): Promise<void> {
    log.info(`[Python Environment] Setting up virtual environment for ${projectName}`)
    try {
      await this.ensureVenv(projectName, pythonVersion)
      log.info(`[Python Environment] Virtual environment for ${projectName} setup completed`)
    } catch (e) {
      const error = e instanceof Error ? e.message : 'Unknown error occurred'
      log.error(`[Python Environment] Virtual environment setup failed for ${projectName}`, e)
      throw new InstallationError(error, 'venv-setup')
    }
  }

  async ensureProjectDirectory(projectName: string): Promise<void> {
    const projectDir = this.getProjectDir(projectName)
    await fs.promises.mkdir(projectDir, { recursive: true })
  }

  async installDependencies(projectName: string, dependencies: string[]): Promise<void> {
    const projectDir = this.getProjectDir(projectName)
    const venvDir = this.getVenvDir(projectName)
    const stamp = path.join(venvDir, `.${projectName}-installed`)

    if (await this.exists(stamp)) {
      log.info(`[Python Environment] Dependencies already installed for ${projectName}`)
      return
    }

    log.info(`[Python Environment] Installing Python dependencies for ${projectName}`)

    // Create requirements.txt
    const requirementsFile = path.join(projectDir, 'requirements.txt')
    await fs.promises.writeFile(requirementsFile, dependencies.join('\n'))

    await this.run(this.UV_PATH, ['pip', 'install', '-r', 'requirements.txt'], {
      cwd: projectDir,
      env: this.getUvEnv(projectName),
      label: 'uv-pip'
    })

    await fs.promises.writeFile(stamp, '')
    log.info(`[Python Environment] Dependencies installation completed for ${projectName}`)
  }
}
