import { spawn } from 'node:child_process'
import log from 'electron-log/main'

import { PythonEnvironmentManager } from './pythonEnvironmentManager'

export class AnonymiserManager {
  private readonly projectName = 'anonymiser'
  private readonly pythonEnv: PythonEnvironmentManager
  private readonly modelPath: string
  private readonly projectDir: string
  private childProcess: import('child_process').ChildProcess | null = null

  constructor(modelPath: string, projectDir: string, pythonEnv: PythonEnvironmentManager) {
    this.pythonEnv = pythonEnv
    this.modelPath = modelPath
    this.projectDir = projectDir
  }

  private async setupProjectFiles(): Promise<void> {
    log.info('[Anonymiser] Setting up anonymiser files')
    await this.pythonEnv.ensureProjectDirectory(this.projectName)

    log.info('[Anonymiser] Anonymiser files created successfully')
  }

  async installPackages(): Promise<void> {
    log.info('[Anonymiser] Installing packages')

    await this.setupProjectFiles()

    const dependencies = [
      'transformers>=4.30.0',
      'torch>=2.0.0',
      'spacy>=3.6.0',
      'presidio-analyzer>=2.2.0',
      'presidio-anonymizer>=2.2.0',
      'datasets>=2.14.0',
      'accelerate>=0.21.0'
    ]

    await this.pythonEnv.installDependencies(this.projectName, dependencies)
    log.info('[Anonymiser] Package installation completed')
  }

  async run(): Promise<void> {
    if (this.childProcess) {
      log.warn('[Anonymiser] Anonymiser is already running')
      return
    }

    log.info('[Anonymiser] Starting anonymiser service')

    this.childProcess = spawn(
      this.pythonEnv.getPythonBin(this.projectName),
      ['anonymiser.py', '--model_path', this.modelPath],
      {
        cwd: this.projectDir,
        env: {
          ...process.env
        },
        stdio: 'pipe'
      }
    )

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
