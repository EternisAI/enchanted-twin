#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_colored() {
    echo -e "${1}${2}${NC}"
}

print_header() {
    echo
    print_colored $BLUE "========================================"
    print_colored $BLUE "$1"
    print_colored $BLUE "========================================"
}

# Validation functions
check_docker() {
    print_colored $BLUE "🐳 Checking Docker installation..."
    
    if command -v docker &> /dev/null; then
        DOCKER_VERSION=$(docker --version)
        print_colored $GREEN "✅ Docker found: $DOCKER_VERSION"
        
        # Check if Docker daemon is running
        if docker info &> /dev/null; then
            print_colored $GREEN "✅ Docker daemon is running"
        else
            print_colored $RED "❌ Docker daemon is not running"
            print_colored $YELLOW "💡 Please start Docker Desktop or the Docker daemon"
            return 1
        fi
    else
        print_colored $RED "❌ Docker not found"
        print_colored $YELLOW "💡 Please install Docker from https://docker.com"
        return 1
    fi
}

check_docker_compose() {
    print_colored $BLUE "🔧 Checking Docker Compose..."
    
    # Check for docker-compose (standalone) or docker compose (plugin)
    if command -v docker-compose &> /dev/null; then
        COMPOSE_VERSION=$(docker-compose --version)
        print_colored $GREEN "✅ Docker Compose found: $COMPOSE_VERSION"
    elif docker compose version &> /dev/null; then
        COMPOSE_VERSION=$(docker compose version)
        print_colored $GREEN "✅ Docker Compose plugin found: $COMPOSE_VERSION"
    else
        print_colored $RED "❌ Docker Compose not found"
        print_colored $YELLOW "💡 Please install Docker Compose"
        return 1
    fi
}

