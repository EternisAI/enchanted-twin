import path from 'node:path'
import fs from 'node:fs'
import { spawn } from 'node:child_process'
import log from 'electron-log/main'

import {
  AgentState,
  AgentStateUpdate,
  AgentCommand,
  LiveKitAgentCallbacks
} from './types/pythonManager.types'
import { PYTHON_REQUIREMENTS, SESSION_READY_INDICATORS } from './constants/pythonManager.constants'
import { InstallationError } from './errors/pythonManager.errors'
import { PythonEnvironmentManager } from './pythonEnvironmentManager'
import { DependencyProgress } from './types/pythonManager.types'

const LIVEKIT_PROGRESS_STEPS = {
  UV_SETUP: 10,
  VENV_SETUP: 30,
  AGENT_FILES: 50,
  DEPENDENCIES: 70,
  INITIALIZATION: 90,
  COMPLETE: 100
} as const

export class LiveKitAgentBootstrap {
  private readonly projectName = 'livekit-agent'
  private readonly pythonEnv: PythonEnvironmentManager
  private readonly projectDir: string
  private readonly greetingFile: string
  private readonly onboardingStateFile: string

  private onProgress?: (data: DependencyProgress) => void
  private onSessionReady?: (ready: boolean) => void
  private onStateChange?: (state: AgentState) => void
  private childProcess: import('child_process').ChildProcess | null = null
  private latestProgress: DependencyProgress

  constructor(callbacks: LiveKitAgentCallbacks = {}) {
    this.pythonEnv = new PythonEnvironmentManager()
    this.projectDir = this.pythonEnv.getProjectDir(this.projectName)
    this.greetingFile = path.join(this.projectDir, 'greeting.txt')
    this.onboardingStateFile = path.join(this.projectDir, 'onboarding_state.txt')

    this.onProgress = callbacks.onProgress
    this.onSessionReady = callbacks.onSessionReady
    this.onStateChange = callbacks.onStateChange

    this.latestProgress = {
      dependency: 'LiveKit Agent',
      progress: 0,
      status: 'Not started'
    }
  }

  private isVoiceDisabled(): boolean {
    return process.env.VITE_DISABLE_VOICE === 'true'
  }

  getLatestProgress(): DependencyProgress {
    return this.latestProgress
  }

  private updateProgress(progress: number, status: string, error?: string) {
    const progressData: DependencyProgress = {
      dependency: 'LiveKit Agent',
      progress,
      status,
      error
    }
    this.latestProgress = progressData
    this.onProgress?.(progressData)
  }

  getCurrentState(): AgentState {
    log.warn('[LiveKit] getCurrentState called but state is managed by Python agent')
    return 'idle' // Default fallback since state is managed by Python
  }

  sendCommand(command: AgentCommand): boolean {
    if (this.isVoiceDisabled()) {
      log.info('[LiveKit] Voice is disabled via VITE_DISABLE_VOICE, skipping command send')
      return false
    }

    if (!this.childProcess || !this.childProcess.stdin) {
      log.warn('[LiveKit] Cannot send command: agent process not running or stdin not available')
      return false
    }

    try {
      const commandStr = JSON.stringify(command) + '\n'
      this.childProcess.stdin.write(commandStr)
      log.info(`[LiveKit] Sent command: ${command.type}`)
      return true
    } catch (error) {
      log.error('[LiveKit] Failed to send command:', error)
      return false
    }
  }

  muteUser(): boolean {
    return this.sendCommand({ type: 'mute', timestamp: Date.now() })
  }

  unmuteUser(): boolean {
    return this.sendCommand({ type: 'unmute', timestamp: Date.now() })
  }

  private handleAgentOutput(data: string) {
    const lines = data.toString().trim().split('\n')
    for (const line of lines) {
      if (line.startsWith('STATE:')) {
        try {
          const stateData = JSON.parse(line.substring(6)) as AgentStateUpdate
          log.info(`[LiveKit] Agent state changed to: ${stateData.state}`)
          this.onStateChange?.(stateData.state)
        } catch (error) {
          log.error('[LiveKit] Failed to parse state update:', error)
        }
      } else if (line.trim()) {
        log.info(`[LiveKit] [agent] ${line}`)
        // Check for session ready indicators
        if (SESSION_READY_INDICATORS.some((indicator) => line.includes(indicator))) {
          this.onSessionReady?.(true)
        }
      }
    }
  }

