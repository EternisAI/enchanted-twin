#!/bin/bash
set -e

echo "🐳 Starting Docker E2E Test Environment..."
echo "========================================"

# Function to log with timestamp
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

# Function to handle cleanup
cleanup() {
    log "🧹 Cleaning up..."
    pkill -f "xvfb" || true
    exit 0
}

# Set up signal handlers
trap cleanup SIGTERM SIGINT

# Start virtual display for headless Electron
log "🖥️  Starting virtual display..."
Xvfb :99 -screen 0 1024x768x24 > /dev/null 2>&1 &
XVFB_PID=$!

# Wait a moment for Xvfb to start
sleep 2

# Verify virtual display is running
if ! ps -p $XVFB_PID > /dev/null; then
    log "❌ Failed to start virtual display"
    exit 1
fi

log "✅ Virtual display started (PID: $XVFB_PID)"

# Wait for backend to be ready
log "⏳ Waiting for backend to be ready..."
BACKEND_HOST=${BACKEND_HOST:-backend}
BACKEND_PORT=${BACKEND_PORT:-44999}
MAX_ATTEMPTS=60
ATTEMPT=0

while [ $ATTEMPT -lt $MAX_ATTEMPTS ]; do
    if curl -f "http://$BACKEND_HOST:$BACKEND_PORT/query" \
        -X POST \
        -H "Content-Type: application/json" \
        -d '{"query": "{ __typename }"}' \
        --connect-timeout 5 \
        --silent > /dev/null 2>&1; then
        log "✅ Backend is ready!"
        break
    fi
    
    ATTEMPT=$((ATTEMPT + 1))
    log "⏳ Waiting for backend... ($ATTEMPT/$MAX_ATTEMPTS)"
    sleep 2
done

if [ $ATTEMPT -eq $MAX_ATTEMPTS ]; then
    log "❌ Backend failed to become ready within $(($MAX_ATTEMPTS * 2)) seconds"
    exit 1
fi

# Set up environment variables for testing
export BACKEND_GRAPHQL_URL="http://$BACKEND_HOST:$BACKEND_PORT/query"
export BACKEND_WEAVIATE_URL="http://$BACKEND_HOST:51415"

# Create test environment file
cat > /app/.env << EOF
E2E_TEST_EMAIL=${E2E_TEST_EMAIL}
E2E_TEST_PASSWORD=${E2E_TEST_PASSWORD}
VITE_FIREBASE_API_KEY=${VITE_FIREBASE_API_KEY}
VITE_FIREBASE_AUTH_DOMAIN=${VITE_FIREBASE_AUTH_DOMAIN}
VITE_FIREBASE_PROJECT_ID=${VITE_FIREBASE_PROJECT_ID}
COMPLETIONS_API_KEY=${COMPLETIONS_API_KEY}
OPENROUTER_API_KEY=${OPENROUTER_API_KEY}
OPENAI_API_KEY=${OPENAI_API_KEY}
EMBEDDINGS_API_KEY=${EMBEDDINGS_API_KEY}
EOF

log "🔧 Environment configured:"
log "   - Backend URL: $BACKEND_GRAPHQL_URL"
log "   - Virtual Display: $DISPLAY"
log "   - Test Mode: $NODE_ENV"

# Run the e2e tests with verbose output
log "🧪 Starting E2E tests..."
echo "========================================"

# Determine test command based on arguments
if [ $# -eq 0 ]; then
    # Default: run master e2e test
    TEST_CMD="pnpm test:e2e:master"
else
    # Use provided arguments
    TEST_CMD="$@"
fi

log "📝 Running command: $TEST_CMD"
echo "========================================"

# Execute the test command with real-time output
$TEST_CMD 2>&1 | tee /app/test-results/e2e-test-output.log

# Capture exit code
TEST_EXIT_CODE=${PIPESTATUS[0]}

echo "========================================"
if [ $TEST_EXIT_CODE -eq 0 ]; then
    log "✅ E2E tests completed successfully!"
else
    log "❌ E2E tests failed with exit code: $TEST_EXIT_CODE"
fi

# Copy artifacts to mounted volume if available
if [ -d "/artifacts" ]; then
    log "📁 Copying test artifacts..."
    cp -r /app/test-results/* /artifacts/ 2>/dev/null || true
    log "✅ Artifacts copied to /artifacts"
fi

# Keep container running for a moment to allow log collection
if [ "$KEEP_CONTAINER_RUNNING" = "true" ]; then
    log "🔄 Keeping container running for debugging..."
    tail -f /dev/null
fi

cleanup
exit $TEST_EXIT_CODE