check_project_files() {
    print_colored $BLUE "📁 Checking project files..."
    
    local required_files=(
        "docker-compose.e2e.yml"
        "app/Dockerfile.e2e"
        "backend/golang/Dockerfile"
        "app/docker/entrypoint-e2e.sh"
        "scripts/docker-e2e.sh"
    )
    
    local missing_files=()
    
    for file in "${required_files[@]}"; do
        if [ -f "$file" ]; then
            print_colored $GREEN "✅ Found: $file"
        else
            print_colored $RED "❌ Missing: $file"
            missing_files+=("$file")
        fi
    done
    
    if [ ${#missing_files[@]} -ne 0 ]; then
        print_colored $RED "❌ Missing required files for Docker e2e setup"
        return 1
    fi
}

check_environment() {
    print_colored $BLUE "🔐 Checking environment variables..."
    
    local env_file_exists=false
    if [ -f ".env" ]; then
        env_file_exists=true
        print_colored $GREEN "✅ Found .env file"
        # Load environment variables from .env
        export $(grep -v '^#' .env | xargs) 2>/dev/null || true
    else
        print_colored $YELLOW "⚠️  No .env file found"
    fi
    
    local required_vars=(
        "E2E_TEST_EMAIL"
        "E2E_TEST_PASSWORD"
        "VITE_FIREBASE_API_KEY"
        "VITE_FIREBASE_AUTH_DOMAIN"
        "VITE_FIREBASE_PROJECT_ID"
    )
    
    local api_key_vars=(
        "COMPLETIONS_API_KEY"
        "OPENROUTER_API_KEY"
        "EMBEDDINGS_API_KEY"
        "OPENAI_API_KEY"
    )
    
    local missing_vars=()
    local has_completion_key=false
    local has_embedding_key=false
    
    # Check required vars
    for var in "${required_vars[@]}"; do
        if [ -z "${!var}" ]; then
            missing_vars+=("$var")
        else
            print_colored $GREEN "✅ $var is set"
        fi
    done
    
    # Check API keys (at least one from each group needed)
    if [ -n "$COMPLETIONS_API_KEY" ] || [ -n "$OPENROUTER_API_KEY" ]; then
        has_completion_key=true
        print_colored $GREEN "✅ Completions API key is available"
    fi
    
    if [ -n "$EMBEDDINGS_API_KEY" ] || [ -n "$OPENAI_API_KEY" ]; then
        has_embedding_key=true
        print_colored $GREEN "✅ Embeddings API key is available"
    fi
    
    if [ ! "$has_completion_key" = true ]; then
        missing_vars+=("COMPLETIONS_API_KEY or OPENROUTER_API_KEY")
    fi
    
    if [ ! "$has_embedding_key" = true ]; then
        missing_vars+=("EMBEDDINGS_API_KEY or OPENAI_API_KEY")
    fi
    
    if [ ${#missing_vars[@]} -ne 0 ]; then
        print_colored $RED "❌ Missing required environment variables:"
        for var in "${missing_vars[@]}"; do
            print_colored $RED "   - $var"
        done
        echo
        if [ "$env_file_exists" = false ]; then
            print_colored $YELLOW "💡 Create a .env file in the project root with the required variables"
            print_colored $YELLOW "   Example .env content:"
            cat << EOF

E2E_TEST_EMAIL=your-test-email@gmail.com
E2E_TEST_PASSWORD=your-test-password
VITE_FIREBASE_API_KEY=your-firebase-api-key
VITE_FIREBASE_AUTH_DOMAIN=your-project.firebaseapp.com
VITE_FIREBASE_PROJECT_ID=your-project-id
COMPLETIONS_API_KEY=your-openrouter-or-openai-key
EMBEDDINGS_API_KEY=your-openai-key

EOF
        fi
        return 1
    fi
}

check_system_resources() {
    print_colored $BLUE "💻 Checking system resources..."
    
    # Check available disk space (need at least 2GB for images)
    local available_space
    if command -v df &> /dev/null; then
        available_space=$(df -BG . | awk 'NR==2 {print $4}' | sed 's/G//')
        if [ "$available_space" -gt 2 ]; then
            print_colored $GREEN "✅ Sufficient disk space available: ${available_space}GB"
        else
            print_colored $YELLOW "⚠️  Low disk space: ${available_space}GB (recommended: 2GB+)"
        fi
    fi
    
    # Check available memory
    if command -v free &> /dev/null; then
        local available_mem=$(free -m | awk 'NR==2{printf "%.0f", $7/1024}')
        if [ "$available_mem" -gt 1 ]; then
            print_colored $GREEN "✅ Sufficient memory available: ${available_mem}GB"
        else
            print_colored $YELLOW "⚠️  Low memory available: ${available_mem}GB (recommended: 2GB+)"
        fi
    fi
}

test_docker_build() {
    print_colored $BLUE "🔨 Testing Docker builds (dry run)..."
    
    # Test backend Dockerfile
    if docker build -f backend/golang/Dockerfile --target builder backend/golang -t enchanted-backend-test:latest &> /dev/null; then
        print_colored $GREEN "✅ Backend Dockerfile syntax is valid"
        docker rmi enchanted-backend-test:latest &> /dev/null || true
    else
        print_colored $RED "❌ Backend Dockerfile has issues"
        return 1
    fi
    
    # Test if we can parse docker-compose.e2e.yml
    if docker-compose -f docker-compose.e2e.yml config &> /dev/null; then
        print_colored $GREEN "✅ Docker Compose configuration is valid"
    else
        print_colored $RED "❌ Docker Compose configuration has issues"
        return 1
    fi
}

# Main validation function
main() {
    print_header "🔍 Docker E2E Setup Validation"
    
    local checks_passed=true
    
    # Run all checks
    check_docker || checks_passed=false
    check_docker_compose || checks_passed=false
    check_project_files || checks_passed=false
    check_environment || checks_passed=false
    check_system_resources || checks_passed=false
    test_docker_build || checks_passed=false
    
    print_header "📊 Validation Results"
    
    if [ "$checks_passed" = true ]; then
        print_colored $GREEN "✅ All checks passed! Your system is ready for Docker E2E testing."
        echo
        print_colored $BLUE "🚀 To run E2E tests:"
        print_colored $BLUE "   ./scripts/docker-e2e.sh"
        echo
        print_colored $BLUE "🔍 To run with debugging:"
        print_colored $BLUE "   ./scripts/docker-e2e.sh --keep-running --logs"
        echo
        print_colored $BLUE "📚 For more options:"
        print_colored $BLUE "   ./scripts/docker-e2e.sh --help"
    else
        print_colored $RED "❌ Some checks failed. Please address the issues above before running Docker E2E tests."
        exit 1
    fi
}

# Run main function
main