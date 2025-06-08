import { VoiceAgent, AgentConfig } from './livekit-agent'

// Global agent instance
let voiceAgent: VoiceAgent | null = null

// Example configuration - in a real app, these would come from environment variables or settings
const DEFAULT_AGENT_CONFIG: AgentConfig = {
  livekitUrl: process.env.LIVEKIT_URL || 'wss://your-livekit-url.livekit.cloud',
  livekitApiKey: process.env.LIVEKIT_API_KEY || '',
  livekitApiSecret: process.env.LIVEKIT_API_SECRET || '',
  openaiApiKey: process.env.OPENAI_API_KEY || ''
}

/**
 * Initialize and start the LiveKit voice agent
 * Call this during app startup
 */
export async function initializeAgent(config?: Partial<AgentConfig>): Promise<void> {
  try {
    const agentConfig = { ...DEFAULT_AGENT_CONFIG, ...config }
    
    console.log('🚀 Initializing LiveKit voice agent...')
    
    // Validate configuration
    if (!agentConfig.livekitUrl || !agentConfig.livekitApiKey || !agentConfig.livekitApiSecret || !agentConfig.openaiApiKey) {
      throw new Error('❌ Missing required configuration. Please set LIVEKIT_URL, LIVEKIT_API_KEY, LIVEKIT_API_SECRET, and OPENAI_API_KEY')
    }
    
    voiceAgent = new VoiceAgent(agentConfig)
    await voiceAgent.start()
    
    console.log('✅ LiveKit voice agent initialized successfully')
    
  } catch (error) {
    console.error('❌ Failed to initialize agent:', error)
    throw error
  }
}

/**
 * Shutdown the LiveKit voice agent
 * Call this during app shutdown
 */
export async function shutdownAgent(): Promise<void> {
  if (!voiceAgent) {
    console.log('No agent running to shutdown')
    return
  }
  
  try {
    console.log('🛑 Shutting down LiveKit voice agent...')
    await voiceAgent.stop()
    voiceAgent = null
    console.log('✅ Agent shutdown complete')
  } catch (error) {
    console.error('❌ Error shutting down agent:', error)
    throw error
  }
}

/**
 * Join a specific LiveKit room
 */
export async function joinRoom(roomName: string): Promise<void> {
  if (!voiceAgent) {
    throw new Error('Agent not initialized. Call initializeAgent() first.')
  }
  
  if (!voiceAgent.isReady()) {
    throw new Error('Agent is not ready. Please wait for agent to start.')
  }
  
  await voiceAgent.joinRoom(roomName)
}

/**
 * Leave the current LiveKit room
 */
export async function leaveRoom(): Promise<void> {
  if (!voiceAgent) {
    console.log('No agent running')
    return
  }
  
  await voiceAgent.leaveRoom()
}

/**
 * Get current agent status
 */
export function getAgentStatus(): { isRunning: boolean; pid?: number } {
  if (!voiceAgent) {
    return { isRunning: false }
  }
  
  return voiceAgent.getStatus()
}

/**
 * Check if agent is ready to use
 */
export function isAgentReady(): boolean {
  return voiceAgent ? voiceAgent.isReady() : false
}

/**
 * Get the current agent instance (for advanced usage)
 */
export function getAgent(): VoiceAgent | null {
  return voiceAgent
}

// Export the VoiceAgent class and types for direct use if needed
export { VoiceAgent } from './livekit-agent'
export type { AgentConfig } from './livekit-agent' 