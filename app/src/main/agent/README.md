# LiveKit Voice Agent Integration

This directory contains the LiveKit voice agent integration for the Electron app. The implementation provides start/stop functionality for a voice AI agent that can connect to LiveKit rooms and handle voice conversations using OpenAI's STT and TTS services.

## Overview

The integration consists of several files:

- `livekit-agent.ts` - Core VoiceAgent class for managing agent lifecycle
- `index.ts` - Main integration module with convenience functions
- `demo.ts` - Demonstration script showing usage patterns
- `voice-agent-worker.ts` - TypeScript template for LiveKit agent worker
- `README.md` - This documentation file

## Architecture

The implementation follows the LiveKit agents framework pattern:

1. **VoiceAgent Class**: Manages configuration and environment setup
2. **Agent Worker Process**: Separate Node.js process running the actual LiveKit agent
3. **Integration Functions**: Convenience functions for app lifecycle integration

## Setup

### 1. Environment Variables

Set the following environment variables:

```bash
# LiveKit Configuration
LIVEKIT_URL=wss://your-instance.livekit.cloud
LIVEKIT_API_KEY=your-api-key
LIVEKIT_API_SECRET=your-api-secret

# OpenAI Configuration
OPENAI_API_KEY=your-openai-api-key
```

### 2. Dependencies

The LiveKit agents framework dependencies are already included in `package.json`:

```json
{
  "@livekit/agents": "^0.7.6",
  "@livekit/agents-plugin-openai": "^0.9.1",
  "@livekit/agents-plugin-silero": "^0.5.6"
}
```

## Usage

### Basic Integration

```typescript
import { initializeAgent, shutdownAgent, joinRoom, leaveRoom } from './agent'

// Initialize agent during app startup
async function onAppReady() {
  await initializeAgent({
    livekitUrl: 'wss://your-instance.livekit.cloud',
    livekitApiKey: 'your-api-key',
    livekitApiSecret: 'your-api-secret',
    openaiApiKey: 'your-openai-api-key'
  })
}

// Shutdown agent during app shutdown
async function onAppShutdown() {
  await shutdownAgent()
}

// Join a room (user action)
async function handleJoinRoom(roomName: string) {
  await joinRoom(roomName)
}

// Leave room (user action)
async function handleLeaveRoom() {
  await leaveRoom()
}
```

### Integration with Electron Main Process

```typescript
import { app } from 'electron'
import { initializeAgent, shutdownAgent } from './agent'

app.whenReady().then(async () => {
  // Initialize the voice agent
  await initializeAgent()
  
  // ... rest of your app initialization
})

app.on('before-quit', async () => {
  // Shutdown the voice agent
  await shutdownAgent()
})
```

## Current Implementation Status

### ✅ Completed Features

- Agent configuration management
- Environment variable setup
- Start/stop lifecycle functions
- Room join/leave configuration
- Status monitoring
- Integration examples

### 🚧 Next Steps for Full Implementation

To complete the implementation, you need to:

1. **Create Agent Worker Script**: Use `voice-agent-worker.ts` as a template to create your actual agent worker

2. **Process Management**: Implement actual process spawning to run the agent worker as a separate Node.js process

3. **Room Dispatch**: Set up proper room dispatching so the agent joins specific rooms

4. **Error Handling**: Add robust error handling and recovery

5. **Logging**: Integrate with your app's logging system

## TypeScript Agent Worker Template

The `voice-agent-worker.ts` file provides a TypeScript template for creating a LiveKit agent worker:

```javascript
const { WorkerOptions, cli } = require('@livekit/agents')
const { VoicePipelineAgent } = require('@livekit/agents/pipeline')
const { STT, TTS, LLM } = require('@livekit/agents-plugin-openai')
const { VAD } = require('@livekit/agents-plugin-silero')

async function entrypoint(ctx) {
  await ctx.connect()
  
  const agent = new VoicePipelineAgent({
    stt: new STT({ model: 'whisper-1' }),
    llm: new LLM({ model: 'gpt-4o-mini', temperature: 0.7 }),
    tts: new TTS({ model: 'tts-1', voice: 'alloy' }),
    vad: VAD.load(),
    // ... configuration options
  })
  
  agent.start(ctx.room)
}

cli.runApp(new WorkerOptions({ agent: entrypoint }))
```

## Testing

Run the demo to test the integration:

```typescript
import { runDemo } from './agent/demo'

// Run the demonstration
await runDemo()
```

This will simulate:
- App startup (agent initialization)
- User actions (join/leave rooms)
- App shutdown (agent cleanup)

## Configuration Options

### AgentConfig Interface

```typescript
interface AgentConfig {
  livekitUrl: string       // LiveKit server URL
  livekitApiKey: string    // LiveKit API key
  livekitApiSecret: string // LiveKit API secret
  openaiApiKey: string     // OpenAI API key
  roomName?: string        // Optional room name
}
```

### Agent Features

The voice agent supports:
- **OpenAI Whisper** for speech-to-text
- **OpenAI GPT-4o-mini** for language understanding
- **OpenAI TTS** for text-to-speech
- **Silero VAD** for voice activity detection
- **Interruption handling** for natural conversations
- **Configurable voice settings**

## LiveKit Cloud Setup

1. Create a LiveKit Cloud account at [livekit.io](https://livekit.io)
2. Create a new project
3. Get your API keys from the project settings
4. Configure the WebSocket URL (usually `wss://your-project.livekit.cloud`)

## Troubleshooting

### Common Issues

1. **Missing Dependencies**: Make sure all LiveKit packages are installed
2. **Environment Variables**: Verify all required environment variables are set
3. **API Keys**: Check that your LiveKit and OpenAI API keys are valid
4. **Network**: Ensure your app can connect to LiveKit servers

### Debug Logging

Enable debug logging by setting:
```bash
DEBUG=livekit:*
```

## Additional Resources

- [LiveKit Agents Documentation](https://docs.livekit.io/agents)
- [LiveKit Agents JS GitHub](https://github.com/livekit/agents-js)
- [Voice Pipeline Agent Examples](https://github.com/livekit-examples/voice-pipeline-agent-node)
- [OpenAI API Documentation](https://platform.openai.com/docs)

## Next Steps

1. Set up your LiveKit Cloud instance
2. Configure your API keys
3. Create and test your agent worker script
4. Integrate the agent functions into your Electron app's main process
5. Add UI controls for starting/stopping the agent and joining/leaving rooms 