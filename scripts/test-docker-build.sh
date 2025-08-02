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

# Test step 1: Check Docker context size
test_context_size() {
    print_header "1. Testing Docker Build Context Size"
    
    cd backend/golang
    
    print_colored $BLUE "📊 Checking build context size..."
    
    # Create a temporary build context to check size
    context_size=$(du -sh . | cut -f1)
    print_colored $GREEN "✅ Build context size: $context_size"
    
    # Check if .dockerignore is working
    if [ -f ".dockerignore" ]; then
        print_colored $GREEN "✅ .dockerignore file exists"
        
        # Test what gets ignored
        print_colored $BLUE "📁 Files being ignored:"
        if command -v docker &> /dev/null; then
            docker build --dry-run -f Dockerfile . 2>&1 | grep "transferring context" || true
        else
            print_colored $YELLOW "⚠️  Docker not available for context size test"
        fi
    else
        print_colored $RED "❌ .dockerignore file missing"
        return 1
    fi
    
    cd ../..
}

# Test step 2: Backend Dockerfile syntax
test_backend_dockerfile() {
    print_header "2. Testing Backend Dockerfile Syntax"
    
    if ! command -v docker &> /dev/null; then
        print_colored $RED "❌ Docker not installed"
        return 1
    fi
    
    cd backend/golang
    
    print_colored $BLUE "🔍 Validating Dockerfile syntax..."
    if docker build --dry-run -f Dockerfile . &> /dev/null; then
        print_colored $GREEN "✅ Dockerfile syntax is valid"
    else
        print_colored $RED "❌ Dockerfile syntax has issues"
        print_colored $YELLOW "📋 Error details:"
        docker build --dry-run -f Dockerfile . 2>&1 | head -20
        cd ../..
        return 1
    fi
    
    cd ../..
}

# Test step 3: Attempt backend build (first stage only)
test_backend_build_stage1() {
    print_header "3. Testing Backend Build (Dependencies Only)"
    
    if ! command -v docker &> /dev/null; then
        print_colored $RED "❌ Docker not installed - skipping build test"
        return 1
    fi
    
    cd backend/golang
    
    print_colored $BLUE "🔨 Testing Go dependency download stage..."
    
    # Build only up to dependency download to test quickly
    if docker build --target builder -f Dockerfile -t enchanted-backend-deps-test . --quiet; then
        print_colored $GREEN "✅ Go dependencies downloaded successfully"
        
        # Clean up test image
        docker rmi enchanted-backend-deps-test &> /dev/null || true
    else
        print_colored $RED "❌ Go dependency download failed"
        print_colored $YELLOW "📋 Error details:"
        docker build --target builder -f Dockerfile -t enchanted-backend-deps-test . 2>&1 | tail -20
        cd ../..
        return 1
    fi
    
    cd ../..
}

# Test step 4: Frontend Dockerfile
test_frontend_dockerfile() {
    print_header "4. Testing Frontend E2E Dockerfile"
    
    if ! command -v docker &> /dev/null; then
        print_colored $RED "❌ Docker not installed - skipping frontend test"
        return 1
    fi
    
    cd app
    
    print_colored $BLUE "🔍 Validating E2E Dockerfile syntax..."
    if docker build --dry-run -f Dockerfile.e2e . &> /dev/null; then
        print_colored $GREEN "✅ E2E Dockerfile syntax is valid"
    else
        print_colored $RED "❌ E2E Dockerfile syntax has issues"
        print_colored $YELLOW "📋 Error details:"
        docker build --dry-run -f Dockerfile.e2e . 2>&1 | head -20
        cd ..
        return 1
    fi
    
    cd ..
}

# Test step 5: Docker Compose validation
test_docker_compose() {
    print_header "5. Testing Docker Compose Configuration"
    
    if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
        print_colored $RED "❌ Docker Compose not installed"
        return 1
    fi
    
    print_colored $BLUE "🔍 Validating docker-compose.e2e.yml..."
    
    # Use docker-compose or docker compose depending on what's available
    COMPOSE_CMD="docker-compose"
    if ! command -v docker-compose &> /dev/null; then
        COMPOSE_CMD="docker compose"
    fi
    
    if $COMPOSE_CMD -f docker-compose.e2e.yml config &> /dev/null; then
        print_colored $GREEN "✅ Docker Compose configuration is valid"
        
        print_colored $BLUE "📋 Services configured:"
        $COMPOSE_CMD -f docker-compose.e2e.yml config --services
    else
        print_colored $RED "❌ Docker Compose configuration has issues"
        print_colored $YELLOW "📋 Error details:"
        $COMPOSE_CMD -f docker-compose.e2e.yml config 2>&1 | head -10
        return 1
    fi
}

# Test step 6: Full backend build (if requested)
test_full_backend_build() {
    print_header "6. Testing Full Backend Build (Optional)"
    
    if ! command -v docker &> /dev/null; then
        print_colored $RED "❌ Docker not installed - skipping full build test"
        return 1
    fi
    
    read -p "$(echo -e ${YELLOW}⚠️  This will take 2-5 minutes. Continue with full backend build test? [y/N]: ${NC})" -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_colored $YELLOW "⏭️  Skipping full backend build test"
        return 0
    fi
    
    cd backend/golang
    
    print_colored $BLUE "🔨 Building full backend image..."
    print_colored $YELLOW "⏳ This may take several minutes..."
    
    if docker build -f Dockerfile -t enchanted-backend-test . --no-cache; then
        print_colored $GREEN "✅ Full backend build successful!"
        
        # Test if the binary was created correctly
        print_colored $BLUE "🔍 Testing binary creation..."
        if docker run --rm enchanted-backend-test ls -la /app/server; then
            print_colored $GREEN "✅ Server binary created successfully"
        else
            print_colored $YELLOW "⚠️  Could not verify binary"
        fi
        
        # Clean up test image
        docker rmi enchanted-backend-test &> /dev/null || true
    else
        print_colored $RED "❌ Full backend build failed"
        cd ../..
        return 1
    fi
    
    cd ../..
}

# Main test runner
main() {
    print_header "🔍 Docker Build Verification Tests"
    
    local tests_passed=0
    local total_tests=0
    
    # Run tests
    echo
    if test_context_size; then
        ((tests_passed++))
    fi
    ((total_tests++))
    
    echo
    if test_backend_dockerfile; then
        ((tests_passed++))
    fi
    ((total_tests++))
    
    echo
    if test_backend_build_stage1; then
        ((tests_passed++))
    fi
    ((total_tests++))
    
    echo
    if test_frontend_dockerfile; then
        ((tests_passed++))
    fi
    ((total_tests++))
    
    echo
    if test_docker_compose; then
        ((tests_passed++))
    fi
    ((total_tests++))
    
    # Optional full build test
    echo
    if test_full_backend_build; then
        ((tests_passed++))
        ((total_tests++))
    fi
    
    print_header "📊 Test Results"
    
    print_colored $BLUE "Tests passed: $tests_passed/$total_tests"
    
    if [ $tests_passed -eq $total_tests ]; then
        print_colored $GREEN "✅ All tests passed! Docker setup should work correctly."
        echo
        print_colored $BLUE "🚀 Ready to run full e2e tests:"
        print_colored $BLUE "   pnpm test:e2e:docker"
        exit 0
    else
        print_colored $RED "❌ Some tests failed. Please check the errors above."
        exit 1
    fi
}

# Run tests
main