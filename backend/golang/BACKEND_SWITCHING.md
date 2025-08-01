# Memory Backend Switching

This document explains how to switch between PostgreSQL and Weaviate memory backends to ensure behavioral equivalence.

## Quick Start

### Use PostgreSQL Backend (Default)
```bash
export MEMORY_BACKEND=postgresql
export POSTGRES_PORT=5432
make run
```

### Use Weaviate Backend
```bash
export MEMORY_BACKEND=weaviate
export WEAVIATE_PORT=51414
make run
```

## Configuration Options

### Environment Variables

| Variable | Description | Default | Valid Values |
|----------|-------------|---------|--------------|
| `MEMORY_BACKEND` | Memory storage backend | `postgresql` | `postgresql`, `weaviate` |
| `POSTGRES_PORT` | PostgreSQL server port | `5432` | Any available port |
| `WEAVIATE_PORT` | Weaviate server port | `51414` | Any available port |

### Backend Features

#### PostgreSQL + pgvector
- **Default backend** - Used when `MEMORY_BACKEND=postgresql` or unset
- **Vector Search**: Uses pgvector extension with cosine, L2, and inner product similarity
- **Performance**: Native SQL queries with proper indexing
- **Storage**: Embedded PostgreSQL server with schema migrations
- **Fallback**: Intelligent fallback to standard PostgreSQL when pgvector unavailable

#### Weaviate
- **Legacy backend** - Used when `MEMORY_BACKEND=weaviate`
- **Vector Search**: Native Weaviate vector database
- **Performance**: GraphQL-based queries with vector similarity
- **Storage**: Embedded Weaviate server with schema management
- **Schema**: Automatic schema creation and validation

## Testing Behavioral Equivalence

### Basic Functionality Test
Both backends implement the same `storage.Interface` and should behave identically:

```bash
# Test PostgreSQL
MEMORY_BACKEND=postgresql make test

# Test Weaviate  
MEMORY_BACKEND=weaviate make test
```

### Development Workflow
1. **Develop with PostgreSQL** (default) for faster iteration
2. **Test with Weaviate** to ensure compatibility
3. **Compare results** for identical behavior
4. **Use integration tests** with both backends

### Switching Backends
To switch backends without data loss:

1. **Export data** from current backend (if needed)
2. **Change environment variable**:
   ```bash
   export MEMORY_BACKEND=weaviate  # or postgresql
   ```
3. **Restart server**: `make run`
4. **Import data** to new backend (if needed)

## Implementation Details

### Backend Selection
The backend is selected in `cmd/server/main.go` based on the `MEMORY_BACKEND` environment variable:

```go
switch envs.MemoryBackend {
case "postgresql":
    storageInterface, err = createPostgreSQLStorage(...)
case "weaviate":
    storageInterface, err = createWeaviateStorage(...)
default:
    logger.Fatal("Unsupported memory backend", "backend", envs.MemoryBackend)
}
```

### Storage Interface
Both backends implement the same interface defined in `storage.Interface`:

```go
type Interface interface {
    GetByID(ctx context.Context, id string) (*memory.MemoryFact, error)
    Update(ctx context.Context, id string, fact *memory.MemoryFact, vector []float32) error
    Delete(ctx context.Context, id string) error
    StoreBatch(ctx context.Context, objects []*models.Object) error
    Query(ctx context.Context, queryText string, filter *memory.Filter) (memory.QueryResult, error)
    // ... additional methods
}
```

### Data Compatibility
Both backends store the same memory fact structure:
- **Content**: Text content of memory facts
- **Vectors**: Embedding vectors for semantic search
- **Metadata**: Structured fact fields (category, subject, attribute, etc.)
- **References**: Document references and relationships
- **Timestamps**: Creation and modification times

## Troubleshooting

### Port Conflicts
If you encounter port conflicts:
```bash
# Use different ports
export POSTGRES_PORT=5433
export WEAVIATE_PORT=51415
```

### Backend Not Found
If you see "Unsupported memory backend":
```bash
# Check your environment variable
echo $MEMORY_BACKEND

# Set to a valid backend
export MEMORY_BACKEND=postgresql  # or weaviate
```

### Schema Issues
If you encounter schema problems:
```bash
# Clear data directories
rm -rf ./output/postgres ./output/weaviate

# Restart with fresh schema
make run
```

## Migration Notes

### From Weaviate to PostgreSQL
The migration has been completed and PostgreSQL is now the default. The Weaviate backend is maintained for:
- **Compatibility testing**
- **Legacy support**
- **Behavioral validation**

### Future Considerations
- **Weaviate backend** may be deprecated in future versions
- **PostgreSQL backend** is the recommended choice for new deployments
- **Data migration utilities** can be developed if needed for production systems

## Examples

### Development Environment
```bash
# Fast development with PostgreSQL
export MEMORY_BACKEND=postgresql
export POSTGRES_PORT=5432
make run
```

### Testing Environment
```bash
# Test with both backends
export MEMORY_BACKEND=postgresql make test
export MEMORY_BACKEND=weaviate make test
```

### Production Environment
```bash
# Recommended production setup
export MEMORY_BACKEND=postgresql
export POSTGRES_PORT=5432
# Add other production environment variables
make run
```