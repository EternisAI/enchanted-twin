#!/bin/bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log() {
    echo -e "${BLUE}[E2E-Docker]${NC} $1"
}

success() {
    echo -e "${GREEN}[E2E-Docker]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[E2E-Docker]${NC} $1"
}

error() {
    echo -e "${RED}[E2E-Docker]${NC} $1"
}

# Check if required files exist
check_prerequisites() {
    log "Checking prerequisites..."
    
    if [ ! -f "$PROJECT_ROOT/tests/e2e/.env" ]; then
        warn "Environment file not found. Please copy env.docker.example to .env and configure it."
        echo "  cp tests/e2e/env.docker.example tests/e2e/.env"
        echo "  # Edit tests/e2e/.env with your API keys"
        exit 1
    fi
    
    if ! command -v docker &> /dev/null; then
        error "Docker is not installed or not in PATH"
        exit 1
    fi
    
    if ! command -v docker-compose &> /dev/null; then
        error "Docker Compose is not installed or not in PATH"
        exit 1
    fi
    
    success "Prerequisites check passed"
}

# Build the Docker image
build_image() {
    log "Building E2E test Docker image..."
    cd "$PROJECT_ROOT"
    docker-compose -f docker-compose.e2e.yml build e2e-tests
    success "Docker image built successfully"
}

# Run the tests
run_tests() {
    log "Running E2E tests in Docker..."
    cd "$PROJECT_ROOT"
    
    # Ensure test results directory exists
    mkdir -p test-results/artifacts
    
    docker-compose -f docker-compose.e2e.yml run --rm e2e-tests test
    success "E2E tests completed"
}

# Debug mode with VNC
debug_mode() {
    log "Starting debug mode with VNC..."
    warn "VNC will be available on port 5900"
    warn "You can connect with any VNC client (password not required)"
    
    cd "$PROJECT_ROOT"
    docker-compose -f docker-compose.e2e.yml run --rm -p 5900:5900 e2e-tests debug
}

# Clean up containers and volumes
cleanup() {
    log "Cleaning up Docker resources..."
    cd "$PROJECT_ROOT"
    docker-compose -f docker-compose.e2e.yml down -v
    docker system prune -f
    success "Cleanup completed"
}

# Show logs
show_logs() {
    cd "$PROJECT_ROOT"
    docker-compose -f docker-compose.e2e.yml logs -f
}

# Main command handler
case "$1" in
    "build")
        check_prerequisites
        build_image
        ;;
    "test")
        check_prerequisites
        run_tests
        ;;
    "debug")
        check_prerequisites
        debug_mode
        ;;
    "logs")
        show_logs
        ;;
    "cleanup")
        cleanup
        ;;
    "setup")
        check_prerequisites
        build_image
        success "Setup completed. You can now run: ./scripts/docker-e2e.sh test"
        ;;
    *)
        echo "Usage: $0 {setup|build|test|debug|logs|cleanup}"
        echo ""
        echo "Commands:"
        echo "  setup   - Check prerequisites and build image"
        echo "  build   - Build the Docker image"
        echo "  test    - Run E2E tests in Docker"
        echo "  debug   - Start debug mode with VNC access"
        echo "  logs    - Show container logs"
        echo "  cleanup - Remove containers and volumes"
        echo ""
        echo "First time setup:"
        echo "  1. cp tests/e2e/env.docker.example tests/e2e/.env"
        echo "  2. Edit tests/e2e/.env with your API keys"
        echo "  3. ./scripts/docker-e2e.sh setup"
        echo "  4. ./scripts/docker-e2e.sh test"
        exit 1
        ;;
esac 