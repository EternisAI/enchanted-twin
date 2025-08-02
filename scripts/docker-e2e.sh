#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_colored() {
    echo -e "${1}${2}${NC}"
}

# Function to print section headers
print_header() {
    echo
    print_colored $BLUE "========================================"
    print_colored $BLUE "$1"
    print_colored $BLUE "========================================"
}

# Function to validate required environment variables
validate_env() {
    local missing_vars=()
    
    # Required test credentials
    [ -z "$E2E_TEST_EMAIL" ] && missing_vars+=("E2E_TEST_EMAIL")
    [ -z "$E2E_TEST_PASSWORD" ] && missing_vars+=("E2E_TEST_PASSWORD")
    
    # Required Firebase config
    [ -z "$VITE_FIREBASE_API_KEY" ] && missing_vars+=("VITE_FIREBASE_API_KEY")
    [ -z "$VITE_FIREBASE_AUTH_DOMAIN" ] && missing_vars+=("VITE_FIREBASE_AUTH_DOMAIN")
    [ -z "$VITE_FIREBASE_PROJECT_ID" ] && missing_vars+=("VITE_FIREBASE_PROJECT_ID")
    
    # Required API keys
    [ -z "$COMPLETIONS_API_KEY" ] && [ -z "$OPENROUTER_API_KEY" ] && missing_vars+=("COMPLETIONS_API_KEY or OPENROUTER_API_KEY")
    [ -z "$EMBEDDINGS_API_KEY" ] && [ -z "$OPENAI_API_KEY" ] && missing_vars+=("EMBEDDINGS_API_KEY or OPENAI_API_KEY")
    
    if [ ${#missing_vars[@]} -ne 0 ]; then
        print_colored $RED "❌ Missing required environment variables:"
        for var in "${missing_vars[@]}"; do
            print_colored $RED "   - $var"
        done
        echo
        print_colored $YELLOW "💡 Create a .env file in the project root with these variables"
        print_colored $YELLOW "   or set them in your environment before running this script."
        exit 1
    fi
}

# Function to show usage
show_usage() {
    cat << EOF
🐳 Docker E2E Test Runner

Usage: $0 [OPTIONS] [TEST_COMMAND]

OPTIONS:
  -h, --help              Show this help message
  -k, --keep-running      Keep containers running after tests (for debugging)
  -c, --clean             Clean up containers and volumes before running
  -l, --logs              Follow logs from both containers during test execution
  --build                 Force rebuild of Docker images
  --no-cache              Build images without using cache

TEST_COMMAND:
  Optional command to run instead of default 'pnpm test:e2e:master'
  Examples:
    $0 "pnpm test:e2e:auth"
    $0 "pnpm test --reporter=verbose"

ENVIRONMENT VARIABLES:
  Required:
    E2E_TEST_EMAIL          - Google test account email
    E2E_TEST_PASSWORD       - Google test account password
    VITE_FIREBASE_API_KEY   - Firebase API key
    VITE_FIREBASE_AUTH_DOMAIN - Firebase auth domain
    VITE_FIREBASE_PROJECT_ID - Firebase project ID
    COMPLETIONS_API_KEY or OPENROUTER_API_KEY - AI completions API key
    EMBEDDINGS_API_KEY or OPENAI_API_KEY - Embeddings API key

EXAMPLES:
  # Run default master e2e test
  $0

  # Run with debug mode (keep containers running)
  $0 --keep-running

  # Clean and rebuild everything
  $0 --clean --build

  # Follow logs during execution
  $0 --logs

  # Run specific test command
  $0 "pnpm test:e2e:auth"

EOF
}

# Parse command line arguments
KEEP_RUNNING=false
CLEAN=false
FOLLOW_LOGS=false
BUILD_ARGS=""
TEST_COMMAND=""

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_usage
            exit 0
            ;;
        -k|--keep-running)
            KEEP_RUNNING=true
            shift
            ;;
        -c|--clean)
            CLEAN=true
            shift
            ;;
        -l|--logs)
            FOLLOW_LOGS=true
            shift
            ;;
        --build)
            BUILD_ARGS="$BUILD_ARGS --build"
            shift
            ;;
        --no-cache)
            BUILD_ARGS="$BUILD_ARGS --no-cache"
            shift
            ;;
        *)
            TEST_COMMAND="$1"
            shift
            ;;
    esac
done

