import { spawn, ChildProcess } from 'child_process'
import { join } from 'path'
import { app } from 'electron'

export interface AgentConfig {
  livekitUrl: string
  livekitApiKey: string
  livekitApiSecret: string
  openaiApiKey: string
  roomName?: string
}

export class VoiceAgent {
  private config: AgentConfig
  private isRunning = false
  private agentProcess: ChildProcess | null = null
  private workerScriptPath: string

  constructor(config: AgentConfig) {
    this.config = config
    // Path to the worker script relative to the main process
    this.workerScriptPath = join(__dirname, 'voice-agent-worker.js')
  }

  async start(): Promise<void> {
    if (this.isRunning) {
      console.log('Agent is already running')
      return
    }

    try {
      console.log('Starting LiveKit voice agent process...')

      // Prepare environment variables for the child process
      const env = {
        ...process.env,
        LIVEKIT_URL: this.config.livekitUrl,
        LIVEKIT_API_KEY: this.config.livekitApiKey,
        LIVEKIT_API_SECRET: this.config.livekitApiSecret,
        OPENAI_API_KEY: this.config.openaiApiKey,
        ROOM_NAME: this.config.roomName || 'default-room'
      }

      // Spawn the agent worker process
      this.agentProcess = spawn('node', [this.workerScriptPath, 'dev'], {
        env,
        stdio: ['pipe', 'pipe', 'pipe'],
        cwd: app.getAppPath()
      })

      // Handle process events
      this.setupProcessHandlers()

      this.isRunning = true
      console.log(`✅ LiveKit agent process started with PID: ${this.agentProcess.pid}`)

    } catch (error) {
      console.error('❌ Failed to start LiveKit agent:', error)
      this.isRunning = false
      throw error
    }
  }

  async stop(): Promise<void> {
    if (!this.isRunning || !this.agentProcess) {
      console.log('Agent is not running')
      return
    }

    try {
      console.log('Stopping LiveKit voice agent...')

      // Gracefully terminate the process
      this.agentProcess.kill('SIGTERM')

      // Wait a bit for graceful shutdown
      await new Promise(resolve => setTimeout(resolve, 2000))

      // Force kill if still running
      if (this.agentProcess && !this.agentProcess.killed) {
        console.log('Force killing agent process...')
        this.agentProcess.kill('SIGKILL')
      }

      this.agentProcess = null
      this.isRunning = false
      console.log('✅ LiveKit agent stopped')

    } catch (error) {
      console.error('❌ Error stopping agent:', error)
      throw error
    }
  }

  async joinRoom(roomName: string): Promise<void> {
    if (!this.isRunning) {
      throw new Error('Agent is not running. Start the agent first.')
    }

    console.log(`📞 Agent joining room: ${roomName}`)
    // The agent worker will handle room joining based on environment variables
    // You could also implement IPC communication here if needed
  }

  async leaveRoom(): Promise<void> {
    if (!this.isRunning) {
      console.log('Agent is not running')
      return
    }

    console.log('📴 Agent leaving room...')
    // The agent worker handles room management
  }

  getStatus(): { isRunning: boolean; pid?: number } {
    return {
      isRunning: this.isRunning,
      pid: this.agentProcess?.pid
    }
  }

  isReady(): boolean {
    return this.isRunning && this.agentProcess !== null && !this.agentProcess.killed
  }

  private setupProcessHandlers(): void {
    if (!this.agentProcess) return

    // Handle process output
    this.agentProcess.stdout?.on('data', (data) => {
      console.log(`[Agent] ${data.toString().trim()}`)
    })

    this.agentProcess.stderr?.on('data', (data) => {
      console.error(`[Agent Error] ${data.toString().trim()}`)
    })

    // Handle process exit
    this.agentProcess.on('exit', (code, signal) => {
      console.log(`Agent process exited with code ${code} and signal ${signal}`)
      this.isRunning = false
      this.agentProcess = null
    })

    // Handle process errors
    this.agentProcess.on('error', (error) => {
      console.error('Agent process error:', error)
      this.isRunning = false
      this.agentProcess = null
    })

    // Clean up on app exit
    process.on('exit', () => {
      if (this.agentProcess && !this.agentProcess.killed) {
        this.agentProcess.kill('SIGKILL')
      }
    })
  }
}

// Utility function to start agent with default config
export async function startAgent(config: AgentConfig): Promise<VoiceAgent> {
  const agent = new VoiceAgent(config)
  await agent.start()
  return agent
}

// Export convenience functions for easy usage
export async function createVoiceAgent(config: AgentConfig): Promise<VoiceAgent> {
  return new VoiceAgent(config)
} 