  private async setupProjectFiles(): Promise<void> {
    log.info('[LiveKit] Setting up agent files')
    await this.pythonEnv.ensureProjectDirectory(this.projectName)

    const agentFile = path.join(this.projectDir, 'agent.py')

    // Copy agent file from embedded Python code
    const sourcePaths = [
      path.join(__dirname, 'python', 'livekit-agent.py'), // Built output path
      path.join(__dirname, '..', '..', '..', 'src', 'main', 'python', 'livekit-agent.py'), // Source path
      path.join(process.cwd(), 'app', 'src', 'main', 'python', 'livekit-agent.py') // Alternative source path
    ]

    let agentCode: string | null = null

    for (const sourcePath of sourcePaths) {
      try {
        agentCode = await fs.promises.readFile(sourcePath, 'utf8')
        log.info(`[LiveKit] Found agent source at: ${sourcePath}`)
        break
      } catch {
        // Continue to next path
      }
    }

    if (!agentCode) {
      log.error('[LiveKit] Failed to read agent source file from any location, using fallback')
      agentCode = await this.getFallbackAgentCode()
    }

    await fs.promises.writeFile(agentFile, agentCode)
    log.info('[LiveKit] Agent files created successfully')
  }

  private async getFallbackAgentCode(): Promise<string> {
    // Simple fallback - read from the external python file if possible
    try {
      const sourcePath = path.join(__dirname, 'python', 'livekit-agent.py')
      return await fs.promises.readFile(sourcePath, 'utf8')
    } catch {
      throw new InstallationError('Unable to locate agent Python source code', 'agent-files')
    }
  }

  private getAdditionalEnvironmentVariables(): Record<string, string> {
    return {
      TTS_URL: process.env.TTS_URL || '',
      TTS_MODEL: process.env.TTS_MODEL || '',
      STT_URL: process.env.STT_URL || '',
      STT_MODEL: process.env.STT_MODEL || '',
      SEND_MESSAGE_URL: `http://localhost:44999/query`,
      TERM: 'dumb', // Use dumb terminal to avoid TTY features
      PYTHONUNBUFFERED: '1', // Ensure immediate output
      NO_COLOR: '1', // Disable color codes
      LIVEKIT_DISABLE_TERMIOS: '1' // Custom flag to disable termios
    }
  }

  async setup(): Promise<void> {
    if (this.isVoiceDisabled()) {
      log.info('[LiveKit] Voice is disabled via VITE_DISABLE_VOICE, skipping LiveKit Agent setup')
      this.updateProgress(100, 'Voice disabled - skipped setup')
      return
    }

    log.info('[LiveKit] Starting LiveKit Agent setup process')
    try {
      this.updateProgress(LIVEKIT_PROGRESS_STEPS.UV_SETUP, 'Setting up Python environment')
      await this.pythonEnv.setupPythonEnvironment()

      this.updateProgress(LIVEKIT_PROGRESS_STEPS.VENV_SETUP, 'Setting up virtual environment')
      await this.pythonEnv.setupProjectVenv(this.projectName)

      this.updateProgress(LIVEKIT_PROGRESS_STEPS.AGENT_FILES, 'Setting up agent files')
      await this.setupProjectFiles()

      this.updateProgress(LIVEKIT_PROGRESS_STEPS.DEPENDENCIES, 'Installing dependencies')
      const dependencies = PYTHON_REQUIREMENTS.split('\n').filter((dep) => dep.trim())
      await this.pythonEnv.installDependencies(this.projectName, dependencies)

      log.info('[LiveKit] Ensuring clean slate before fake agent initialization')
      await this.stopAgent()

      this.updateProgress(LIVEKIT_PROGRESS_STEPS.INITIALIZATION, 'Initializing agent')
      await this.startAgent('SETUP_VALIDATION', false, true, undefined)

      log.info('[LiveKit] Waiting for first initialization test to complete...')
      await new Promise((resolve) => {
        const checkInterval = setInterval(() => {
          if (!this.childProcess) {
            log.info('[LiveKit] First initialization test completed and exited')
            clearInterval(checkInterval)
            resolve(undefined)
          }
        }, 500)

        setTimeout(() => {
          if (this.childProcess) {
            log.warn('[LiveKit] First initialization test did not exit after 10s, force stopping')
            this.stopAgent()
          }
          clearInterval(checkInterval)
          resolve(undefined)
        }, 10000)
      })

      this.updateProgress(LIVEKIT_PROGRESS_STEPS.COMPLETE, 'Ready')
      log.info('[LiveKit] LiveKit Agent setup completed successfully')
    } catch (e) {
      const error = e instanceof Error ? e.message : 'Unknown error occurred'
      log.error('[LiveKit] LiveKit Agent setup failed', e)
      this.updateProgress(this.latestProgress.progress, 'Failed', error)

      if (e instanceof InstallationError) {
        throw e
      }
      throw new InstallationError(error, 'livekit-setup')
    }
  }

