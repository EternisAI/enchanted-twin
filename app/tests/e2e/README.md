# E2E Testing Setup

## Master Test Suite

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

## Files Overview

### Core Files
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

## Running Tests

### Run the Complete Master Test
```bash
# Run auth + chat flow in single instance
npm run test:e2e:master
# OR with pnpm
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

## Configuration

Make sure your test configuration includes:

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
