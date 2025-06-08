/**
 * LiveKit Voice Agent Worker - TypeScript Template
 * 
 * This TypeScript file provides the structure for a LiveKit voice agent worker.
 * Due to the current LiveKit agents framework v0.7.6 API design, this serves as
 * a template that should be compiled and run as a separate Node.js process.
 * 
 * To use this:
 * 1. Set environment variables: LIVEKIT_URL, LIVEKIT_API_KEY, LIVEKIT_API_SECRET, OPENAI_API_KEY
 * 2. Compile: npx tsc voice-agent-worker.ts --target es2020 --module commonjs
 * 3. Run: node voice-agent-worker.js dev
 */

// Type definitions that match the LiveKit agents framework
interface ChatMessage {
  role: 'system' | 'user' | 'assistant'
  content: string
}

interface ChatContext {
  messages: ChatMessage[]
}

interface AgentConfig {
  stt: {
    model: string
  }
  llm: {
    model: string
    temperature: number
  }
  tts: {
    model: string
    voice: string
  }
  chatCtx: ChatContext
  allowInterruptions: boolean
  interruptSpeechDuration: number
  interruptMinWords: number
  minEndpointingDelay: number
}

/**
 * Environment validation utility
 */
export function validateEnvironment(): boolean {
  const required = ['LIVEKIT_URL', 'LIVEKIT_API_KEY', 'LIVEKIT_API_SECRET', 'OPENAI_API_KEY']
  const missing = required.filter(key => !process.env[key])
  
  if (missing.length > 0) {
    console.error('\n❌ Missing required environment variables:')
    missing.forEach(key => console.log(`- ${key}`))
    console.log('\nPlease set the following environment variables:')
    console.log('- LIVEKIT_URL=wss://your-instance.livekit.cloud')
    console.log('- LIVEKIT_API_KEY=your-api-key')
    console.log('- LIVEKIT_API_SECRET=your-api-secret')
    console.log('- OPENAI_API_KEY=your-openai-api-key')
    return false
  }
  
  return true
}

/**
 * Create the agent configuration
 */
export function createAgentConfig(): AgentConfig {
  const initialCtx: ChatContext = {
    messages: [
      {
        role: 'system',
        content: 'You are a helpful AI assistant built with LiveKit. Keep your responses concise and conversational. Respond naturally as if you are having a real conversation with the user.'
      }
    ]
  }

  return {
    stt: {
      model: 'whisper-1'
    },
    llm: {
      model: 'gpt-4o-mini',
      temperature: 0.7
    },
    tts: {
      model: 'tts-1',
      voice: 'alloy'
    },
    chatCtx: initialCtx,
    allowInterruptions: true,
    interruptSpeechDuration: 0.6,
    interruptMinWords: 0,
    minEndpointingDelay: 0.5
  }
}

/**
 * Agent entrypoint function template
 * 
 * This function shows the structure for the actual LiveKit agent worker.
 * In a real implementation, this would use the LiveKit agents framework:
 * 
 * ```javascript
 * const { WorkerOptions, cli } = require('@livekit/agents')
 * const { VoicePipelineAgent } = require('@livekit/agents/pipeline')
 * const { STT, TTS, LLM } = require('@livekit/agents-plugin-openai')
 * const { VAD } = require('@livekit/agents-plugin-silero')
 * 
 * async function entrypoint(ctx) {
 *   await ctx.connect()
 *   const agent = new VoicePipelineAgent(config)
 *   agent.start(ctx.room)
 * }
 * 
 * cli.runApp(new WorkerOptions({ agent: entrypoint }))
 * ```
 */
export function getAgentEntrypointTemplate(): string {
  return `
// LiveKit Agent Worker Implementation
const { WorkerOptions, cli } = require('@livekit/agents')
const { VoicePipelineAgent } = require('@livekit/agents/pipeline')
const { STT, TTS, LLM } = require('@livekit/agents-plugin-openai')
const { VAD } = require('@livekit/agents-plugin-silero')

async function entrypoint(ctx) {
  console.log(\`🎤 Agent connecting to room: \${ctx.room.name}\`)
  
  await ctx.connect()

  const agent = new VoicePipelineAgent({
    stt: new STT({ model: 'whisper-1' }),
    llm: new LLM({ model: 'gpt-4o-mini', temperature: 0.7 }),
    tts: new TTS({ model: 'tts-1', voice: 'alloy' }),
    vad: VAD.load(),
    chatCtx: {
      messages: [{
        role: 'system',
        content: 'You are a helpful AI assistant built with LiveKit.'
      }]
    },
    allowInterruptions: true,
    interruptSpeechDuration: 0.6,
    interruptMinWords: 0,
    minEndpointingDelay: 0.5
  })

  agent.start(ctx.room)
  console.log('✅ Voice agent started and ready for conversations')
}

if (require.main === module) {
  cli.runApp(new WorkerOptions({ agent: entrypoint }))
}
`
}

/**
 * Main function for demonstrating the worker structure
 */
async function main(): Promise<void> {
  console.log('🚀 LiveKit Voice Agent Worker Template')
  console.log('=====================================\n')
  
  console.log('Environment check:')
  console.log(`- LIVEKIT_URL: ${process.env.LIVEKIT_URL ? '✅' : '❌ Missing'}`)
  console.log(`- LIVEKIT_API_KEY: ${process.env.LIVEKIT_API_KEY ? '✅' : '❌ Missing'}`)
  console.log(`- LIVEKIT_API_SECRET: ${process.env.LIVEKIT_API_SECRET ? '✅' : '❌ Missing'}`)
  console.log(`- OPENAI_API_KEY: ${process.env.OPENAI_API_KEY ? '✅' : '❌ Missing'}`)
  
  if (!validateEnvironment()) {
    console.log('\n📝 This is a template file. To create the actual worker:')
    console.log('1. Set the required environment variables')
    console.log('2. Create a separate .js file with the LiveKit agents imports')
    console.log('3. Use the template structure provided by getAgentEntrypointTemplate()')
    return
  }
  
  console.log('\n🎯 Environment configured successfully!')
  console.log('Agent configuration:')
  const config = createAgentConfig()
  console.log(JSON.stringify(config, null, 2))
  
  console.log('\n📋 Next steps:')
  console.log('1. Create a JavaScript worker file using the template')
  console.log('2. Install LiveKit agents dependencies')
  console.log('3. Run the worker with: node worker.js dev')
  
  console.log('\n📄 Worker template:')
  console.log(getAgentEntrypointTemplate())
}

// Export utilities for integration with the main app
export type { ChatMessage, ChatContext, AgentConfig }

// Run if this file is executed directly
if (require.main === module) {
  main().catch((error) => {
    console.error('❌ Error:', error)
    process.exit(1)
  })
} 