  async startAgent(
    chatId: string,
    isOnboarding: boolean = false,
    isInitialising: boolean = false,
    jwtToken?: string
  ): Promise<void> {
    if (this.isVoiceDisabled()) {
      log.info('[LiveKit] Voice is disabled via VITE_DISABLE_VOICE, skipping agent start')
      return
    }

    if (this.childProcess) {
      log.warn('[LiveKit] Agent is already running, stopping it first...')
      await this.stopAgent()
    }

    log.info('[LiveKit] Starting LiveKit agent', isOnboarding)

    let greeting = ``
    if (isOnboarding) {
      greeting = `Hello there! Welcome to Enchanted, what is your name?`
    }

    await fs.promises.writeFile(this.greetingFile, greeting)
    await fs.promises.writeFile(this.onboardingStateFile, isOnboarding.toString())

    isOnboarding && console.log('isOnboarding starting', isOnboarding)

    const initialising = isInitialising ? 'true' : 'false'

    // Start the agent using the virtual environment Python
    this.childProcess = spawn(
      this.pythonEnv.getPythonBin(this.projectName),
      ['agent.py', 'console'],
      {
        cwd: this.projectDir,
        env: {
          ...process.env,
          ...this.pythonEnv.getUvEnv(this.projectName),
          ...this.getAdditionalEnvironmentVariables(),
          CHAT_ID: chatId,
          FIRST_INITIALIZE: initialising,
          FIREBASE_JWT_TOKEN: jwtToken || '' // Pass JWT token as environment variable
        },
        stdio: 'pipe' // Use pipe for logging
      }
    )

    this.childProcess.stdout?.on('data', (data) => {
      this.handleAgentOutput(data.toString())
    })

    this.childProcess.stderr?.on('data', (data) => {
      const output = data.toString().trim()
      if (output) {
        log.error(`[LiveKit] [agent] ${output}`)
      }
    })

    this.childProcess.on('exit', (code) => {
      log.info(`[LiveKit] Agent exited with code ${code}`)
      this.onSessionReady?.(false)
      this.childProcess = null
    })

    log.info('[LiveKit] Agent started successfully')
  }

  async stopAgent(): Promise<void> {
    if (this.isVoiceDisabled()) {
      log.info('[LiveKit] Voice is disabled via VITE_DISABLE_VOICE, skipping agent stop')
      return
    }

    await this.stopChildProcess()
    this.onSessionReady?.(false)

    // Clear the greeting and onboarding state files
    await fs.promises.writeFile(this.greetingFile, '')
    await fs.promises.writeFile(this.onboardingStateFile, 'false')
  }

  private async stopChildProcess(): Promise<void> {
    if (!this.childProcess) {
      log.warn('[LiveKit] No child process to stop')
      return
    }

    const pid = this.childProcess.pid
    log.info(`[LiveKit] Stopping child process with PID: ${pid}`)

    this.childProcess.kill('SIGTERM')

    const processExited = await new Promise<boolean>((resolve) => {
      const timeout = setTimeout(() => resolve(false), 2000) // 2 second timeout

      this.childProcess?.on('exit', () => {
        clearTimeout(timeout)
        resolve(true)
      })
    })

    if (!processExited && this.childProcess) {
      log.info(`[LiveKit] Process ${pid} didn't exit gracefully, force killing`)
      this.childProcess.kill('SIGKILL')

      await new Promise<void>((resolve) => {
        const timeout = setTimeout(resolve, 1000)
        this.childProcess?.on('exit', () => {
          clearTimeout(timeout)
          resolve()
        })
      })
    }

    this.childProcess = null
    log.info(`[LiveKit] Child process ${pid} cleanup completed`)
  }

  isAgentRunning(): boolean {
    if (this.isVoiceDisabled()) {
      return false
    }
    return this.childProcess !== null
  }

  async updateOnboardingState(isOnboarding: boolean): Promise<void> {
    if (this.isVoiceDisabled()) {
      log.info(
        '[LiveKit] Voice is disabled via VITE_DISABLE_VOICE, skipping onboarding state update'
      )
      return
    }

    await fs.promises.writeFile(this.onboardingStateFile, isOnboarding.toString())
    log.info(`[LiveKit] Updated onboarding state to: ${isOnboarding}`)
  }

  async cleanup(): Promise<void> {
    if (this.isVoiceDisabled()) {
      log.info('[LiveKit] Voice is disabled via VITE_DISABLE_VOICE, skipping cleanup')
      return
    }

    await this.stopAgent()
  }
}
