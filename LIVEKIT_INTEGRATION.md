# LiveKit Agent Integration

This document explains how to use the integrated LiveKit voice agent in your Electron app.

## Overview

The LiveKit agent integration allows you to add voice conversation capabilities to your app. When a user starts a voice conversation, the agent will:

1. Connect to a LiveKit room
2. Listen for voice input using Voice Activity Detection (VAD)
3. Convert speech to text using OpenAI's STT
4. Process the text using GPT-4o-mini
5. Convert the response back to speech using OpenAI's TTS
6. Play the response to the user

## Setup

The LiveKit agent requires the following environment variable:

```bash
OPENAI_API_KEY=your_openai_api_key
```

## Usage from Renderer Process

### 1. Agent Dependencies (Automatic)

The LiveKit agent dependencies (Python environment, packages, etc.) are automatically installed when the app launches, similar to other system dependencies. You can monitor this progress using the launch progress listener.

If you need to manually trigger setup (e.g., after an error), you can call:

```typescript
// Manual setup if needed
const setupResult = await window.api.livekit.setup()
if (!setupResult.success) {
  console.error('Failed to setup LiveKit agent:', setupResult.error)
}
```

### 2. Start a Voice Conversation

```typescript
// Start the agent - room connection is handled by LiveKit framework
const startResult = await window.api.livekit.start()
if (startResult.success) {
  console.log('Voice agent started successfully')
} else {
  console.error('Failed to start agent:', startResult.error)
}
```

### 3. Stop the Conversation

```typescript
// Stop the agent when conversation is over
const stopResult = await window.api.livekit.stop()
if (stopResult.success) {
  console.log('Voice agent stopped')
}
```

### 4. Check Agent Status

```typescript
// Check if agent is currently running
const isRunning = await window.api.livekit.isRunning()
console.log('Agent running:', isRunning)

// Get detailed agent state
const state = await window.api.livekit.getState()
console.log('Agent state:', state)
// Returns: { dependency: 'LiveKit Agent', progress: 100, status: 'Ready' }
```

## Integration Example

Here's a complete example of how to integrate voice conversations:

```typescript
class VoiceConversationManager {
  async ensureReady() {
    // Check if dependencies are ready
    const state = await window.api.livekit.getState()
    if (state.status !== 'Ready' && state.progress !== 100) {
      throw new Error(`LiveKit agent not ready: ${state.status}`)
    }
  }
  
  async startVoiceChat() {
    await this.ensureReady()
    
    const result = await window.api.livekit.start()
    if (!result.success) {
      throw new Error(`Failed to start voice chat: ${result.error}`)
    }
    
    console.log('Voice chat started - user can now speak')
  }
  
  async endVoiceChat() {
    const result = await window.api.livekit.stop()
    if (!result.success) {
      console.warn('Failed to stop voice chat:', result.error)
    }
    
    console.log('Voice chat ended')
  }
  
  async getStatus() {
    return {
      isRunning: await window.api.livekit.isRunning(),
      state: await window.api.livekit.getState()
    }
  }
}

// Usage
const voiceManager = new VoiceConversationManager()

// Start a conversation (dependencies are automatically installed at app launch)
await voiceManager.startVoiceChat()

// Later, end the conversation
await voiceManager.endVoiceChat()
```

## Room Management

The LiveKit agent uses the LiveKit framework's built-in room management. The agent will:

1. Start with `python agent.py console` command
2. Connect to rooms automatically through the LiveKit framework
3. Handle room joining and participant management internally

If you need specific room management, you'll need to configure this through:
- Your LiveKit server configuration
- Environment variables for the LiveKit framework
- Or modify the agent code to handle specific room joining logic

Example room token generation (server-side):

```javascript
// Server-side code (Node.js) - requires LIVEKIT_API_KEY and LIVEKIT_API_SECRET
const { AccessToken } = require('livekit-server-sdk')

function generateRoomToken(roomName, participantName) {
  const token = new AccessToken(
    process.env.LIVEKIT_API_KEY,    // Your server's LiveKit API key
    process.env.LIVEKIT_API_SECRET, // Your server's LiveKit API secret
    {
      identity: participantName,
    }
  )
  
  token.addGrant({
    room: roomName,
    roomJoin: true,
    canPublish: true,
    canSubscribe: true,
  })
  
  return token.toJwt()
}
```

## Monitoring Progress

The agent setup process emits progress events that you can listen to:

```typescript
// Listen for setup progress
const removeListener = window.api.launch.onProgress((data) => {
  if (data.dependency === 'LiveKit Agent') {
    console.log(`Setup progress: ${data.progress}% - ${data.status}`)
    
    if (data.error) {
      console.error('Setup error:', data.error)
    }
  }
})

// Don't forget to remove the listener when done
removeListener()
```

## Customization

### Custom Prompt

You can customize the agent's behavior by placing a `meta_prompt2.txt` file in your app directory. The agent will automatically load and use this prompt instead of the default one.

### Environment Variables

The agent supports this environment variable:

- `OPENAI_API_KEY` - Required for STT, LLM, and TTS

## Error Handling

All API methods return result objects with `success` boolean and optional `error` message:

```typescript
const result = await window.api.livekit.start()
if (!result.success) {
  // Handle error
  console.error('Failed to start:', result.error)
  // Show user-friendly error message
}
```

## Lifecycle Management

The agent is automatically cleaned up when the app exits. The main process handles:

- Stopping the Python process
- Cleaning up resources
- Terminating connections

You should stil manually stop the agent when conversations end to free up resources. 