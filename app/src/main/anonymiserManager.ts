import { spawn } from 'node:child_process'
import log from 'electron-log/main'
import path from 'node:path'
import fs from 'node:fs'

import { PythonEnvironmentManager } from './pythonEnvironmentManager'
import { copyDirectoryRecursive, fileExists } from './helpers'
import { getDependencyPath, hasDependenciesDownloaded } from './dependenciesDownload'
import type { DependencyName } from './types/dependencies'

export class AnonymiserManager {
  private readonly projectName = 'anonymiser'
  private readonly pythonEnv: PythonEnvironmentManager
  private readonly modelPath: string
  private childProcess: import('child_process').ChildProcess | null = null

  constructor(modelPath: string, pythonEnv: PythonEnvironmentManager) {
    this.pythonEnv = pythonEnv
    this.modelPath = modelPath
  }

  private async setupProjectFiles(): Promise<void> {
    log.info('[Anonymiser] Setting up anonymiser files')
    await this.pythonEnv.ensureProjectDirectory(this.projectName)

    const runtimeProjectDir = this.pythonEnv.getProjectDir(this.projectName)
    const stamp = path.join(runtimeProjectDir, '.anonymiser-files-copied')

    if (await fileExists(stamp)) {
      log.info('[Anonymiser] Anonymiser files already copied, skipping setup')
      return
    }

    const sourcePaths = [
      path.join(__dirname, 'python', 'anonymiser'),
      path.join(__dirname, '..', '..', '..', 'src', 'main', 'python', 'anonymiser'),
      path.join(process.cwd(), 'app', 'src', 'main', 'python', 'anonymiser')
    ]

    let sourceDir: string | null = null

    for (const sourcePath of sourcePaths) {
      try {
        if (await fileExists(sourcePath)) {
          sourceDir = sourcePath
          log.info(`[Anonymiser] Found anonymiser source at: ${sourcePath}`)
          break
        }
      } catch {
        // Continue to next path
      }
    }

    if (!sourceDir) {
      throw new Error('[Anonymiser] Failed to find anonymiser source files from any location')
    }

    await copyDirectoryRecursive(sourceDir, runtimeProjectDir)

    await fs.promises.writeFile(stamp, '')

    log.info('[Anonymiser] Anonymiser files created successfully')
  }

  async installPackages(): Promise<void> {
    log.info('[Anonymiser] Installing packages')
    log.info('[Anonymiser] Package installation completed')
  }

  async run(): Promise<void> {
    if (this.childProcess) {
      log.warn('[Anonymiser] Anonymiser is already running')
      return
    }

    log.info('[Anonymiser] Starting anonymiser service')
    log.info({ modelPath: this.modelPath })
    log.info({ uvPath: this.pythonEnv.getUvPath() })

    this.childProcess = spawn(this.pythonEnv.getUvPath(), ['run', 'anonymizer.py'], {
      cwd: this.pythonEnv.getProjectDir(this.projectName),
      env: {
        ...this.pythonEnv.getUvEnv(this.projectName),
        MODEL_PATH: this.modelPath
      }
      // stdio: 'pipe'
    })

    this.childProcess.stdout?.on('data', (data) => {
      const output = data.toString().trim()
      if (output) {
        log.info(`[Anonymiser] ${output}`)
      }
    })

    this.childProcess.stderr?.on('data', (data) => {
      const output = data.toString().trim()
      if (output) {
        log.error(`[Anonymiser] ${output}`)
      }
    })

    this.childProcess.on('exit', (code) => {
      log.info(`[Anonymiser] Service exited with code ${code}`)
      this.childProcess = null
    })

    log.info('[Anonymiser] Anonymiser service started successfully')
  }
}

export async function startAnonymiserSetup(): Promise<void> {
  try {
    log.info('[Anonymiser] Starting anonymiser setup')

    const dependencies = hasDependenciesDownloaded()

    if (!dependencies.anonymizer) {
      log.warn('[Anonymiser] Anonymizer model not yet downloaded, skipping setup')
      return
    }

    const modelPath = getDependencyPath('anonymizer' as DependencyName)
    log.info(`[Anonymiser] Using Anonymizer model from: ${modelPath}`)

    const pythonEnv = new PythonEnvironmentManager()
    const anonymiser = new AnonymiserManager(modelPath, pythonEnv)
    console.log(anonymiser)

    // await anonymiser.installPackages()
    await anonymiser.run()

    log.info('[Anonymiser] Anonymiser setup completed successfully')
  } catch (error) {
    log.error('[Anonymiser] Failed to setup anonymiser:', error)
  }
}
