# E2E Testing Setup

## Overview

This E2E testing setup provides **two ways** to run comprehensive tests:

1. **🐳 Docker Setup (Recommended)** - Complete isolation with automatic environment setup
2. **💻 Local Setup** - Traditional local development testing

## 🐳 Docker E2E Testing (Recommended)

The Docker setup provides complete isolation and consistent environment across all machines.

### Quick Start

```bash
# 1. Copy and configure environment
cp tests/e2e/env.docker.example tests/e2e/.env
# Edit tests/e2e/.env with your API keys

# 2. Setup Docker environment
pnpm run docker:e2e:setup

# 3. Run tests
pnpm run docker:e2e:test
```

### Docker Commands

```bash
pnpm run docker:e2e:setup     # Build Docker image and check prerequisites
pnpm run docker:e2e:test      # Run complete E2E test suite
pnpm run docker:e2e:debug     # Start debug mode with VNC access
pnpm run docker:e2e:build     # Rebuild Docker image
pnpm run docker:e2e:cleanup   # Clean up containers and volumes
```

### Docker Features

- **✅ Complete Isolation** - No local dependencies except Docker
- **✅ Optimized Context** - 53,000x reduction in build context (5.74GB → 108kB)
- **✅ Electron Support** - Proper X11 forwarding with virtual display
- **✅ VNC Debugging** - Visual access to running tests on port 5900
- **✅ Backend Integration** - Automatic Go backend building and startup
- **✅ Consistent Environment** - Same environment across all machines

### VNC Debugging

```bash
pnpm run docker:e2e:debug
# Connect VNC client to localhost:5900 to see Electron app running
```

## 💻 Local E2E Testing

### Master Test Suite

The master test suite (`master.e2e.ts`) runs a complete flow that includes:

1. **Backend Startup** - Build and start the backend server
2. **Electron Launch** - Start a fresh Electron application instance
3. **Authentication Flow** - Google OAuth login with verification
4. **Chat Functionality** - Message sending and interface interaction
5. **Final Verification** - Confirm all systems working correctly
6. **Cleanup** - Graceful shutdown of all components

This setup uses a **modular, phase-based architecture** that avoids restarting the Electron instance between auth and chat tests, preventing the need to re-authenticate.

## Architecture Overview

### Core Structure
- **Phase-based execution** - Each major step is isolated into focused functions
- **Utility classes** - Consistent logging and screenshot management
- **Type safety** - Full TypeScript types for Playwright components
- **Error resilience** - Graceful handling of failures at any stage

### Utility Classes

#### TestLogger
Provides consistent, emoji-based logging throughout the test suite:
- `TestLogger.phase(number, title)` - Major phase announcements
- `TestLogger.step(phase, step, title)` - Sub-step tracking
- `TestLogger.success(message)` - Success confirmations
- `TestLogger.warning(message)` - Non-critical issues
- `TestLogger.error(message)` - Error reporting
- `TestLogger.info(message)` - General information

#### ScreenshotHelper
Manages screenshot capture with consistent naming:
- `ScreenshotHelper.capture(page, path, description?)` - Captures full-page screenshots
- Automatic logging of screenshot saves
- Centralized path management

### Constants Configuration
All timeouts and screenshot paths are centralized at the top of the file:
```typescript
const TIMEOUTS = {
  SHORT: 5000,      // Basic UI waits
  MEDIUM: 10000,    // Fallback waits
  LONG: 60000       // OAuth/network operations
}

const SCREENSHOT_PATHS = {
  INITIAL: 'test-results/artifacts/master-test-initial-state.png',
  AUTHENTICATED: 'test-results/artifacts/master-test-authenticated-state.png',
  CHAT_NOT_READY: 'test-results/artifacts/master-test-chat-not-ready.png',
  FINAL_SUCCESS: 'test-results/artifacts/master-test-final-success.png',
  ERROR: 'test-results/artifacts/master-test-error.png'
}
```

