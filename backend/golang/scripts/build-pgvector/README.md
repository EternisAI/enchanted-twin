# pgvector Binary Builder

This directory contains scripts to build PostgreSQL with pgvector extension for embedded deployment.

## Quick Start

1. **Build pgvector binaries for all platforms:**
   ```bash
   ./build-pgvector-binaries.sh
   ```

2. **Build for specific platform:**
   ```bash
   docker buildx build \
     --platform linux/amd64 \
     --build-arg POSTGRES_VERSION=16.4 \
     --build-arg PGVECTOR_VERSION=0.7.4 \
     --target binaries \
     --output type=local,dest=./output \
     -f Dockerfile.pgvector-builder .
   ```

## Usage in Go Application

The pgvector binary manager automatically handles binary distribution:

```go
import "github.com/EternisAI/enchanted-twin/pkg/bootstrap/pgvector"

// Create binary manager
binaryManager := pgvector.NewBinaryManager(logger, "")

// Get pgvector-enabled binaries (automatically downloads if needed)
binariesPath, hasPgvector, err := binaryManager.GetBinariesPath(ctx)
if err != nil {
    log.Fatal(err)
}

if hasPgvector {
    log.Println("Using pgvector-enabled PostgreSQL")
    // Use binariesPath with embedded-postgres
} else {
    log.Println("Using standard PostgreSQL")
    // Fall back to standard embedded-postgres
}
```

## Integration with Bootstrap

The PostgreSQL bootstrap automatically uses pgvector when available:

```go
import "github.com/EternisAI/enchanted-twin/pkg/bootstrap"

// Bootstrap with pgvector support (automatic fallback)
server, err := bootstrap.BootstrapPostgresServer(ctx, logger, "5432", "./data")
if err != nil {
    log.Fatal(err)
}

// Check if pgvector is available
if server.HasPgvector() {
    log.Println("Vector search available!")
} else {
    log.Println("Standard PostgreSQL only")
}
```

## Build Configuration

### PostgreSQL Version
- Default: 16.4
- Configure: `--build-arg POSTGRES_VERSION=16.4`

### pgvector Version  
- Default: 0.7.4
- Configure: `--build-arg PGVECTOR_VERSION=0.7.4`

### Target Platforms
- linux/amd64
- linux/arm64  
- darwin/amd64
- darwin/arm64
- windows/amd64

## Output Structure

```
output/pgvector-binaries/
├── manifest.json                           # Build metadata
├── darwin-amd64/
│   ├── postgresql-16.4-pgvector0.7.4-darwin-amd64.tar.gz
│   └── postgresql-16.4-pgvector0.7.4-darwin-amd64.tar.gz.sha256
├── linux-amd64/
│   ├── postgresql-16.4-pgvector0.7.4-linux-amd64.tar.gz
│   └── postgresql-16.4-pgvector0.7.4-linux-amd64.tar.gz.sha256
└── release/
    ├── postgresql-16.4-pgvector0.7.4-*.tar.gz
    ├── checksums.sha256
    └── manifest.json
```

## Distribution Strategies

### 1. Bundle with Application
```
your-app/
├── app.exe
└── binaries/
    ├── linux-amd64/
    ├── darwin-amd64/
    └── windows-amd64/
```

### 2. Download on Demand
The binary manager automatically downloads the correct binaries for the current platform.

### 3. Hybrid (Recommended)
1. Check for bundled binaries
2. Check cache for downloaded binaries  
3. Download if needed
4. Fall back to standard PostgreSQL

## Build Requirements

- Docker with BuildKit support
- `docker buildx` for cross-platform builds
- `jq` for JSON processing (included in build container)

## Security

- All binaries include SHA256 checksums
- Portable compilation flags prevent architecture-specific optimizations that could cause compatibility issues
- Extracted files are validated for path traversal attacks

## Troubleshooting

### Build Fails
```bash
# Check Docker BuildKit
docker buildx version

# Check available platforms
docker buildx ls
```

### Download Fails
```bash
# Check network connectivity
curl -I https://github.com/EternisAI/pgvector-binaries/releases/latest

# Check cache directory permissions
ls -la ~/.cache/enchanted-twin/pgvector/
```

### pgvector Extension Not Found
This is expected when using standard PostgreSQL. The application automatically falls back to non-vector operations.

## Performance Notes

- pgvector binaries are compiled with portable flags for compatibility
- Vector operations support up to 2000 dimensions  
- HNSW and IVFFlat indexing available for performance
- Full PostgreSQL ACID compliance maintained