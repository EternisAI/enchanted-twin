import log from 'electron-log/main'
import { AgentState, DependencyProgress, LiveKitAgentBootstrap } from './pythonManager'

let livekitAgent: LiveKitAgentBootstrap | null = null
let sessionReady = false

export async function startLiveKitSetup(mainWindow: Electron.BrowserWindow) {
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

  livekitAgent = new LiveKitAgentBootstrap({
    onProgress: agentProgress,
    onSessionReady: agentSessionReady,
    onStateChange: agentStateChange
  })

  try {
    await livekitAgent.setup()
  } catch (error) {
    log.error('Failed to setup LiveKit agent environment:', error)
  }

  return livekitAgent
}

export async function startLiveKitAgent(
  chatId: string,
  isOnboarding = false,
  isInitialising = false,
  jwtToken?: string
): Promise<void> {
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
  return livekitAgent?.isAgentRunning() ?? false
}

export function isLiveKitSessionReady(): boolean {
  return sessionReady && isLiveKitAgentRunning()
}

export async function getLiveKitAgentState(): Promise<DependencyProgress> {
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
  if (livekitAgent) {
    log.info('Cleaning up LiveKit agent...')
    await livekitAgent.cleanup()
    livekitAgent = null
  }
}

export function muteLiveKitAgent(): boolean {
  if (!livekitAgent) {
    log.warn('LiveKit agent not initialized')
    return false
  }
  return livekitAgent.muteUser()
}

export function unmuteLiveKitAgent(): boolean {
  if (!livekitAgent) {
    log.warn('LiveKit agent not initialized')
    return false
  }
  return livekitAgent.unmuteUser()
}

export function getCurrentAgentState(): AgentState {
  if (!livekitAgent) {
    return 'idle'
  }
  return livekitAgent.getCurrentState()
}