## 📁 Files Overview

### Docker Files
- `Dockerfile.e2e` - Main E2E testing container with Node.js, Go, Electron dependencies
- `docker-compose.e2e.yml` - Docker Compose configuration with networking and volumes
- `docker/entrypoint-e2e.sh` - Container startup script with multiple modes
- `scripts/docker-e2e.sh` - Helper script for Docker operations
- `tests/e2e/env.docker.example` - Environment configuration template
- `scripts/validate-docker-e2e.sh` - Setup validation script
- `.dockerignore` - Context optimization (reduces build context by 53,000x)

### Core Test Files
- `master.e2e.ts` - **Refactored modular test suite** with phase-based execution

### Helper Functions

#### Auth Helpers (`auth.helpers.ts`)
- `signInWithGoogle()` - Complete Google OAuth flow
- `isAuthenticated()` - Check authentication status
- `clearAuthState()` - Clear authentication data
- `createCleanElectronConfig()` - Create fresh Electron config

#### Chat Helpers (`chat.helpers.ts`)
- `waitForChatInterface()` - Wait for chat UI to be ready
- `sendChatMessage()` - Send a message in chat
- `waitForChatResponse()` - Wait for chat response
- `getChatInput()` - Get chat input element
- `screenshotChatState()` - Take chat screenshots

## 🚀 Running Tests

### Docker Tests (Recommended)
```bash
# Complete setup and test run
pnpm run docker:e2e:setup && pnpm run docker:e2e:test

# Individual commands
pnpm run docker:e2e:test      # Run tests
pnpm run docker:e2e:debug     # Debug with VNC
pnpm run docker:e2e:cleanup   # Clean up
```

### Local Tests
```bash
# Run auth + chat flow in single instance
pnpm test:e2e:master
```


## Test Flow (Phase-Based)

The master test follows this **modular sequence**:

### Phase 1: Backend Startup
- `startBackendPhase()` - Verify and start backend services

### Phase 2: Electron Launch  
- `launchElectronPhase()` - Create fresh Electron instance with clean config

### Phase 3: Authentication
- `verifyUnauthenticatedState()` - Confirm clean starting state
- `performGoogleSignIn()` - Execute OAuth flow
- `verifyAuthenticationSuccess()` - Validate auth state and user data

### Phase 4: Chat Testing
- `waitForChatInterfaceReady()` - Ensure chat UI is loaded
- `runAllChatTests()` - Execute comprehensive chat test suite

### Phase 5: Final Verification
- `finalVerificationPhase()` - Confirm all systems still working

### Cleanup
- `cleanupPhase()` - Graceful shutdown of Electron and backend
- `closeElectronApp()` - Safe app termination
- `stopBackend()` - Backend service shutdown

## Error Handling & Recovery

The refactored test suite includes **comprehensive error handling**:

### Error Capture
- **Automatic screenshots** on any failure
- **Detailed logging** of error context
- **Page state inspection** (URL, content, localStorage)
- **Graceful degradation** for non-critical failures

### Recovery Strategies
- **Chat interface fallbacks** - Multiple detection strategies
- **Optional error screenshots** - Continue if screenshot fails  
- **Null-safe cleanup** - Handle partial initialization failures
- **Timeout escalation** - Progressive wait strategies

### Debug Information
On failures, the system automatically captures:
- Current page URL and content
- localStorage state (user data)
- Full-page error screenshots
- Console error details

## Chat Interface Detection

The chat helpers try **multiple selectors** to find the chat input:

1. **Primary**: `textarea[placeholder*="message"], textarea[placeholder*="chat"], textarea[placeholder*="type"]`
2. **Alternative**: `textarea.outline-none.bg-transparent` 
3. **Fallback**: `textarea, input[type="text"][placeholder*="message"]`

## Screenshots & Artifacts

