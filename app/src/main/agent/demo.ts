/**
 * Demo script showing how to integrate LiveKit voice agent with app lifecycle
 * 
 * This demonstrates the integration pattern you requested:
 * - Agent starts when app starts
 * - Agent stops when app stops
 * - Functions to join/leave rooms
 */

import { initializeAgent, shutdownAgent, joinRoom, leaveRoom, getAgentStatus, isAgentReady } from './index'

// Example configuration (in a real app, these would come from your app's configuration)
const DEMO_CONFIG = {
  livekitUrl: 'wss://your-livekit-instance.livekit.cloud',
  livekitApiKey: 'your-api-key',
  livekitApiSecret: 'your-api-secret',
  openaiApiKey: 'your-openai-api-key'
}

/**
 * Simulate app startup - initialize the agent
 */
async function simulateAppStartup(): Promise<void> {
  console.log('🚀 App starting up...')
  
  try {
    // Initialize the agent with your configuration
    await initializeAgent(DEMO_CONFIG)
    
    console.log('✅ App startup complete - Agent ready!')
    
    // Check status after startup
    const status = getAgentStatus()
    console.log('📊 Agent Status:', status)
    console.log('🔍 Agent Ready:', isAgentReady())
    
  } catch (error) {
    console.error('❌ Failed to start app:', error)
  }
}

/**
 * Simulate joining a room (like when user clicks join button)
 */
async function simulateJoinRoom(): Promise<void> {
  console.log('👆 User clicked join room button...')
  
  try {
    if (!isAgentReady()) {
      console.log('⏳ Agent not ready yet, waiting...')
      // In a real app, you might want to show a loading state
      return
    }
    
    await joinRoom('demo-room-123')
    console.log('✅ Successfully joined room!')
    
  } catch (error) {
    console.error('❌ Failed to join room:', error)
  }
}

/**
 * Simulate leaving a room (like when user clicks leave button)
 */
async function simulateLeaveRoom(): Promise<void> {
  console.log('👆 User clicked leave room button...')
  
  try {
    await leaveRoom()
    console.log('✅ Successfully left room!')
    
  } catch (error) {
    console.error('❌ Failed to leave room:', error)
  }
}

/**
 * Simulate app shutdown - cleanup the agent
 */
async function simulateAppShutdown(): Promise<void> {
  console.log('🛑 App shutting down...')
  
  try {
    await shutdownAgent()
    console.log('✅ App shutdown complete!')
    
  } catch (error) {
    console.error('❌ Error during shutdown:', error)
  }
}

/**
 * Run the full demo sequence
 */
async function runDemo(): Promise<void> {
  console.log('🎬 Starting LiveKit Agent Demo...')
  console.log('=====================================')
  
  try {
    // 1. Simulate app startup
    await simulateAppStartup()
    
    // Wait a bit to let the agent fully initialize
    console.log('⏳ Waiting for agent to be ready...')
    await new Promise(resolve => setTimeout(resolve, 3000))
    
    // 2. Simulate user joining a room
    await simulateJoinRoom()
    
    // Keep the agent running for a bit
    console.log('🎤 Agent is running and ready for voice conversations...')
    console.log('💬 In a real app, users can now talk to the AI agent!')
    await new Promise(resolve => setTimeout(resolve, 5000))
    
    // 3. Simulate user leaving the room
    await simulateLeaveRoom()
    
    // Wait a bit
    await new Promise(resolve => setTimeout(resolve, 2000))
    
    // 4. Simulate app shutdown
    await simulateAppShutdown()
    
    console.log('🎬 Demo completed successfully!')
    
  } catch (error) {
    console.error('❌ Demo failed:', error)
    // Make sure to cleanup even if demo fails
    try {
      await simulateAppShutdown()
    } catch (shutdownError) {
      console.error('❌ Failed to cleanup after demo error:', shutdownError)
    }
  }
}

/**
 * Export functions for use in your main app
 */
export {
  simulateAppStartup,
  simulateJoinRoom,
  simulateLeaveRoom,
  simulateAppShutdown,
  runDemo
}

// Auto-run demo if this file is executed directly
if (require.main === module) {
  runDemo().catch(console.error)
} 