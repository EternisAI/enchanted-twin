# Docker-Based E2E Testing

This guide covers the complete Docker-based end-to-end testing setup for Enchanted Twin, designed for both local development and CI/CD environments.

## 🐳 Overview

Our Docker e2e testing setup provides:
- **Isolated test environment** - Clean, reproducible testing conditions
- **Full stack testing** - Backend + Frontend + Database in containers
- **Visible logs** - Real-time log streaming for debugging
- **CI/CD integration** - Optimized GitHub Actions workflow
- **Developer-friendly** - Easy local testing with comprehensive tooling

## 📋 Architecture

```
┌─────────────────┐    ┌─────────────────┐
│   E2E Tests     │    │   Backend       │
│   (Electron +   │───▶│   (Go Server)   │
│   Playwright)   │    │   + SQLite      │
└─────────────────┘    └─────────────────┘
        │                       │
        └───────────────────────┘
              Docker Network
```

### Components

1. **Backend Container**: Go server with SQLite database
2. **E2E Container**: Headless Electron + Playwright tests
3. **Docker Compose**: Orchestrates both containers with proper networking
4. **Shared Volumes**: Test results, logs, and artifacts

## 🚀 Quick Start

### Prerequisites

- Docker & Docker Compose
- Required environment variables (see [Environment Setup](#environment-setup))

### Run Tests

```bash
# Validate setup
npm run docker:validate

# Run e2e tests
npm run test:e2e:docker

# Run with debugging (keeps containers running)
npm run test:e2e:docker:debug

# Clean rebuild
npm run test:e2e:docker:clean
```

## 🔧 Environment Setup

### Required Environment Variables

Create a `.env` file in the project root:

```env
# Test Credentials
E2E_TEST_EMAIL=your-test-email@gmail.com
E2E_TEST_PASSWORD=your-test-password

# Firebase Configuration
VITE_FIREBASE_API_KEY=your-firebase-api-key
VITE_FIREBASE_AUTH_DOMAIN=your-project.firebaseapp.com
VITE_FIREBASE_PROJECT_ID=your-project-id

# AI API Keys
COMPLETIONS_API_KEY=your-openrouter-key
OPENROUTER_API_KEY=your-openrouter-key
EMBEDDINGS_API_KEY=your-openai-key
OPENAI_API_KEY=your-openai-key

# Optional Configuration
COMPLETIONS_API_URL=https://openrouter.ai/api/v1
COMPLETIONS_MODEL=openai/gpt-4o-mini
REASONING_MODEL=openai/gpt-4.1
EMBEDDINGS_API_URL=https://api.openai.com/v1
EMBEDDINGS_MODEL=text-embedding-3-small
ANONYMIZER_TYPE=no-op
```

### Environment Validation

```bash
# Validate all prerequisites
./scripts/validate-docker-e2e.sh

# Or using npm
npm run docker:validate
```

## 📚 Available Commands

### NPM Scripts (Project Root)

```bash
# Quick commands
npm run test:e2e:docker         # Run Docker e2e tests
npm run test:e2e:docker:debug   # Run with debugging
npm run test:e2e:docker:clean   # Clean rebuild and run
npm run docker:validate         # Validate setup

# Individual components
npm run dev:frontend            # Start frontend dev server
npm run dev:backend            # Start backend dev server
npm run build:frontend         # Build frontend
npm run build:backend          # Build backend
```

### Direct Script Usage

```bash
# Full option control
./scripts/docker-e2e.sh [OPTIONS] [TEST_COMMAND]

# Examples
./scripts/docker-e2e.sh --help
./scripts/docker-e2e.sh --keep-running --logs
./scripts/docker-e2e.sh --clean --build
./scripts/docker-e2e.sh "pnpm test:e2e:auth"
```

### Docker Compose Commands

```bash
# Manual container management
docker-compose -f docker-compose.e2e.yml up
docker-compose -f docker-compose.e2e.yml down
docker-compose -f docker-compose.e2e.yml logs backend
docker-compose -f docker-compose.e2e.yml logs e2e-tests
docker-compose -f docker-compose.e2e.yml exec e2e-tests bash
```

## 🔍 Debugging & Troubleshooting

### Local Debugging

```bash
# Run with debugging enabled
./scripts/docker-e2e.sh --keep-running --logs

# Access running containers
docker-compose -f docker-compose.e2e.yml exec e2e-tests bash
docker-compose -f docker-compose.e2e.yml exec backend bash

# View logs
docker-compose -f docker-compose.e2e.yml logs -f
```

### Log Locations

- **Test Results**: `test-results/artifacts/`
- **Screenshots**: `test-results/artifacts/*.png`
- **Backend Logs**: Available via `docker-compose logs backend`
- **E2E Logs**: Available via `docker-compose logs e2e-tests`

### Common Issues

#### 1. Environment Variables Missing
```bash
# Validate setup
./scripts/validate-docker-e2e.sh

# Check .env file exists and has required variables
```

#### 2. Docker Issues
```bash
# Check Docker is running
docker info

# Clean Docker state
docker system prune -af --volumes
```

#### 3. Backend Not Starting
```bash
# Check backend logs
docker-compose -f docker-compose.e2e.yml logs backend

# Verify API keys are set correctly
docker-compose -f docker-compose.e2e.yml exec backend env | grep API
```

#### 4. Test Failures
```bash
# Check screenshots
ls -la test-results/artifacts/

# Run with debug mode
./scripts/docker-e2e.sh --keep-running --logs

# Access test container for debugging
docker-compose -f docker-compose.e2e.yml exec e2e-tests bash
```

## 🚦 CI/CD Integration

### GitHub Actions

The project includes an optimized GitHub Actions workflow (`.github/workflows/e2e-docker.yml`) with:

- **Change Detection**: Only runs when relevant files change
- **Docker Layer Caching**: Speeds up builds using BuildKit cache
- **Parallel Execution**: Backend and frontend builds happen simultaneously
- **Artifact Collection**: Automatically uploads test results and screenshots
- **PR Comments**: Provides detailed test results in pull requests

### Manual Workflow Triggers

```bash
# Trigger via GitHub UI or API
gh workflow run e2e-docker.yml \
  --field keep_containers=true \
  --field test_command="pnpm test:e2e:auth"
```

### CI Optimization Features

1. **Smart Caching**:
   - Docker layer caching for faster builds
   - Separate caches for backend and frontend
   - Cache invalidation on dependency changes

2. **Change Detection**:
   - Skips tests when no relevant files changed
   - Monitors backend, frontend, and e2e-specific files
   - Reduces unnecessary CI runs

3. **Parallel Processing**:
   - Backend health checks run while building frontend
   - Simultaneous container preparation
   - Optimized for GitHub Actions runner limits

## 📁 File Structure

```
enchanted-twin/
├── docker-compose.e2e.yml          # Main orchestration file
├── scripts/
│   ├── docker-e2e.sh              # Main test runner script
│   └── validate-docker-e2e.sh     # Setup validation script
├── app/
│   ├── Dockerfile.e2e             # Frontend + E2E test container
│   └── docker/
│       └── entrypoint-e2e.sh      # E2E container entrypoint
├── backend/golang/
│   └── Dockerfile                 # Backend container
├── .github/workflows/
│   └── e2e-docker.yml            # CI/CD workflow
└── test-results/                  # Generated test artifacts
    └── artifacts/                 # Screenshots, logs, etc.
```

## 🛠️ Customization

### Custom Test Commands

```bash
# Run specific test suite
./scripts/docker-e2e.sh "pnpm test:e2e:auth"

# Run with custom Playwright options
./scripts/docker-e2e.sh "pnpm exec playwright test --headed"

# Run multiple test files
./scripts/docker-e2e.sh "pnpm test auth.e2e.ts chat.e2e.ts"
```

### Environment Overrides

```bash
# Override specific environment variables
BACKEND_PORT=45000 ./scripts/docker-e2e.sh

# Use different test credentials
E2E_TEST_EMAIL=other@test.com ./scripts/docker-e2e.sh
```

### Docker Compose Overrides

Create `docker-compose.override.yml` for local customizations:

```yaml
version: '3.8'
services:
  backend:
    ports:
      - "45000:44999"  # Use different port
  e2e-tests:
    environment:
      - DEBUG=true     # Enable debug mode
```

## 🔄 Development Workflow

### 1. Initial Setup
```bash
# Clone repository
git clone <repo-url>
cd enchanted-twin

# Validate Docker setup
npm run docker:validate

# Set up environment
cp .env.example .env
# Edit .env with your credentials
```

### 2. Running Tests
```bash
# Quick test run
npm run test:e2e:docker

# Development with debugging
npm run test:e2e:docker:debug
```

### 3. Debugging Issues
```bash
# Keep containers running for inspection
./scripts/docker-e2e.sh --keep-running

# Access test environment
docker-compose -f docker-compose.e2e.yml exec e2e-tests bash

# Check backend health
curl http://localhost:44999/query -X POST -H "Content-Type: application/json" -d '{"query": "{ __typename }"}'
```

### 4. Clean Up
```bash
# Manual cleanup
docker-compose -f docker-compose.e2e.yml down -v

# Full system cleanup
docker system prune -af --volumes
```

## 📊 Performance Optimization

### Local Development
- Use `--logs` flag to see real-time output
- Use `--keep-running` for debugging sessions
- Cache Docker images between runs

### CI/CD Environment
- Docker layer caching reduces build times by ~60%
- Change detection skips unnecessary test runs
- Parallel container startup optimizes execution time
- Artifact uploading provides debugging context

## 🤝 Contributing

When modifying the Docker e2e setup:

1. Test changes locally first
2. Update documentation if adding new features
3. Validate with `./scripts/validate-docker-e2e.sh`
4. Test in CI environment via PR

### Adding New Test Cases

1. Add test files to `app/tests/e2e/`
2. Update master test suite if needed
3. Test with Docker setup
4. Update CI workflow if new dependencies needed

### Modifying Docker Setup

1. Update relevant Dockerfile or docker-compose.e2e.yml
2. Test build process: `docker-compose -f docker-compose.e2e.yml build`
3. Validate full flow: `npm run test:e2e:docker:clean`
4. Update documentation

## 📞 Support

- **Documentation Issues**: Update this file via PR
- **Test Failures**: Check `test-results/artifacts/` for screenshots and logs
- **Docker Issues**: Validate setup with `npm run docker:validate`
- **CI Issues**: Check GitHub Actions logs and artifacts

---

Happy testing! 🚀