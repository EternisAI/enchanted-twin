#!/bin/bash
set -e

echo "🐳 Starting E2E Test Container..."

# Start Xvfb for headless display
echo "🖥️  Starting virtual display..."
Xvfb :99 -screen 0 1280x1024x24 -ac -nolisten tcp -dpi 96 &
export DISPLAY=:99

# Wait for display to be ready
sleep 2

# Optional: Start VNC server for debugging (uncomment if needed)
# x11vnc -display :99 -forever -usepw -create &

# Start window manager
fluxbox &

echo "✅ Virtual display ready"

# Function to handle different commands
case "$1" in
  "test")
    echo "🧪 Running E2E tests..."
    
    # Start backend in background
    echo "🚀 Starting backend server..."
    cd /app/backend/golang
    ./bin/enchanted-twin &
    BACKEND_PID=$!
    
    # Wait for backend to be ready
    echo "⏳ Waiting for backend to start..."
    timeout=60
    while ! curl -f http://localhost:44999/query >/dev/null 2>&1; do
      sleep 1
      timeout=$((timeout - 1))
      if [ $timeout -eq 0 ]; then
        echo "❌ Backend failed to start"
        exit 1
      fi
    done
    echo "✅ Backend is ready"
    
    # Switch to app directory and run tests
    cd /app
    echo "🎯 Running master E2E test suite..."
    pnpm test:e2e:master
    
    # Cleanup
    kill $BACKEND_PID 2>/dev/null || true
    ;;
    
  "backend-only")
    echo "🏗️  Starting backend only..."
    cd /app/backend/golang
    ./bin/enchanted-twin
    ;;
    
  "debug")
    echo "🔍 Starting debug mode..."
    echo "VNC available on port 5900"
    echo "Backend will start on port 44999"
    
    # Start VNC server
    x11vnc -display :99 -forever -nopw -create &
    
    cd /app/backend/golang
    ./bin/enchanted-twin &
    
    cd /app
    echo "🎯 Container ready for debugging. Connect via VNC or run commands manually."
    tail -f /dev/null
    ;;
    
  "shell")
    echo "🐚 Starting interactive shell..."
    exec /bin/bash
    ;;
    
  *)
    echo "Usage: $0 {test|backend-only|debug|shell}"
    echo "  test       - Run complete E2E test suite"
    echo "  backend-only - Start only backend server"
    echo "  debug      - Start with VNC for debugging"
    echo "  shell      - Interactive shell"
    exit 1
    ;;
esac 