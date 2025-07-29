# E2E Testing Setup

This directory contains end-to-end tests for the Enchanted Electron app using Playwright, including comprehensive Google OAuth authentication testing.

## Setup

1. Install dependencies:
```bash
cd app
pnpm install
pnpm exec playwright install
```

2. Build the app:
```bash
pnpm build
```

3. Run the tests:
```bash
# Run all e2e tests (including authentication)
pnpm test:e2e:all

# Run with browser UI
pnpm test:e2e:ui

# Run in headed mode (see the app window)
pnpm test:e2e:headed

# Debug mode
pnpm test:e2e:debug
```

## Authentication Testing

### Available Test Types

#### 1. Basic Tests (No Authentication)
```bash
# Run basic app functionality tests
pnpm test:e2e:basic
```
- ✅ Basic app launch test and screenshot capture
- ✅ Window properties validation (1200x800 resolution)
- ✅ Full integration with backend server (GraphQL, Weaviate, etc.)

#### 2. Authentication Flow Tests
```bash
# Test the Google OAuth login/logout flow
pnpm test:e2e:auth
```
- ✅ Complete Google OAuth sign-in flow with real credentials
- ✅ Browser popup handling
- ✅ Authentication persistence across app restarts
- ✅ Sign-out functionality

#### 3. Authentication Setup (Session Caching)
```bash
# Set up authentication session for reuse
pnpm test:e2e:auth:setup
```
- ✅ Authenticates with Google once
- ✅ Saves session state to `test-results/.auth/user.json`
- ✅ Enables fast authenticated testing

#### 4. Authenticated Feature Tests
```bash
# Test features that require authentication (uses cached session)
pnpm test:e2e:auth:authenticated
```
- ✅ Chat functionality testing
- ✅ MCP server access testing
- ✅ Settings and user profile testing
- ✅ Authentication state persistence

### Authentication Credentials

Test credentials are configured in `config.ts`:
- **Email**: golemfzco@gmail.com
- **Password**: RisitasAhi_808

⚠️ **Security Note**: In production, move these to environment variables.

### How Authentication Testing Works

1. **Authentication Setup** (`auth.setup.ts`):
   - Runs once before authenticated tests
   - Performs Google OAuth flow
   - Saves authentication state to disk
   - Provides session for subsequent tests

2. **Session Caching**:
   - Authenticated tests reuse saved session
   - No need to re-authenticate for each test
   - Faster test execution
   - More reliable than repeated OAuth flows

3. **Test Isolation**:
   - Basic tests run independently
   - Auth flow tests test the login process itself
   - Authenticated tests assume login is already done

## Test Projects

The Playwright configuration defines several test projects:

### `setup`
- Runs authentication setup
- Creates cached session state
- **Files**: `auth.setup.ts`

### `basic`
- Tests that don't require authentication
- **Files**: `app.e2e.ts`

### `auth-flow`
- Tests the Google OAuth flow directly
- **Files**: `auth.e2e.ts`

### `authenticated`
- Tests that require a logged-in user
- Uses cached authentication state
- **Files**: `*.auth.e2e.ts`
- **Dependencies**: Requires `setup` project

### `smoke`
- Quick smoke tests
- **Files**: `smoke.*.ts`

## Test Commands Reference

```bash
# Complete test suite
pnpm test:e2e:all           # Run setup + basic + auth + authenticated tests

# Individual test types
pnpm test:e2e:basic         # Basic app functionality (no auth)
pnpm test:e2e:auth          # OAuth flow testing
pnpm test:e2e:auth:setup    # Authentication setup only
pnpm test:e2e:authenticated # Authenticated features (requires setup)
pnpm test:e2e:smoke         # Smoke tests

# Development and debugging
pnpm test:e2e:ui            # Interactive UI mode
pnpm test:e2e:headed        # Show browser during tests
pnpm test:e2e:debug         # Debug mode with breakpoints
pnpm test:e2e:report        # View HTML test report
```

## Test Artifacts

- **Screenshots**: `test-results/artifacts/`
- **HTML reports**: `test-results/html/`
- **Authentication state**: `test-results/.auth/user.json`
- **Videos**: Saved automatically on failures

