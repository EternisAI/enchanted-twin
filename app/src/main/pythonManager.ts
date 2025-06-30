import { app } from 'electron'
import path from 'node:path'
import fs, { constants as fsc } from 'node:fs'
import { spawn } from 'node:child_process'
import log from 'electron-log/main'

import {
  DependencyProgress,
  AgentState,
  AgentStateUpdate,
  AgentCommand,
  RunOptions,
  LiveKitAgentCallbacks
} from './types/pythonManager.types'
import {
  PYTHON_VERSION,
  UV_INSTALL_SCRIPT,
  REQUIRED_ENV_VARS,
  PYTHON_REQUIREMENTS,
  PROGRESS_STEPS,
  SESSION_READY_INDICATORS,
  GRACEFUL_SHUTDOWN_TIMEOUT_MS
} from './constants/pythonManager.constants'
import {
  PythonManagerError,
  EnvironmentError,
  InstallationError,
  AgentError
} from './errors/pythonManager.errors'

// Re-export types for external use
export type {
  DependencyProgress,
  AgentState,
  AgentStateUpdate,
  AgentCommand,
  LiveKitAgentCallbacks
}
export { PythonManagerError, EnvironmentError, InstallationError, AgentError }

/* ─────────────────────────────────────────────────────────────────────────── */

export class LiveKitAgentBootstrap {
  private readonly USER_DIR = app.getPath('userData')
  private readonly USER_BIN = path.join(this.USER_DIR, 'bin')
  private readonly UV_PATH = path.join(
    this.USER_BIN,
    process.platform === 'win32' ? 'uv.exe' : 'uv'
  )

  private readonly LIVEKIT_DIR = path.join(this.USER_DIR, 'dependencies', 'livekit-agent')
  private readonly VENV_DIR = path.join(this.LIVEKIT_DIR, '.venv')
  private readonly greetingFile = path.join(this.LIVEKIT_DIR, 'greeting.txt')
  private readonly onboardingStateFile = path.join(this.LIVEKIT_DIR, 'onboarding_state.txt')

  /** absolute path to the python executable in the venv */
  private pythonBin(): string {
    const sub =
      process.platform === 'win32' ? path.join('Scripts', 'python.exe') : path.join('bin', 'python')
    return path.join(this.VENV_DIR, sub)
  }

  private agentProc: import('child_process').ChildProcess | null = null
  private onProgress?: (data: DependencyProgress) => void
  private onSessionReady?: (ready: boolean) => void
  private onStateChange?: (state: AgentState) => void
  private latestProgress: DependencyProgress = {
    dependency: 'LiveKit Agent',
    progress: 0,
    status: 'Not started'
  }

  constructor(callbacks: LiveKitAgentCallbacks = {}) {
    this.onProgress = callbacks.onProgress
    this.onSessionReady = callbacks.onSessionReady
    this.onStateChange = callbacks.onStateChange
  }

  getLatestProgress() {
    return this.latestProgress
  }

