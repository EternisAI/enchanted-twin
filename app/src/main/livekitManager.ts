import log from 'electron-log/main'
import { DependencyProgress, LiveKitAgentBootstrap } from './pythonManager'

let livekitAgent: LiveKitAgentBootstrap | null = null
let sessionReady = false

export function startLiveKitSetup(mainWindow: Electron.BrowserWindow) {
  const agentProgress = (data: DependencyProgress) => {
    if (mainWindow) {
      log.info(`[LiveKit] Emitting launch-progress: ${data.progress}, Status: ${data.status}`)
      mainWindow.webContents.send('launch-progress', data)
    }
  }

  const agentSessionReady = (ready: boolean) => {
    sessionReady = ready
    if (mainWindow) {
      log.info(`[LiveKit] Session ready state changed: ${ready}`)
      mainWindow.webContents.send('livekit-session-state', { sessionReady: ready })
    }
  }

  livekitAgent = new LiveKitAgentBootstrap(agentProgress, agentSessionReady)

  try {
    livekitAgent.setup()
  } catch (error) {
    log.error('Failed to setup LiveKit agent environment:', error)
  }

  return livekitAgent
}

export async function setupLiveKitAgent(): Promise<void> {
  if (!livekitAgent) {
    throw new Error('LiveKit agent not initialized')
  }

  const currentState = livekitAgent.getLatestProgress()
  if (currentState.status === 'Ready' || currentState.progress === 100) {
    log.info('LiveKit agent already setup, skipping')
    return
  }

  try {
    await livekitAgent.setup()
  } catch (error) {
    log.error('Failed to setup LiveKit agent:', error)
    throw error
  }
}

export async function startLiveKitAgent(): Promise<void> {
  if (!livekitAgent) {
    throw new Error('LiveKit agent not initialized')
  }

  try {
    sessionReady = false // Reset session state when starting
    await livekitAgent.startAgent()
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