All test runs generate **organized screenshots** in `test-results/artifacts/`:

- `master-test-initial-state.png` - Before authentication
- `master-test-authenticated-state.png` - After successful auth
- `master-test-chat-not-ready.png` - Debug chat interface issues
- `master-test-final-success.png` - Complete test success
- `master-test-error.png` - Failure state capture

## 🐳 Docker Configuration

### Environment Setup

1. **Copy environment template:**
   ```bash
   cp tests/e2e/env.docker.example tests/e2e/.env
   ```

2. **Configure required variables:**
   ```env
   # Required API Keys
   COMPLETIONS_API_KEY=your_actual_key
   OPENROUTER_API_KEY=your_actual_key  
   OPENAI_API_KEY=your_actual_key
   EMBEDDINGS_API_KEY=your_actual_key
   
   # Firebase Configuration
   FIREBASE_API_KEY=your_firebase_key
   FIREBASE_AUTH_DOMAIN=your-project.firebaseapp.com
   FIREBASE_PROJECT_ID=your-project-id
   
   # Google OAuth (for E2E testing)
   GOOGLE_TEST_EMAIL=your_test_email@gmail.com
   GOOGLE_TEST_PASSWORD=your_test_password
   ```

### Validation

Run the validation script to check your setup:
```bash
./scripts/validate-docker-e2e.sh
```

### Docker Architecture

- **Container**: Node.js 20 + Go 1.22 + Electron dependencies
- **Virtual Display**: Xvfb with Fluxbox window manager
- **Context Optimization**: `.dockerignore` reduces context from 5.74GB to 108kB
- **Build Process**: Automatic backend compilation and frontend setup
- **Networking**: Isolated network with port forwarding

## 🛠️ Docker Troubleshooting

### Common Issues

**1. Environment file missing**
```bash
Error: Environment file not found
Solution: cp tests/e2e/env.docker.example tests/e2e/.env
```

**2. API key errors** 
```bash
Error: Invalid API key
Solution: Edit tests/e2e/.env with valid API keys
```

**3. Backend build fails**
```bash
Error: Backend build failed
Solution: Check logs with pnpm run docker:e2e:debug
```

**4. Docker permission errors**
```bash
Error: Permission denied  
Solution: Ensure Docker daemon is running and user has permissions
```

### Debug Steps

1. **Validation**: `./scripts/validate-docker-e2e.sh`
2. **Build logs**: `pnpm run docker:e2e:build`
3. **VNC debugging**: `pnpm run docker:e2e:debug` (connect to `localhost:5900`)
4. **Interactive shell**: `docker-compose -f docker-compose.e2e.yml run --rm e2e-tests shell`

## 💻 Local Configuration

Make sure your local test configuration includes:

- **Google test credentials** - Valid OAuth test accounts
- **Firebase test config** - Proper project settings
- **Backend URL configuration** - Correct service endpoints
- **Proper timeouts** - Adequate time for OAuth flow

## Benefits of Refactored Architecture

### ✅ **Maintainability**
- **Single responsibility** functions
- **Easy to modify** individual phases
- **Clear separation** of concerns

### ✅ **Debugging**
- **Consistent logging** with phase tracking
- **Detailed error context** capture
- **Strategic screenshot** placement

### ✅ **Reliability**  
- **Type-safe** function signatures
- **Null-safe** error handling
- **Progressive fallback** strategies

### ✅ **Extensibility**
- **Modular functions** can be reused
- **Utility classes** provide consistent behavior
- **Phase-based structure** allows easy insertion of new steps

## Tips

1. **Backend must be running** before starting tests
2. **Google credentials** must be valid test accounts
3. **Chat interface timing** may vary - tests include progressive wait strategies
4. **Screenshots help debug** issues when tests fail
5. **Master test recommended** over individual tests for full flow verification
6. **Phase logs** make it easy to identify where failures occur
7. **Utility functions** can be reused in other test files 
