import log from 'electron-log/main'
import { AgentState, DependencyProgress } from './types/pythonManager.types'
import { LiveKitAgentBootstrap } from './livekitAgent'
import { exec } from 'child_process'

let livekitAgent: LiveKitAgentBootstrap | null = null
let sessionReady = false
let setupCompleted = false

function isVoiceDisabled(): boolean {
  return process.env.VITE_DISABLE_VOICE === 'true'
}

function cleanupOrphanedAgents(): Promise<void> {
  return new Promise((resolve) => {
    log.info('[LiveKit] Cleaning up any orphaned agents from previous sessions...')

    const commands =
      process.platform === 'win32'
        ? [
            'taskkill /F /FI "COMMANDLINE eq *livekit-agent*"',
            'taskkill /F /FI "COMMANDLINE eq *agent.py console*"'
          ]
        : ['pkill -f "python.*agent.py console"', 'pkill -f "livekit-agent"']

    let completed = 0
    const totalCommands = commands.length

    commands.forEach((command) => {
      exec(command, (error) => {
        completed++

        if (error && error.code !== 1) {
          log.debug(`[LiveKit] Cleanup command "${command}" result:`, error.message)
        }

        if (completed === totalCommands) {
          log.info('[LiveKit] Orphaned agent cleanup completed')
          setTimeout(resolve, 500)
        }
      })
    })
  })
}

export async function startLiveKitSetup(mainWindow: Electron.BrowserWindow) {
  if (isVoiceDisabled()) {
    log.info('[LiveKit] Voice is disabled via VITE_DISABLE_VOICE, skipping LiveKit setup')
    return null
  }

  await cleanupOrphanedAgents()

  // Check if LiveKit is already set up
  if (livekitAgent && setupCompleted) {
    log.info('LiveKit agent already set up, skipping initialization')
    return livekitAgent
  }

  const agentProgress = (data: DependencyProgress) => {
    if (mainWindow && !mainWindow.isDestroyed()) {
      log.info(`[LiveKit] Emitting launch-progress: ${data.progress}, Status: ${data.status}`)
      mainWindow.webContents.send('launch-progress', data)
    }
  }

  const agentSessionReady = (ready: boolean) => {
    sessionReady = ready
    if (mainWindow && !mainWindow.isDestroyed()) {
      log.info(`[LiveKit] Session ready state changed: ${ready}`)
      mainWindow.webContents.send('livekit-session-state', { sessionReady: ready })
    }
  }

  const agentStateChange = (state: AgentState) => {
    if (mainWindow && !mainWindow.isDestroyed()) {
      log.info(`[LiveKit] Agent state changed: ${state}`)
      mainWindow.webContents.send('livekit-agent-state', { state })
    }
  }

  if (!livekitAgent) {
    livekitAgent = new LiveKitAgentBootstrap({
      onProgress: agentProgress,
      onSessionReady: agentSessionReady,
      onStateChange: agentStateChange
    })
  }

  try {
    // NOTE: This assumes the Python environment (UV, Python, venv) is already set up
    // by the application initialization process. The LiveKit agent only handles
    // its own files and dependencies.
    await livekitAgent.setup()
    setupCompleted = true
    log.info('LiveKit agent setup completed successfully')
  } catch (error) {
    log.error('Failed to setup LiveKit agent environment:', error)
    setupCompleted = false
  }

  return livekitAgent
}

export async function startLiveKitAgent(
  chatId: string,
  isOnboarding = false,
  isInitialising = false,
  jwtToken?: string
): Promise<void> {
  if (isVoiceDisabled()) {
    log.info('[LiveKit] Voice is disabled via VITE_DISABLE_VOICE, skipping agent start')
    return
  }

  log.info('Starting LiveKit agent. Is initialising: ' + isInitialising)

  if (!livekitAgent) {
    throw new Error('LiveKit agent not initialized')
  }

  try {
    sessionReady = false // Reset session state when starting
    await livekitAgent.startAgent(chatId, isOnboarding, isInitialising, jwtToken)
    log.info('LiveKit agent started successfully')
  } catch (error) {
    log.error('Failed to start LiveKit agent:', error)
    throw error
  }
}

export async function stopLiveKitAgent(): Promise<void> {
  if (isVoiceDisabled()) {
    log.info('[LiveKit] Voice is disabled via VITE_DISABLE_VOICE, skipping agent stop')
    return
  }

  if (!livekitAgent) {
    log.warn('LiveKit agent not initialized')
    return
  }

  try {
    sessionReady = false // Reset session state when stopping
    await livekitAgent.stopAgent()
    log.info('LiveKit agent stopped successfully')
  } catch (error) {
    log.error('Failed to stop LiveKit agent:', error)
    throw error
  }
}

export function isLiveKitAgentRunning(): boolean {
  if (isVoiceDisabled()) {
    return false
  }
  return livekitAgent?.isAgentRunning() ?? false
}

export function isLiveKitSessionReady(): boolean {
  if (isVoiceDisabled()) {
    return false
  }
  return sessionReady && isLiveKitAgentRunning()
}

export async function getLiveKitAgentState(): Promise<DependencyProgress> {
  if (isVoiceDisabled()) {
    return {
      dependency: 'LiveKit Agent',
      progress: 100,
      status: 'Voice disabled'
    }
  }

  if (!livekitAgent) {
    return {
      dependency: 'LiveKit Agent',
      progress: 0,
      status: 'Not initialized'
    }
  }
  return livekitAgent.getLatestProgress()
}

export async function cleanupLiveKitAgent() {
  if (isVoiceDisabled()) {
    log.info('[LiveKit] Voice is disabled via VITE_DISABLE_VOICE, skipping cleanup')
    return
  }

  if (livekitAgent) {
    log.info('Cleaning up LiveKit agent...')
    await livekitAgent.cleanup()
    livekitAgent = null
  }

  setupCompleted = false
}

export function muteLiveKitAgent(): boolean {
  if (isVoiceDisabled()) {
    log.info('[LiveKit] Voice is disabled via VITE_DISABLE_VOICE, skipping mute')
    return false
  }

  if (!livekitAgent) {
    log.warn('LiveKit agent not initialized')
    return false
  }
  return livekitAgent.muteUser()
}

export function unmuteLiveKitAgent(): boolean {
  if (isVoiceDisabled()) {
    log.info('[LiveKit] Voice is disabled via VITE_DISABLE_VOICE, skipping unmute')
    return false
  }

  if (!livekitAgent) {
    log.warn('LiveKit agent not initialized')
    return false
  }
  return livekitAgent.unmuteUser()
}

export function getCurrentAgentState(): AgentState {
  if (isVoiceDisabled()) {
    return 'idle'
  }

  if (!livekitAgent) {
    return 'idle'
  }
  return livekitAgent.getCurrentState()
}
