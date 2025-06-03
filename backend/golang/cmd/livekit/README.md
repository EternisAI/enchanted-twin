# LiveKit Voice Agent in Go

A voice AI agent implementation using LiveKit with STT (Speech-to-Text), LLM (Large Language Model), and TTS (Text-to-Speech) capabilities.

## Features

- **Speech-to-Text**: Uses Deepgram API for real-time speech recognition
- **Large Language Model**: Uses OpenAI GPT for intelligent responses
- **Text-to-Speech**: Uses Cartesia API for natural voice synthesis
- **Voice Activity Detection**: Simple energy-based VAD implementation
- **Real-time Audio Processing**: Handles audio streams through LiveKit WebRTC

## Architecture

The agent consists of several key components:

- `DeepgramSTT`: Handles speech-to-text conversion using Deepgram's API
- `OpenAILLM`: Processes natural language using OpenAI's GPT models
- `CartesiaTTS`: Converts text responses back to speech using Cartesia
- `SimpleVAD`: Detects voice activity in audio streams
- `AgentSession`: Orchestrates the entire conversation flow

## Prerequisites

1. **LiveKit Server**: Either use [LiveKit Cloud](https://livekit.io/cloud) or self-host
2. **API Keys**: You'll need API keys for:
   - OpenAI (for LLM)
   - Deepgram (for STT)
   - Cartesia (for TTS)

## Environment Variables

Create a `.env` file in the project root with the following variables:

```env
# LiveKit Configuration
LIVEKIT_URL=wss://your-livekit-server.com
LIVEKIT_API_KEY=your-api-key
LIVEKIT_API_SECRET=your-api-secret
LIVEKIT_ROOM_NAME=voice-agent-room

# OpenAI Configuration
OPENAI_API_KEY=your-openai-api-key

# Deepgram Configuration (for STT)
DEEPGRAM_API_KEY=your-deepgram-api-key

# Cartesia Configuration (for TTS)
CARTESIA_API_KEY=your-cartesia-api-key
```

## Installation

1. **Install Dependencies**:

   ```bash
   cd backend/golang
   go mod tidy
   ```

2. **Build the Agent**:
   ```bash
   go build -o livekit-agent cmd/livekit/main.go
   ```

## Usage

### Running the Agent

1. **Set up your environment variables** as described above

2. **Run the agent**:

   ```bash
   ./livekit-agent
   ```

   Or directly with Go:

   ```bash
   go run cmd/livekit/main.go
   ```

3. **Connect a client** to the same LiveKit room to start interacting with the agent

### Client Integration

Use any LiveKit client SDK to connect to the same room:

- **Web**: [LiveKit JavaScript SDK](https://github.com/livekit/client-sdk-js)
- **Mobile**: [iOS](https://github.com/livekit/client-sdk-swift) / [Android](https://github.com/livekit/client-sdk-android)
- **Desktop**: [Electron](https://github.com/livekit/client-sdk-js)

## How It Works

1. **Audio Capture**: The agent subscribes to audio tracks from participants in the room
2. **Voice Activity Detection**: Uses energy-based VAD to detect when speech is occurring
3. **Speech-to-Text**: Sends audio chunks to Deepgram for transcription
4. **LLM Processing**: Processes the transcript with OpenAI GPT to generate a response
5. **Text-to-Speech**: Converts the response to audio using Cartesia
6. **Audio Playback**: Publishes the synthesized audio back to the room

## Configuration

### Voice Activity Detection

The current implementation uses a simple energy-based VAD. You can adjust the sensitivity by modifying the `threshold` value in the `SimpleVAD.DetectSpeech` method.

### Audio Processing

- Audio is buffered for 2 seconds or until 32KB of data is accumulated
- RTP packets are processed in real-time from WebRTC audio tracks
- Audio format expected: 16-bit PCM, various sample rates supported

### API Models

- **STT**: Deepgram Nova-3 model with multilingual support
- **LLM**: OpenAI GPT-4o-mini for fast responses
- **TTS**: Cartesia Sonic model for natural voice synthesis

## Extending the Agent

### Adding New STT Providers

Implement the STT interface:

```go
type STTProvider interface {
    Transcribe(ctx context.Context, audio []byte) (string, error)
}
```

### Adding New LLM Providers

Implement the LLM interface:

```go
type LLMProvider interface {
    Generate(ctx context.Context, messages []Message) (string, error)
}
```

### Adding New TTS Providers

Implement the TTS interface:

```go
type TTSProvider interface {
    Synthesize(ctx context.Context, text string) ([]byte, error)
}
```

## Troubleshooting

### Common Issues

1. **Connection Errors**: Verify your LiveKit URL and credentials
2. **Audio Issues**: Check that audio tracks are being published from clients
3. **API Errors**: Ensure all API keys are valid and have sufficient credits
4. **Build Errors**: Run `go mod tidy` to ensure all dependencies are installed

### Logging

The agent provides detailed logging for:

- Connection events
- Audio processing
- STT/LLM/TTS API calls
- Error conditions

## Performance Considerations

- **Latency**: Current implementation prioritizes accuracy over speed
- **Concurrent Processing**: Single-threaded processing to avoid race conditions
- **Memory Usage**: Audio buffering uses limited memory (32KB buffers)
- **API Rate Limits**: No built-in rate limiting; relies on API provider limits

## Future Improvements

- [ ] Audio track publishing for TTS responses
- [ ] Advanced VAD using ML models
- [ ] Support for multiple simultaneous conversations
- [ ] Conversation memory and context management
- [ ] Streaming audio processing for lower latency
- [ ] Enhanced error handling and retry logic
- [ ] Metrics and monitoring integration

## License

This project follows the main repository's license terms.