# Main execution
main() {
    print_header "🐳 Docker E2E Test Setup"
    
    # Check if we're in the right directory
    if [ ! -f "docker-compose.e2e.yml" ]; then
        print_colored $RED "❌ docker-compose.e2e.yml not found. Please run this script from the project root."
        exit 1
    fi
    
    # Load .env file if it exists
    if [ -f ".env" ]; then
        print_colored $GREEN "📁 Loading environment from .env file..."
        export $(grep -v '^#' .env | xargs)
    fi
    
    # Validate environment
    print_colored $BLUE "🔍 Validating environment variables..."
    validate_env
    print_colored $GREEN "✅ All required environment variables are set"
    
    # Clean up if requested
    if [ "$CLEAN" = true ]; then
        print_header "🧹 Cleaning up existing containers and volumes"
        docker-compose -f docker-compose.e2e.yml down -v --remove-orphans || true
        docker system prune -f || true
        print_colored $GREEN "✅ Cleanup completed"
    fi
    
    # Set environment for Docker Compose
    export KEEP_CONTAINER_RUNNING=$KEEP_RUNNING
    
    # Create test results directory
    mkdir -p test-results/artifacts
    
    print_header "🚀 Starting Docker E2E Tests"
    
    print_colored $BLUE "📋 Test Configuration:"
    print_colored $BLUE "   - Keep running after tests: $KEEP_RUNNING"
    print_colored $BLUE "   - Follow logs: $FOLLOW_LOGS"
    print_colored $BLUE "   - Test command: ${TEST_COMMAND:-'pnpm test:e2e:master (default)'}"
    print_colored $BLUE "   - Build args: ${BUILD_ARGS:-'none'}"
    
    # Function to run tests
    run_tests() {
        if [ -n "$TEST_COMMAND" ]; then
            docker-compose -f docker-compose.e2e.yml run --rm e2e-tests $TEST_COMMAND
        else
            docker-compose -f docker-compose.e2e.yml run --rm e2e-tests
        fi
    }
    
    # Function to follow logs
    follow_logs() {
        # Start services in background
        docker-compose -f docker-compose.e2e.yml up -d $BUILD_ARGS
        
        # Follow logs from both services
        print_colored $BLUE "📄 Following logs from both services..."
        docker-compose -f docker-compose.e2e.yml logs -f &
        LOGS_PID=$!
        
        # Wait for backend to be healthy
        print_colored $BLUE "⏳ Waiting for backend to be healthy..."
        timeout 120 bash -c 'until docker-compose -f docker-compose.e2e.yml ps backend | grep -q "healthy"; do sleep 2; done'
        
        # Run tests
        if [ -n "$TEST_COMMAND" ]; then
            docker-compose -f docker-compose.e2e.yml exec e2e-tests $TEST_COMMAND
        else
            docker-compose -f docker-compose.e2e.yml exec e2e-tests pnpm test:e2e:master
        fi
        
        # Stop following logs
        kill $LOGS_PID 2>/dev/null || true
    }
    
    # Execute tests
    if [ "$FOLLOW_LOGS" = true ]; then
        follow_logs
    else
        docker-compose -f docker-compose.e2e.yml up $BUILD_ARGS --abort-on-container-exit --exit-code-from e2e-tests
    fi
    
    TEST_EXIT_CODE=$?
    
    print_header "📊 Test Results"
    
    if [ $TEST_EXIT_CODE -eq 0 ]; then
        print_colored $GREEN "✅ E2E tests completed successfully!"
    else
        print_colored $RED "❌ E2E tests failed with exit code: $TEST_EXIT_CODE"
        
        print_colored $YELLOW "🔍 Debug information:"
        if [ -d "test-results/artifacts" ]; then
            print_colored $YELLOW "   - Screenshots available in: test-results/artifacts/"
            ls -la test-results/artifacts/ 2>/dev/null || true
        fi
        
        print_colored $YELLOW "📄 To view container logs:"
        print_colored $YELLOW "   docker-compose -f docker-compose.e2e.yml logs backend"
        print_colored $YELLOW "   docker-compose -f docker-compose.e2e.yml logs e2e-tests"
    fi
    
    # Cleanup unless keeping containers running
    if [ "$KEEP_RUNNING" = false ]; then
        print_colored $BLUE "🧹 Cleaning up containers..."
        docker-compose -f docker-compose.e2e.yml down
        print_colored $GREEN "✅ Cleanup completed"
    else
        print_colored $YELLOW "🔄 Containers are kept running for debugging"
        print_colored $YELLOW "   To clean up later: docker-compose -f docker-compose.e2e.yml down"
        print_colored $YELLOW "   To access test container: docker-compose -f docker-compose.e2e.yml exec e2e-tests bash"
    fi
    
    exit $TEST_EXIT_CODE
}

# Run main function
main