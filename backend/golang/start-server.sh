#!/bin/bash

# Enchanted Twin Backend Server Startup Script
# This script cleans up ports and starts the server with PostgreSQL backend

echo "🚀 Starting Enchanted Twin Backend Server..."

# Kill processes on required ports
echo "🧹 Cleaning up ports..."
lsof -ti:5432 | xargs kill -9 2>/dev/null || true   # PostgreSQL
lsof -ti:44999 | xargs kill -9 2>/dev/null || true  # GraphQL
lsof -ti:4222 | xargs kill -9 2>/dev/null || true   # NATS
lsof -ti:7233 | xargs kill -9 2>/dev/null || true   # Temporal
lsof -ti:8233 | xargs kill -9 2>/dev/null || true   # Temporal UI
lsof -ti:45001 | xargs kill -9 2>/dev/null || true  # TTS

echo "✅ Ports cleaned up"

# Wait a moment for processes to fully terminate
sleep 2

# Set environment variables and start server
echo "🔧 Starting server with PostgreSQL backend and local embeddings..."

nohup env \
  COMPLETIONS_API_URL=https://openrouter.ai/api/v1 \
  COMPLETIONS_MODEL=openai/gpt-4.1 \
  REASONING_MODEL=openai/o3 \
  EMBEDDINGS_API_URL=https://api.openai.com/v1 \
  EMBEDDINGS_MODEL=text-embedding-3-small \
  IS_PROD_BUILD=true \
  OLLAMA_BASE_URL=https://enchanted.ngrok.pro \
  TELEGRAM_CHAT_SERVER=https://enchanted-proxy-telegram-dev.up.railway.app/query \
  ENCHANTED_MCP_URL=https://proxy-api-dev.ep-use1.ghostagent.org/mcp \
  TTS_MODEL=kokoro \
  TTS_URL=https://inference.tinfoil.sh/v1/ \
  STT_MODEL=whisper-large-v3-turbo \
  STT_URL=https://inference.tinfoil.sh/v1/ \
  PROXY_TEE_URL=https://proxy-api-dev.ep-use1.ghostagent.org \
  HOLON_API_URL=http://23.22.67.228:8123 \
  ANONYMIZER_TYPE=no-op \
  USE_LOCAL_EMBEDDINGS=true \
  TTS_ENDPOINT=https://inference.tinfoil.sh/v1/audio/speech \
  BUILD_CHANNEL=dev \
  MEMORY_BACKEND=postgresql \
  POSTGRES_DATA_PATH=./postgres-data \
  GRAPHQL_PORT=44999 \
  APP_DATA_PATH="/Users/innokentii/Library/Application Support/enchanted" \
  make run > backend.log 2>&1 &

echo "🚀 Server started in background!"
echo "📊 Monitor logs with: tail -f backend.log"
echo "🌐 GraphQL server will be available at: http://localhost:44999"
echo "⏱️  Temporal UI will be available at: http://127.0.0.1:8233"

# Wait a moment and check if server started successfully
sleep 3
if pgrep -f "go run cmd/server/main.go" > /dev/null; then
    echo "✅ Server is running successfully!"
else
    echo "❌ Server failed to start. Check backend.log for details:"
    tail -20 backend.log
fi