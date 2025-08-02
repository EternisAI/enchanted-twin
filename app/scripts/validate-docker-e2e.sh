#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🔍 Docker E2E Setup Validation${NC}"
echo "============================================"

# Check if we're in the right directory
if [ ! -f "package.json" ]; then
    echo -e "${RED}❌ Error: Run this script from the app directory${NC}"
    exit 1
fi

ERRORS=0
WARNINGS=0

check_file() {
    if [ -f "$1" ]; then
        echo -e "${GREEN}✅ $1${NC}"
    else
        echo -e "${RED}❌ Missing: $1${NC}"
        ((ERRORS++))
    fi
}

check_executable() {
    if [ -x "$1" ]; then
        echo -e "${GREEN}✅ $1 (executable)${NC}"
    else
        echo -e "${YELLOW}⚠️  $1 (not executable)${NC}"
        echo "   Run: chmod +x $1"
        ((WARNINGS++))
    fi
}

check_command() {
    if command -v "$1" &> /dev/null; then
        echo -e "${GREEN}✅ $1 installed${NC}"
    else
        echo -e "${RED}❌ Missing: $1${NC}"
        ((ERRORS++))
    fi
}

echo ""
echo "📁 Checking required files..."
check_file "Dockerfile.e2e"
check_file "docker-compose.e2e.yml"
check_file "docker/entrypoint-e2e.sh"
check_file "scripts/docker-e2e.sh"
check_file "tests/e2e/env.docker.example"
check_file "../backend/golang/Dockerfile.backend"

echo ""
echo "🔧 Checking executables..."
check_executable "scripts/docker-e2e.sh"
check_executable "docker/entrypoint-e2e.sh"

echo ""
echo "💻 Checking system requirements..."
check_command "docker"
check_command "docker-compose"
check_command "pnpm"

echo ""
echo "⚙️  Checking configuration..."
if [ -f "tests/e2e/.env" ]; then
    echo -e "${GREEN}✅ tests/e2e/.env exists${NC}"
    
    # Check for required environment variables
    required_vars=("COMPLETIONS_API_KEY" "OPENAI_API_KEY" "FIREBASE_API_KEY" "FIREBASE_AUTH_DOMAIN" "FIREBASE_PROJECT_ID")
    
    for var in "${required_vars[@]}"; do
        if grep -q "^${var}=" tests/e2e/.env && ! grep -q "^${var}=your_" tests/e2e/.env; then
            echo -e "${GREEN}✅ $var configured${NC}"
        else
            echo -e "${YELLOW}⚠️  $var not configured${NC}"
            ((WARNINGS++))
        fi
    done
else
    echo -e "${YELLOW}⚠️  tests/e2e/.env missing${NC}"
    echo "   Run: cp tests/e2e/env.docker.example tests/e2e/.env"
    echo "   Then edit with your API keys"
    ((WARNINGS++))
fi

echo ""
echo "📦 Checking package.json scripts..."
if grep -q "docker:e2e:setup" package.json; then
    echo -e "${GREEN}✅ Docker E2E scripts added to package.json${NC}"
else
    echo -e "${RED}❌ Docker E2E scripts missing from package.json${NC}"
    ((ERRORS++))
fi

echo ""
echo "============================================"

if [ $ERRORS -eq 0 ] && [ $WARNINGS -eq 0 ]; then
    echo -e "${GREEN}🎉 Setup validation passed! You're ready to run:${NC}"
    echo -e "${BLUE}   pnpm run docker:e2e:setup${NC}"
    echo -e "${BLUE}   pnpm run docker:e2e:test${NC}"
elif [ $ERRORS -eq 0 ]; then
    echo -e "${YELLOW}⚠️  Setup validation passed with $WARNINGS warning(s)${NC}"
    echo -e "${YELLOW}   Address warnings above, then run:${NC}"
    echo -e "${BLUE}   pnpm run docker:e2e:setup${NC}"
else
    echo -e "${RED}❌ Setup validation failed with $ERRORS error(s) and $WARNINGS warning(s)${NC}"
    echo -e "${RED}   Fix errors above before proceeding${NC}"
    exit 1
fi

echo ""
echo "📖 For detailed instructions, see: DOCKER_E2E.md" 