  getCurrentState(): AgentState {
    log.warn('[LiveKit] getCurrentState called but state is managed by Python agent')
    return 'idle' // Default fallback since state is managed by Python
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

  sendCommand(command: AgentCommand) {
    if (!this.agentProc || !this.agentProc.stdin) {
      log.warn('[LiveKit] Cannot send command: agent process not running or stdin not available')
      return false
    }

    try {
      const commandStr = JSON.stringify(command) + '\n'
      this.agentProc.stdin.write(commandStr)
      log.info(`[LiveKit] Sent command: ${command.type}`)
      return true
    } catch (error) {
      log.error('[LiveKit] Failed to send command:', error)
      return false
    }
  }

  muteUser() {
    return this.sendCommand({ type: 'mute', timestamp: Date.now() })
  }

  unmuteUser() {
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
      log.info(`[LiveKit] [${opts.label}] → ${cmd} ${args.join(' ')}`)
      const p = spawn(cmd, args, { ...opts, stdio: 'pipe' })

      p.stdout?.on('data', (data) => {
        const output = data.toString().trim()
        if (output) {
          log.info(`[LiveKit] [${opts.label}] ${output}`)
        }
      })

      p.stderr?.on('data', (data) => {
        const output = data.toString().trim()
        if (output) {
          log.error(`[LiveKit] [${opts.label}] ${output}`)
        }
      })

      p.once('error', reject)
      p.once('exit', (c) => (c === 0 ? resolve() : reject(new Error(`${opts.label} exit ${c}`))))
    })
  }

  /* ── install steps ──────────────────────────────────────────────────────── */
  private async ensureUv() {
    if (await this.exists(this.UV_PATH, fsc.X_OK)) {
      log.info('[LiveKit] UV already installed')
      return
    }
    log.info('[LiveKit] Installing UV package manager')
    await fs.promises.mkdir(this.USER_BIN, { recursive: true })
    await this.run('sh', ['-c', UV_INSTALL_SCRIPT], {
      label: 'uv-install',
      env: { ...process.env, UV_INSTALL_DIR: this.USER_BIN }
    })
  }

  private async ensurePython312() {
    await this.run(this.UV_PATH, ['python', 'install', PYTHON_VERSION, '--quiet'], {
      label: 'py312'
    })
  }

  private async ensureAgentFiles() {
    log.info('[LiveKit] Setting up agent files')
    await fs.promises.mkdir(this.LIVEKIT_DIR, { recursive: true })

    const agentFile = path.join(this.LIVEKIT_DIR, 'agent.py')
    const requirementsFile = path.join(this.LIVEKIT_DIR, 'requirements.txt')

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

    await fs.promises.writeFile(requirementsFile, PYTHON_REQUIREMENTS)
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

  private async ensureVenv() {
    if (await this.exists(this.VENV_DIR)) {
      log.info('[LiveKit] Virtual environment already exists')
      return
    }

    log.info(`[LiveKit] Creating virtual environment with Python ${PYTHON_VERSION}`)

    await this.run(this.UV_PATH, ['venv', '--python', PYTHON_VERSION, this.VENV_DIR], {
      label: 'uv-venv'
    })
  }

  private async ensureDeps() {
    const stamp = path.join(this.VENV_DIR, '.livekit-installed')
    if (await this.exists(stamp)) {
      log.info('[LiveKit] Dependencies already installed')
      return
    }

    log.info('[LiveKit] Installing Python dependencies using uv pip install')
    await this.run(this.UV_PATH, ['pip', 'install', '-r', 'requirements.txt'], {
      cwd: this.LIVEKIT_DIR,
      env: this.uvEnv(this.VENV_DIR),
      label: 'uv-pip'
    })

    await fs.promises.writeFile(stamp, '')
    log.info('[LiveKit] Dependencies installation completed')
  }

  async setup() {
    log.info('[LiveKit] Starting LiveKit Agent setup process')
    try {
      this.updateProgress(PROGRESS_STEPS.UV_SETUP, 'Setting up dependency manager')
      await this.ensureUv()

      this.updateProgress(PROGRESS_STEPS.PYTHON_INSTALL, 'Installing Python')
      await this.ensurePython312()

      this.updateProgress(PROGRESS_STEPS.AGENT_FILES, 'Setting up agent files')
      await this.ensureAgentFiles()

      this.updateProgress(PROGRESS_STEPS.VENV_CREATION, 'Creating virtual environment')
      await this.ensureVenv()

      this.updateProgress(PROGRESS_STEPS.DEPENDENCIES, 'Installing dependencies')
      await this.ensureDeps()

      this.updateProgress(PROGRESS_STEPS.COMPLETE, 'Ready')

      log.info('[LiveKit] LiveKit Agent setup completed successfully')
    } catch (e) {
      const error = e instanceof Error ? e.message : 'Unknown error occurred'
      log.error('[LiveKit] LiveKit Agent setup failed', e)
      this.updateProgress(this.latestProgress.progress, 'Failed', error)

      if (e instanceof InstallationError) {
        throw e
      }
      throw new InstallationError(error, 'setup')
    }
  }

  async startAgent(chatId: string, isOnboarding: boolean = false, isInitialising: boolean = false) {
    if (this.agentProc) {
      log.warn('[LiveKit] Agent is already running')
      return
    }

    log.info('[LiveKit] Starting LiveKit agent', isOnboarding)

    // Note: Room connection is handled by the LiveKit agent framework via ctx.connect()

    // Check for required environment variables before starting
    const missingEnvVars = REQUIRED_ENV_VARS.filter((envVar) => !process.env[envVar])
    if (missingEnvVars.length > 0) {
      throw new EnvironmentError(missingEnvVars)
    }

    let greeting = ``
    if (isOnboarding) {
      greeting = `Hello there! Welcome to Enchanted, what is your name?`
    }

    await fs.promises.writeFile(this.greetingFile, greeting)
    await fs.promises.writeFile(this.onboardingStateFile, isOnboarding.toString())

    isOnboarding && console.log('isOnboarding starting', isOnboarding)

    const initialising = isInitialising ? 'true' : 'false'

    // Start the agent using the virtual environment Python
    this.agentProc = spawn(this.pythonBin(), ['agent.py', 'console'], {
      cwd: this.LIVEKIT_DIR,
      env: {
        ...process.env,
        CHAT_ID: chatId,
        FAKE_INIT: initialising,
        TINFOIL_API_KEY: process.env.TINFOIL_API_KEY,
        TINFOIL_AUDIO_URL: process.env.TINFOIL_AUDIO_URL,
        TINFOIL_STT_MODEL: process.env.TINFOIL_STT_MODEL,
        TINFOIL_TTS_MODEL: process.env.TINFOIL_TTS_MODEL,
        SEND_MESSAGE_URL: `http://localhost:44999/query`,
        TERM: 'dumb', // Use dumb terminal to avoid TTY features
        PYTHONUNBUFFERED: '1', // Ensure immediate output
        NO_COLOR: '1', // Disable color codes
        LIVEKIT_DISABLE_TERMIOS: '1' // Custom flag to disable termios
      },
      stdio: 'pipe' // Use pipe for logging
    })

    this.agentProc.stdout?.on('data', (data) => {
      this.handleAgentOutput(data.toString())
    })

    this.agentProc.stderr?.on('data', (data) => {
      const output = data.toString().trim()
      if (output) {
        log.error(`[LiveKit] [agent] ${output}`)
      }
    })

    this.agentProc.on('exit', (code) => {
      log.info(`[LiveKit] Agent exited with code ${code}`)
      this.onSessionReady?.(false)
      this.agentProc = null
    })

    log.info('[LiveKit] Agent started successfully')
  }

  async stopAgent() {
    if (!this.agentProc) {
      log.warn('[LiveKit] No agent process to stop')
      return
    }

    log.info('[LiveKit] Stopping LiveKit agent')
    this.onSessionReady?.(false)
    this.agentProc.kill('SIGTERM')

    // Give it a moment to exit gracefully
    await new Promise((resolve) => setTimeout(resolve, GRACEFUL_SHUTDOWN_TIMEOUT_MS))

    if (this.agentProc) {
      log.info('[LiveKit] Force killing agent process')
      this.agentProc.kill('SIGKILL')
    }

    this.agentProc = null
    log.info('[LiveKit] Agent stopped')

    // Clear the greeting and onboarding state files
    await fs.promises.writeFile(this.greetingFile, '')
    await fs.promises.writeFile(this.onboardingStateFile, 'false')
  }

  isAgentRunning(): boolean {
    return this.agentProc !== null
  }

  async updateOnboardingState(isOnboarding: boolean): Promise<void> {
    await fs.promises.writeFile(this.onboardingStateFile, isOnboarding.toString())
    log.info(`[LiveKit] Updated onboarding state to: ${isOnboarding}`)
  }

  async cleanup() {
    await this.stopAgent()
  }
}
