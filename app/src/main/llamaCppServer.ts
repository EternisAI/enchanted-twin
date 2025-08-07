import { spawn } from 'node:child_process'
import log from 'electron-log/main'
import path from 'node:path'
import fs from 'node:fs'

import { getDependencyPath, hasDependenciesDownloaded } from './dependenciesDownload'

export class LlamaCppServerManager {
  private childProcess: import('child_process').ChildProcess | null = null
  private readonly model4bPath: string
  private readonly model06bPath: string | null
  private readonly port: number

  constructor(model4bPath: string, model06bPath: string | null = null, port: number = 11435) {
    this.model4bPath = model4bPath
    this.model06bPath = model06bPath
    this.port = port
  }

  async run(): Promise<void> {
    if (this.childProcess) {
      log.warn('[LlamaCpp] Llama server is already running')
      return
    }

    log.info('[LlamaCpp] Starting llama-server service')
    log.info({
      model4bPath: this.model4bPath,
      model06bPath: this.model06bPath,
      port: this.port
    })

    const llamaServerPath = this.findLlamaServerExecutable()
    if (!llamaServerPath) {
      throw new Error('[LlamaCpp] llama-server executable not found')
    }

    const args = [
      '-m',
      this.model4bPath,
      '--flash-attn',
      '--ctx-size',
      '8192',
      '--cache-type-k',
      'q4_0',
      '--cache-type-v',
      'q4_0',
      '-ngl',
      '99',
      '-t',
      '-1',
      '-b',
      '2048',
      '--mlock',
      '--port',
      this.port.toString(),
      '--jinja'
    ]

    // Add 0.6b model arguments if 0.6b model is available
    if (this.model06bPath) {
      args.push('-md', this.model06bPath)
      args.push('-ngld', '99')
      args.push('--draft-max', '12')
      args.push('--draft-min', '1')
      args.push('--draft-p-min', '0.9')
    }

    this.childProcess = spawn(llamaServerPath, args, {
      cwd: path.dirname(llamaServerPath),
      stdio: 'pipe'
    })

    this.childProcess.stdout?.on('data', (data) => {
      const output = data.toString().trim()
      if (output) {
        log.info(`[LlamaCpp] ${output}`)
      }
    })

    this.childProcess.stderr?.on('data', (data) => {
      const output = data.toString().trim()
      if (output) {
        log.error(`[LlamaCpp] ${output}`)
      }
    })

    this.childProcess.on('exit', (code) => {
      log.info(`[LlamaCpp] Service exited with code ${code}`)
      this.childProcess = null
    })

    this.childProcess.on('error', (error) => {
      log.error(`[LlamaCpp] Service error:`, error)
      this.childProcess = null
    })

    log.info('[LlamaCpp] Llama server service started successfully')
  }

  private findLlamaServerExecutable(): string | null {
    const llamaCppPath = getDependencyPath('llamaccp')
    const possibleExecutables = [
      path.join(llamaCppPath, 'llama-server'),
      path.join(llamaCppPath, 'llama-server.exe'),
      path.join(llamaCppPath, 'bin', 'llama-server'),
      path.join(llamaCppPath, 'build', 'bin', 'llama-server')
    ]

    for (const execPath of possibleExecutables) {
      try {
        if (fs.existsSync(execPath)) {
          log.info(`[LlamaCpp] Found llama-server at: ${execPath}`)
          return execPath
        }
      } catch {
        // Continue to next path
      }
    }

    log.error(
      `[LlamaCpp] llama-server executable not found in any of the expected paths:`,
      possibleExecutables
    )
    return null
  }

  isRunning(): boolean {
    return this.childProcess !== null && !this.childProcess.killed
  }

  async cleanup(): Promise<void> {
    if (this.childProcess) {
      log.info('[LlamaCpp] Stopping llama server service')
      this.childProcess.kill()
      this.childProcess = null
    }
  }
}

let llamaCppInstance: LlamaCppServerManager | null = null
let setupCompleted = false
let setupInProgress = false

export async function startLlamaCppSetup(): Promise<void> {
  if (process.env.ANONYMIZER_TYPE === 'no-op') {
    log.info('[LlamaCpp] Skipping LlamaCpp setup - ANONYMIZER_TYPE is no-op')
    return
  }

  if (llamaCppInstance && setupCompleted && llamaCppInstance.isRunning()) {
    log.info('[LlamaCpp] Llama server already set up and running, skipping setup')
    return
  }

  if (setupInProgress) {
    log.info('[LlamaCpp] Llama server setup already in progress, skipping')
    return
  }

  try {
    setupInProgress = true

    const dependencies = hasDependenciesDownloaded()

    if (!dependencies.llamaccp) {
      log.warn('[LlamaCpp] LLAMACCP binaries not yet downloaded, skipping setup')
      return
    }

    if (!dependencies.anonymizer) {
      log.warn('[LlamaCpp] Anonymizer model not yet downloaded, skipping setup')
      return
    }

    const modelDir = getDependencyPath('anonymizer')
    const { model4b, model06b } = findModelFiles(modelDir)

    if (!model4b) {
      log.warn('[LlamaCpp] No 4b GGUF model file found in anonymizer directory, skipping setup')
      return
    }

    if (!llamaCppInstance) {
      llamaCppInstance = new LlamaCppServerManager(model4b, model06b, 11435)
    }

    await llamaCppInstance.run()
    setupCompleted = true

    log.info('[LlamaCpp] Llama server setup completed successfully')
  } catch (error) {
    log.error('[LlamaCpp] Failed to setup llama server:', error)
    setupCompleted = false
  } finally {
    setupInProgress = false
  }
}

export async function cleanupLlamaCpp(): Promise<void> {
  if (llamaCppInstance) {
    await llamaCppInstance.cleanup()
    llamaCppInstance = null
  }
  setupCompleted = false
  setupInProgress = false
}

export function getLlamaCppStatus(): {
  isRunning: boolean
  isSetup: boolean
  setupInProgress: boolean
} {
  return {
    isRunning: llamaCppInstance?.isRunning() ?? false,
    isSetup: setupCompleted,
    setupInProgress
  }
}

function findModelFiles(modelDir: string): { model4b: string | null; model06b: string | null } {
  try {
    const files = fs.readdirSync(modelDir)
    const ggufs = files.filter((file) => file.endsWith('.gguf'))

    if (ggufs.length === 0) {
      log.warn(`[LlamaCpp] No GGUF files found in ${modelDir}`)
      return { model4b: null, model06b: null }
    }

    const model4b = ggufs.find(
      (file) =>
        file.toLowerCase().includes('qwen') &&
        (file.toLowerCase().includes('4b') || file.toLowerCase().includes('4-b'))
    )

    const model06b = ggufs.find(
      (file) =>
        file.toLowerCase().includes('qwen') &&
        (file.toLowerCase().includes('0.6b') || file.toLowerCase().includes('0.6-b'))
    )

    const model4bPath = model4b ? path.join(modelDir, model4b) : null
    const model06bPath = model06b ? path.join(modelDir, model06b) : null

    log.info(`[LlamaCpp] Found 4b model: ${model4bPath}`)
    log.info(`[LlamaCpp] Found 0.6b model: ${model06bPath}`)

    return { model4b: model4bPath, model06b: model06bPath }
  } catch (error) {
    log.error(`[LlamaCpp] Error searching for model files in ${modelDir}:`, error)
    return { model4b: null, model06b: null }
  }
}
