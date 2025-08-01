# E2E Testing Setup

## Master Test Suite

The master test suite (`master.e2e.ts`) runs a complete flow that includes:

1. **Authentication Flow** - Google OAuth login
2. **Chat Functionality** - Message sending and interface interaction

This setup avoids restarting the Electron instance between auth and chat tests, which prevents the need to re-authenticate.

## Files Overview

### Core Files
- `master.e2e.ts` - Main test file that runs auth → chat flow
- `auth.helpers.ts` - Authentication helper functions
- `chat.helpers.ts` - Chat interaction helper functions  
- `chat.e2e.ts` - Chat test functions (can be used standalone or called from master)

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

### Run Individual Test Suites
```bash
# Run only authentication tests
npm run test:e2e:auth
# OR
pnpm test:e2e:auth

# Run only chat tests (requires prior auth)
npm run test:e2e:chat
# OR  
pnpm test:e2e:chat
```

## Test Flow

The master test follows this sequence:

1. **Backend Check** - Verify backend is running
2. **App Launch** - Start fresh Electron instance  
3. **Authentication** - Complete Google OAuth
4. **Chat Interface** - Wait for chat to be ready
5. **Chat Testing** - Send messages and verify functionality
6. **Cleanup** - Close app and clean temporary files

## Chat Interface Detection

The chat helpers try multiple selectors to find the chat input:

1. **Primary**: `textarea[placeholder*="message"], textarea[placeholder*="chat"], textarea[placeholder*="type"]`
2. **Alternative**: `textarea.outline-none.bg-transparent` 
3. **Fallback**: `textarea, input[type="text"][placeholder*="message"]`

## Screenshots

All test runs generate screenshots in `test-results/artifacts/`:

- `master-test-initial-state.png` - Before authentication
- `master-test-authenticated-state.png` - After successful auth
- `chat-interface-ready.png` - Chat interface detected
- `chat-before-send.png` - Before sending message
- `chat-after-send.png` - After sending message
- `master-test-final-success.png` - Complete test success

## Error Handling

The tests include comprehensive error handling:

- Screenshots on failures
- Fallback chat selectors
- Graceful degradation for optional tests
- Detailed console logging

## Configuration

Make sure your test configuration includes:

- Google test credentials
- Firebase test config  
- Backend URL configuration
- Proper timeouts for OAuth flow

## Tips

1. **Backend must be running** before starting tests
2. **Google credentials** must be valid test accounts
3. **Chat interface** timing may vary - tests include wait strategies
4. **Screenshots** help debug issues when tests fail
5. **Master test** is recommended over individual tests for full flow verification 