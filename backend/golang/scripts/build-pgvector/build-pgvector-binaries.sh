#!/bin/bash

# build-pgvector-binaries.sh
# Builds PostgreSQL binaries with pgvector extension for multiple platforms

set -euo pipefail

# Configuration
POSTGRES_VERSION="16.4"
PGVECTOR_VERSION="0.7.4"
OUTPUT_DIR="$(pwd)/output/pgvector-binaries"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

# Platform configurations
declare -A PLATFORMS=(
    ["linux/amd64"]="linux-amd64"
    ["linux/arm64"]="linux-arm64"
    ["darwin/amd64"]="darwin-amd64"
    ["darwin/arm64"]="darwin-arm64"
    ["windows/amd64"]="windows-amd64"
)

# Check dependencies
check_dependencies() {
    log_info "Checking dependencies..."
    
    if ! command -v docker &> /dev/null; then
        log_error "Docker is required but not installed"
        exit 1
    fi
    
    if ! docker buildx version &> /dev/null; then
        log_error "Docker Buildx is required but not available"
        exit 1
    fi
    
    log_success "All dependencies found"
}

# Create output directory
setup_output_dir() {
    log_info "Setting up output directory: $OUTPUT_DIR"
    mkdir -p "$OUTPUT_DIR"
    
    # Create manifest file
    cat > "$OUTPUT_DIR/manifest.json" << EOF
{
    "version": "${POSTGRES_VERSION}-pgvector${PGVECTOR_VERSION}",
    "postgres_version": "${POSTGRES_VERSION}",
    "pgvector_version": "${PGVECTOR_VERSION}",
    "build_date": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
    "binaries": {}
}
EOF
}

