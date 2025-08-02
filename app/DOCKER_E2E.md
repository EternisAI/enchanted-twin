# Docker E2E Testing Setup

This guide explains how to run the master E2E test suite in Docker containers using pnpm.

## Overview

The Docker E2E setup provides:

- **Complete Isolation**: Everything runs in containers
- **Electron Support**: Proper X11 forwarding with Xvfb
- **VNC Debugging**: Visual access to the running tests
- **Backend Integration**: Automatic backend building and startup
- **OAuth Testing**: Headless OAuth flow support

## Files Created

```
app/
├── Dockerfile.e2e                 # Main E2E testing container
├── docker-compose.e2e.yml         # Docker Compose configuration
├── docker/
│   └── entrypoint-e2e.sh         # Container startup script
├── scripts/
│   └── docker-e2e.sh             # Helper script for Docker operations
└── tests/e2e/
    └── env.docker.example        # Environment configuration template

backend/golang/
└── Dockerfile.backend            # Optional backend-only container
```

## First Time Setup

### 1. Configure Environment

Copy the environment template and configure it with your API keys:

```bash
cd app
cp tests/e2e/env.docker.example tests/e2e/.env
```

Edit `tests/e2e/.env` with your actual values:

```env
# Required API Keys
COMPLETIONS_API_KEY=your_actual_key
OPENROUTER_API_KEY=your_actual_key
OPENAI_API_KEY=your_actual_key
EMBEDDINGS_API_KEY=your_actual_key

# Firebase Test Configuration
FIREBASE_API_KEY=your_firebase_key
FIREBASE_AUTH_DOMAIN=your-project.firebaseapp.com
FIREBASE_PROJECT_ID=your-project-id

# Google Test Credentials (for OAuth)
GOOGLE_TEST_EMAIL=your_test_email@gmail.com
GOOGLE_TEST_PASSWORD=your_test_password
```

### 2. Setup Docker Environment

```bash
pnpm run docker:e2e:setup
```

This will:
- Check prerequisites (Docker, Docker Compose)
- Build the Docker image
- Prepare the environment

## Running Tests

### Complete E2E Test Suite

```bash
pnpm run docker:e2e:test
```

This runs the full master test suite:
1. Builds and starts backend server
2. Launches Electron application
3. Performs Google OAuth authentication
4. Tests chat functionality
5. Verifies final state
6. Cleans up gracefully

### Available Commands

```bash
# Setup and build
pnpm run docker:e2e:setup

# Run tests
pnpm run docker:e2e:test

# Debug mode with VNC
pnpm run docker:e2e:debug

# Rebuild Docker image
pnpm run docker:e2e:build

# Clean up containers and volumes
pnpm run docker:e2e:cleanup
```

### Using the Helper Script Directly

```bash
# All commands available
./scripts/docker-e2e.sh {setup|build|test|debug|logs|cleanup}

# Examples
./scripts/docker-e2e.sh test
./scripts/docker-e2e.sh debug
./scripts/docker-e2e.sh cleanup
```

## Debugging

### VNC Access

Start debug mode to get VNC access:

```bash
pnpm run docker:e2e:debug
```

Then connect with any VNC client:
- **Host**: `localhost`
- **Port**: `5900`
- **Password**: None required

You can see the Electron application running and interact with it manually.

### Interactive Shell

Get shell access to the container:

```bash
docker-compose -f docker-compose.e2e.yml run --rm e2e-tests shell
```

### View Logs

```bash
./scripts/docker-e2e.sh logs
```

### Manual Test Execution

In debug mode or shell mode, you can run tests manually:

```bash
# Start backend manually
cd /app/backend/golang
./bin/enchanted-twin &

# Run specific tests
cd /app
pnpm test:e2e:master
```

## Architecture

### Container Structure

The main container includes:
- **Node.js 20** with pnpm
- **Go 1.22** for backend compilation
- **Electron dependencies** (X11, graphics libraries)
- **Playwright browsers**
- **Virtual display** (Xvfb) with window manager (Fluxbox)
- **VNC server** for debugging

### Network Setup

- **Port 44999**: GraphQL backend
- **Port 51415**: Weaviate database
- **Port 5900**: VNC for debugging

### Volume Mounts

- `./test-results`: Test artifacts and screenshots
- `./tests/e2e/.env`: Environment configuration
- `e2e-temp`: Temporary files
- `backend-data`: Backend data persistence

## Troubleshooting

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

**3. Docker permission errors**
```bash
Error: Permission denied
Solution: Ensure Docker daemon is running and user has Docker permissions
```

**4. Backend fails to start**
```bash
Error: Backend failed to start
Solution: Check logs with ./scripts/docker-e2e.sh logs
```

### Debug Steps

1. **Check prerequisites**:
   ```bash
   ./scripts/docker-e2e.sh setup
   ```

2. **View detailed logs**:
   ```bash
   ./scripts/docker-e2e.sh logs
   ```

3. **Use VNC debugging**:
   ```bash
   pnpm run docker:e2e:debug
   # Connect VNC to localhost:5900
   ```

4. **Interactive troubleshooting**:
   ```bash
   docker-compose -f docker-compose.e2e.yml run --rm e2e-tests shell
   ```

### Performance Tips

1. **Build once, run multiple times**:
   ```bash
   pnpm run docker:e2e:build  # Build once
   pnpm run docker:e2e:test   # Run multiple times
   ```

2. **Keep containers running for development**:
   ```bash
   pnpm run docker:e2e:debug  # Keeps container alive
   ```

3. **Clean up periodically**:
   ```bash
   pnpm run docker:e2e:cleanup
   ```

## Integration with CI/CD

### GitHub Actions Example

```yaml
name: E2E Tests (Docker)

on: [push, pull_request]

jobs:
  e2e-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Environment
        run: |
          cp app/tests/e2e/env.docker.example app/tests/e2e/.env
          # Set environment variables from secrets
        env:
          COMPLETIONS_API_KEY: ${{ secrets.COMPLETIONS_API_KEY }}
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
          # ... other secrets
      
      - name: Run E2E Tests
        run: |
          cd app
          ./scripts/docker-e2e.sh test
      
      - name: Upload Test Results
        uses: actions/upload-artifact@v3
        if: always()
        with:
          name: e2e-test-results
          path: app/test-results/
```

## Benefits

### ✅ **Isolation**
- No local dependencies except Docker
- Consistent environment across machines
- No conflicts with local development setup

### ✅ **Reliability**
- Reproducible test environment
- Proper Electron sandboxing
- Comprehensive error handling

### ✅ **Debugging**
- VNC access for visual debugging
- Interactive shell access
- Detailed logging and screenshots

### ✅ **Automation**
- CI/CD friendly
- Parallel test execution
- Automatic cleanup

This Docker setup provides a robust, isolated environment for running your E2E tests with full Electron and backend support. 