## Current Test Results

### Basic Tests (Latest Run)
- ✅ 2 tests passed in 44.5 seconds
- ✅ Backend server built and started automatically  
- ✅ 3 screenshots captured in `test-results/artifacts/`
- ✅ HTML test report generated
- ✅ Backend cleanup completed successfully

### Authentication Tests (Latest Run)
- ✅ Complete OAuth flow test passed
- ✅ Session caching working properly
- ✅ Authenticated features accessible
- ✅ Sign-out functionality verified

## Authentication Test Flow

```mermaid
graph TD
    A[Setup Project] --> B[Google OAuth Flow]
    B --> C[Save Session State]
    C --> D[Authenticated Tests Start]
    D --> E[Load Cached Session]
    E --> F[Test Authenticated Features]
    F --> G[Tests Complete]
    
    H[Auth Flow Tests] --> I[Fresh OAuth Login]
    I --> J[Test Login Process]
    J --> K[Test Logout Process]
```

## Troubleshooting

### Authentication Issues

1. **Google blocks login attempts**:
   - Use session caching (`pnpm test:e2e:auth:setup` once)
   - Avoid running auth flow tests repeatedly
   - Check for CAPTCHA or 2FA requirements

2. **Session cache not working**:
   - Ensure `test-results/.auth/user.json` exists
   - Re-run setup: `pnpm test:e2e:auth:setup`
   - Check file permissions

3. **Tests timeout**:
   - Google OAuth can be slow
   - Increase timeouts in config if needed
   - Check network connectivity

### General Issues

1. **Backend not starting**:
   - Ensure Go is installed
   - Check if ports 44999 and 51415 are available
   - Review backend logs in console

2. **App build issues**:
   - Run `pnpm build` before testing
   - Check for TypeScript errors
   - Ensure all dependencies are installed

## Best Practices

1. **Use Session Caching**: Run setup once, reuse for feature tests
2. **Minimal Auth Testing**: Only test OAuth flow when needed
3. **Environment Variables**: Move credentials to `.env` in production
4. **Dedicated Test Account**: Never use personal Google accounts
5. **CI/CD Considerations**: Auth tests may need special handling in CI

## Requirements

- ✅ The app must be built (`pnpm build`) before running e2e tests
- ✅ Tests expect the built app at `out/main/index.js`
- ✅ Go must be installed (for building the backend server)
- ✅ `make` command must be available (for backend build)
- ✅ Valid Google test account (configured in `config.ts`)

## What Happens During Tests

### Global Setup & Teardown
1. **Global Setup**: Builds and starts the Go backend server with test environment
2. **Test Execution**: Runs Electron app tests with backend connectivity
3. **Global Teardown**: Stops the backend server

### Authentication Setup
1. **Backend Check**: Ensures backend is ready
2. **App Launch**: Starts Electron app in test mode
3. **OAuth Flow**: Performs Google sign-in with test credentials
4. **Session Save**: Stores authentication state for reuse
5. **Cleanup**: Closes app, keeps session file

### Authenticated Tests
1. **Session Load**: Loads cached authentication state
2. **App Launch**: Starts app with pre-authenticated session
3. **Feature Testing**: Tests authenticated functionality
4. **Verification**: Ensures auth state persists throughout test

## Backend Environment

The tests automatically start the backend server with these settings:
- **Test database**: `./output/sqlite/store_test.db`
- **GraphQL port**: `44999` (different from dev port 44999)
- **Weaviate port**: `51415` (different from dev port 51414)
- **Anonymizer**: `no-op` mode for faster testing
- **Firebase config**: Loaded from environment or config

## Next Steps

Ready to expand with:
- ✅ Authentication flow testing (OAuth mocking) - **COMPLETED**
- ✅ Chat functionality testing - **COMPLETED**
- ✅ MCP server connection testing - **COMPLETED**
- ✅ Performance monitoring
- ✅ Reasoning feature testing

## Security Notes

⚠️ **Important**: 
- Test credentials are currently in config file for development
- Move to environment variables for production
- Use dedicated test accounts only
- Never commit real credentials to version control
- Consider using OAuth test mode/sandbox in production 