# Build binaries for a specific platform
build_platform() {
    local platform="$1"
    local platform_name="${PLATFORMS[$platform]}"
    local os_arch=(${platform//\// })
    local target_os="${os_arch[0]}"
    local target_arch="${os_arch[1]}"
    
    log_info "Building binaries for $platform_name ($platform)..."
    
    # Create platform-specific output directory
    local platform_output="$OUTPUT_DIR/$platform_name"
    mkdir -p "$platform_output"
    
    # Build Docker image and extract binaries
    local image_name="pgvector-builder:${platform_name}"
    
    log_info "Building Docker image for $platform_name..."
    docker buildx build \
        --platform "$platform" \
        --build-arg POSTGRES_VERSION="$POSTGRES_VERSION" \
        --build-arg PGVECTOR_VERSION="$PGVECTOR_VERSION" \
        --build-arg TARGETOS="$target_os" \
        --build-arg TARGETARCH="$target_arch" \
        --target binaries \
        --output type=local,dest="$platform_output" \
        -f "$SCRIPT_DIR/Dockerfile.pgvector-builder" \
        "$SCRIPT_DIR"
    
    # Find the generated tar.gz file
    local archive_file=$(find "$platform_output" -name "*.tar.gz" | head -1)
    if [[ -z "$archive_file" ]]; then
        log_error "No archive file found for $platform_name"
        return 1
    fi
    
    # Rename to standard format
    local standard_name="postgresql-${POSTGRES_VERSION}-pgvector${PGVECTOR_VERSION}-${platform_name}.tar.gz"
    mv "$archive_file" "$platform_output/$standard_name"
    
    # Generate checksum
    local checksum=$(sha256sum "$platform_output/$standard_name" | cut -d' ' -f1)
    echo "$checksum" > "$platform_output/$standard_name.sha256"
    
    # Get file size
    local file_size=$(stat -c%s "$platform_output/$standard_name" 2>/dev/null || stat -f%z "$platform_output/$standard_name" 2>/dev/null || echo "0")
    
    # Update manifest
    local temp_manifest=$(mktemp)
    jq --arg platform "$platform_name" \
       --arg url "https://github.com/EternisAI/pgvector-binaries/releases/download/v${POSTGRES_VERSION}-pgvector${PGVECTOR_VERSION}/$standard_name" \
       --arg checksum "$checksum" \
       --arg size "$file_size" \
       '.binaries[$platform] = {
           "url": $url,
           "sha256": $checksum,
           "size": ($size | tonumber),
           "filename": "'"$standard_name"'"
       }' "$OUTPUT_DIR/manifest.json" > "$temp_manifest"
    mv "$temp_manifest" "$OUTPUT_DIR/manifest.json"
    
    log_success "Built binaries for $platform_name (size: $(numfmt --to=iec $file_size), checksum: ${checksum:0:8}...)"
}

# Build all platforms
build_all_platforms() {
    local failed_platforms=()
    
    for platform in "${!PLATFORMS[@]}"; do
        if build_platform "$platform"; then
            log_success "Successfully built ${PLATFORMS[$platform]}"
        else
            log_error "Failed to build ${PLATFORMS[$platform]}"
            failed_platforms+=("${PLATFORMS[$platform]}")
        fi
        echo # Add spacing between platforms
    done
    
    # Report results
    if [[ ${#failed_platforms[@]} -eq 0 ]]; then
        log_success "All platforms built successfully!"
    else
        log_warn "Failed platforms: ${failed_platforms[*]}"
        return 1
    fi
}

# Create release packages
create_release_packages() {
    log_info "Creating release packages..."
    
    local release_dir="$OUTPUT_DIR/release"
    mkdir -p "$release_dir"
    
    # Copy all binaries to release directory
    for platform_name in "${PLATFORMS[@]}"; do
        local platform_dir="$OUTPUT_DIR/$platform_name"
        if [[ -d "$platform_dir" ]]; then
            cp "$platform_dir"/*.tar.gz "$release_dir/" 2>/dev/null || true
            cp "$platform_dir"/*.sha256 "$release_dir/" 2>/dev/null || true
        fi
    done
    
    # Copy manifest
    cp "$OUTPUT_DIR/manifest.json" "$release_dir/"
    
    # Create checksums file
    cd "$release_dir"
    sha256sum *.tar.gz > checksums.sha256
    cd - > /dev/null
    
    log_success "Release packages created in $release_dir"
}

# Generate usage instructions
generate_usage_instructions() {
    local readme_file="$OUTPUT_DIR/README.md"
    
    cat > "$readme_file" << EOF
# PostgreSQL with pgvector Binaries

This directory contains pre-built PostgreSQL ${POSTGRES_VERSION} binaries with pgvector ${PGVECTOR_VERSION} extension.

## Available Platforms

$(for platform in "${PLATFORMS[@]}"; do echo "- $platform"; done)

## Usage

### With Go embedded-postgres

\`\`\`go
import (
    "github.com/EternisAI/enchanted-twin/pkg/bootstrap/pgvector"
)

// Create binary manager
binaryManager := pgvector.NewBinaryManager(logger, "")

// Get pgvector-enabled binaries
binariesPath, hasPgvector, err := binaryManager.GetBinariesPath(ctx)
if err != nil {
    log.Fatal(err)
}

if hasPgvector {
    log.Println("Using pgvector-enabled PostgreSQL")
} else {
    log.Println("Using standard PostgreSQL")
}
\`\`\`

### Manual Installation

1. Download the appropriate binary for your platform
2. Extract: \`tar -xzf postgresql-${POSTGRES_VERSION}-pgvector${PGVECTOR_VERSION}-<platform>.tar.gz\`
3. Use the extracted \`bin/\` directory as your PostgreSQL installation

## Verification

All binaries include SHA256 checksums for verification:

\`\`\`bash
sha256sum -c postgresql-${POSTGRES_VERSION}-pgvector${PGVECTOR_VERSION}-<platform>.tar.gz.sha256
\`\`\`

## Build Information

- PostgreSQL Version: ${POSTGRES_VERSION}
- pgvector Version: ${PGVECTOR_VERSION}
- Build Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)
- Built with portable compilation flags for maximum compatibility

## pgvector Features

- Vector similarity search (cosine, L2, inner product)
- Support for up to 2000 dimensions
- HNSW and IVFFlat indexing for performance
- Full ACID compliance with PostgreSQL
EOF

    log_success "Usage instructions created: $readme_file"
}

# Cleanup function
cleanup() {
    log_info "Cleaning up temporary files..."
    # Add any cleanup logic here
}

# Main execution
main() {
    log_info "Starting pgvector binary build process..."
    log_info "PostgreSQL Version: $POSTGRES_VERSION"
    log_info "pgvector Version: $PGVECTOR_VERSION"
    log_info "Output Directory: $OUTPUT_DIR"
    
    # Set up cleanup trap
    trap cleanup EXIT
    
    # Execute build steps
    check_dependencies
    setup_output_dir
    build_all_platforms
    create_release_packages
    generate_usage_instructions
    
    log_success "Build process completed successfully!"
    log_info "Binaries available in: $OUTPUT_DIR/release"
    log_info "Upload these files to GitHub releases for distribution"
}

# Handle command line arguments
case "${1:-}" in
    --help|-h)
        echo "Usage: $0 [--help] [--clean]"
        echo "Build PostgreSQL with pgvector extension for multiple platforms"
        echo ""
        echo "Options:"
        echo "  --help    Show this help message"
        echo "  --clean   Clean output directory before building"
        exit 0
        ;;
    --clean)
        log_info "Cleaning output directory..."
        rm -rf "$OUTPUT_DIR"
        ;;
esac

# Run main function
main "$@"