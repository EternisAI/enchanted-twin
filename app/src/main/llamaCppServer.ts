import { spawn } from 'node:child_process'
import log from 'electron-log/main'
import path from 'node:path'
import fs from 'node:fs'

import { getDependencyPath, hasDependenciesDownloaded } from './dependenciesDownload'

function findModelFile(modelDir: string): string | null {
  try {
    //@TODO: Confirm if this is needed
    const files = fs.readdirSync(modelDir)

    const ggufs = files.filter((file) => file.endsWith('.gguf'))

    if (ggufs.length === 0) {
      log.warn(`[LlamaCpp] No GGUF files found in ${modelDir}`)
      return null
    }

    const qwenModel = ggufs.find((file) => file.toLowerCase().includes('qwen'))
    const selectedModel = qwenModel || ggufs[0]

    const modelPath = path.join(modelDir, selectedModel)
    log.info(`[LlamaCpp] Found model file: ${modelPath}`)
    return modelPath
  } catch (error) {
    log.error(`[LlamaCpp] Error searching for model file in ${modelDir}:`, error)
    return null
  }
}

export class LlamaCppServerManager {
  private childProcess: import('child_process').ChildProcess | null = null
  private readonly modelPath: string
  private readonly port: number

  constructor(modelPath: string, port: number = 8000) {
    this.modelPath = modelPath
    this.port = port
  }

  async run(): Promise<void> {
    if (this.childProcess) {
      log.warn('[LlamaCpp] Llama server is already running')
      return
    }

    log.info('[LlamaCpp] Starting llama-server service')
    log.info({ modelPath: this.modelPath, port: this.port })

    const llamaServerPath = this.findLlamaServerExecutable()
    if (!llamaServerPath) {
      throw new Error('[LlamaCpp] llama-server executable not found')
    }

    const args = [
      '-m',
      this.modelPath,
      '--flash-attn',
      '--ctx-size',
      '8192',
      '--cache-type-k',
      'q8_0',
      '--cache-type-v',
      'q8_0',
      '-ngl',
      '99',
      '-t',
      '-1',
      '-b',
      '2048',
      '--mlock',
      '--metrics',
      '--port',
      this.port.toString()
    ]

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
    const llamaCppPath = getDependencyPath('LLAMACCP')
    const possibleExecutables = [
      path.join(llamaCppPath, 'build', 'bin', 'llama-server'),
      path.join(llamaCppPath, 'llama-server.exe') // Windows
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

    // Check if we have the required dependencies
    if (!dependencies.LLAMACCP) {
      log.warn('[LlamaCpp] LLAMACCP binaries not yet downloaded, skipping setup')
      return
    }

    if (!dependencies.anonymizer) {
      log.warn('[LlamaCpp] Anonymizer model not yet downloaded, skipping setup')
      return
    }

    const modelDir = getDependencyPath('anonymizer')
    // Look for GGUF model file in the anonymizer directory
    const modelPath = findModelFile(modelDir)

    if (!modelPath) {
      log.warn('[LlamaCpp] No GGUF model file found in anonymizer directory, skipping setup')
      return
    }

    // Create instance only if it doesn't exist
    if (!llamaCppInstance) {
      llamaCppInstance = new LlamaCppServerManager(modelPath, 8000)
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
