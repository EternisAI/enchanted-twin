# Evolving Memory Package

## What is this?

This package stores and retrieves user memories using hot-swappable storage backends (currently Weaviate). It processes documents, extracts facts using LLMs, and manages memory updates intelligently through a clean 3-layer architecture.

## Architecture Overview

The package follows clean architecture principles with clear separation of concerns:

```
┌─────────────────┐    ┌──────────────────────┐    ┌─────────────────┐
│   StorageImpl   │───▶│ MemoryOrchestrator   │───▶│  MemoryEngine   │
│ (Public API)    │    │   (Coordination)     │    │ (Business Logic)│
└─────────────────┘    └──────────────────────┘    └─────────────────┘
         │                       │                        │
         │                       │                        ▼
         │                       ▼               ┌──────────────────┐
         │              ┌──────────────────┐     │ storage.Interface│ ← HOT-SWAPPABLE! |                  |     |                  |
         │              │   Channels &     │     └──────────────────┘
         │              │   Workers &      │             │
         │              │   Progress       │             ▼
         │              └──────────────────┘    ┌─────────────────┐
         │                                      │ WeaviateStorage │
         ▼ (Clean Public Interface)             │ RedisStorage    │
   ┌─────────────────┐                          │ PostgresStorage │
   │  MemoryStorage  │                          └─────────────────┘
   │   (Interface)   │
   └─────────────────┘
```

### Layer Responsibilities

1. **StorageImpl** - Thin public API maintaining interface compatibility
2. **MemoryOrchestrator** - Infrastructure concerns (workers, channels, batching, timeouts)
3. **MemoryEngine** - Pure business logic (fact extraction, memory decisions)
4. **storage.Interface** - Hot-swappable storage abstraction

## Quick Start

```go
// Create storage instance with hot-swappable backend
storage, err := evolvingmemory.New(evolvingmemory.Dependencies{
    Logger:             logger,
    Storage:            storageBackend,        // Any storage.Interface implementation
    CompletionsService: completionsAI,
    EmbeddingsService:  embeddingsAI,         // Can be different from completions
})
if err != nil {
    log.Fatal("Failed to create storage:", err)
}

docs := []memory.Document{
    &memory.TextDocument{...},           // Simple text
    &memory.ConversationDocument{...},   // Chat conversations
}

// Backward compatible API (still works)
err := storage.Store(ctx, docs, func(processed, total int) {
    log.Printf("Progress: %d/%d", processed, total)
})

// New channel-based API (recommended)
config := evolvingmemory.DefaultConfig()
progressCh, errorCh := storage.StoreV2(ctx, docs, config)

// Process results
for progressCh != nil || errorCh != nil {
    select {
    case progress, ok := <-progressCh:
        if !ok { progressCh = nil; continue }
        log.Printf("Progress: %d/%d", progress.Processed, progress.Total)
    case err, ok := <-errorCh:
        if !ok { errorCh = nil; continue }
        log.Printf("Error: %v", err)
    }
}

// Query with advanced filtering (NEW!)
filter := &memory.Filter{
    Source:      stringPtr("conversations"),
    ContactName: stringPtr("alice"),
    Distance:    0.7,  // Max semantic distance
    Limit:       intPtr(10),
}
result, err := storage.Query(ctx, "work discussions", filter)
```

## How does it work?

### The Flow

1. **Documents come in** → Text documents or conversation transcripts
2. **Extract facts** → MemoryEngine uses LLM to pull out interesting facts
3. **Check existing memories** → Search storage for similar facts
4. **Decide what to do** → LLM decides: ADD new memory, UPDATE existing, DELETE outdated, or NONE
5. **Execute decision** → MemoryEngine executes immediately (UPDATE/DELETE) or batches (ADD)
6. **Store in backend** → MemoryOrchestrator coordinates batched inserts via storage.Interface

### Key Concepts

**Document Types:**
- `TextDocument` - Basic text with metadata (emails, notes, etc.)
- `ConversationDocument` - Structured chats with multiple speakers

**Memory Operations:**
- `ADD` - Create a new memory (batched for efficiency)
- `UPDATE` - Replace an existing memory's content (immediate)
- `DELETE` - Remove an outdated memory (immediate)
- `NONE` - Do nothing (fact isn't worth remembering)

**Speaker Rules:**
- Document-level memories can't modify speaker-specific ones
- Speakers can only modify their own memories
- Validation happens before any UPDATE/DELETE

**Hot-Swappable Storage:**
- Depends on `storage.Interface`, not specific implementations
- Currently supports WeaviateStorage
- Easy to add RedisStorage, PostgresStorage, etc.

## Advanced Filtering

The memory system now supports powerful filtering capabilities for precise memory retrieval:

### Filter Structure

```go
type Filter struct {
    Source      *string // Filter by document source
    ContactName *string // Filter by contact/speaker name  
    Distance    float32 // Maximum semantic distance (0 = disabled)
    Limit       *int    // Maximum number of results to return
}
```

### Usage Examples

```go
// Filter by source
filter := &memory.Filter{
    Source: stringPtr("email"),
}
result, err := storage.Query(ctx, "work meetings", filter)

// Filter by contact
filter := &memory.Filter{
    ContactName: stringPtr("alice"),
}
result, err := storage.Query(ctx, "alice's preferences", filter)

// Limit results with semantic distance
filter := &memory.Filter{
    Distance: 0.7,
    Limit:    intPtr(5),
}
result, err := storage.Query(ctx, "recent activities", filter)

// Combined filtering
filter := &memory.Filter{
    Source:      stringPtr("conversations"),
    ContactName: stringPtr("bob"),
    Distance:    0.8,
    Limit:       intPtr(10),
}
result, err := storage.Query(ctx, "work discussions", filter)

// Helper functions
func stringPtr(s string) *string { return &s }
func intPtr(i int) *int { return &i }
```

### Schema Improvements

The filtering system uses a **hybrid approach** for maximum performance and backward compatibility:

**Direct Object Fields** (new, efficient):
- `source` - Document source as a direct field
- `speakerID` - Speaker/contact ID as a direct field  
- Uses exact matching with `filters.Equal` operator

**Legacy JSON Metadata** (backward compatible):
- `metadataJson` - Still maintained for complex metadata
- Automatically merges direct fields into metadata maps
- Zero breaking changes for existing code

**Migration Behavior:**
- New schemas get both direct fields and JSON metadata
- Existing schemas automatically get new fields added
- All existing code continues to work unchanged

### Performance Benefits

- **Before**: `LIKE *"source":"value"*` pattern matching on JSON strings
- **After**: Direct field queries with proper indexing
- **Result**: Faster queries and cleaner Weaviate GraphQL

**Backward Compatibility**: 100% maintained - pass `nil` filter to use original behavior.

## Advanced Tags Filtering 🏷️

The evolvingmemory package now supports **polynomial boolean logic** for tags filtering, enabling complex search patterns beyond simple AND operations.

### Simple Usage

```go
// 📝 Simple AND (backward compatible): Find documents with ALL tags
filter := &memory.Filter{
    Tags: memory.NewTagsFilterAll("work", "important"),
    Limit: intPtr(10),
}

// 🔍 Simple OR: Find documents with ANY of the tags  
filter := &memory.Filter{
    Tags: memory.NewTagsFilterAny("urgent", "deadline", "asap"),
    Limit: intPtr(20),
}
```

### Complex Boolean Expressions

```go
// 🌟 Complex query: (work AND Q1) OR (personal AND urgent)
expr := memory.NewBooleanExpressionBranch(
    memory.OR,
    memory.NewBooleanExpressionLeaf(memory.AND, "work", "Q1"),
    memory.NewBooleanExpressionLeaf(memory.AND, "personal", "urgent"),
)

filter := &memory.Filter{
    Tags: memory.NewTagsFilterExpression(expr),
    Source: stringPtr("conversations"),
    Distance: 0.8,
}

result, err := storage.Query(ctx, "project updates", filter)
```

### Advanced Nested Logic

```go
// 🔥 Super complex: ((project AND alpha) OR (project AND beta)) AND important
innerExpr := memory.NewBooleanExpressionBranch(
    memory.OR,
    memory.NewBooleanExpressionLeaf(memory.AND, "project", "alpha"),
    memory.NewBooleanExpressionLeaf(memory.AND, "project", "beta"),
)

outerExpr := memory.NewBooleanExpressionBranch(
    memory.AND,
    innerExpr,
    memory.NewBooleanExpressionLeaf(memory.OR, "important"),
)

filter := &memory.Filter{
    Tags: memory.NewTagsFilterExpression(outerExpr),
}
```

### Performance Characteristics

- **Simple ALL** (`NewTagsFilterAll`): ⚡ **Very Fast** - Single `ContainsAll` query
- **Simple ANY** (`NewTagsFilterAny`): ⚡ **Fast** - OR conditions  
- **Complex Expressions**: 🐌 **Slower but Powerful** - Nested boolean queries

### Backward Compatibility

Old code patterns are automatically migrated:

```go
// ❌ Old way (no longer works):
filter := &memory.Filter{
    Tags: []string{"work", "important"}, // Compile error!
}

// ✅ New equivalent:
filter := &memory.Filter{
    Tags: memory.NewTagsFilterAll("work", "important"),
}
```

## Integration with Other Filters

## Where to find things

```
evolvingmemory/
├── evolvingmemory.go       # StorageImpl, MemoryStorage interface, Dependencies
├── engine.go               # MemoryEngine - pure business logic
├── orchestrator.go         # MemoryOrchestrator - coordination & workers
├── pipeline.go             # Utilities, DefaultConfig, helper functions
├── pure.go                 # Pure functions (validation, object creation)
├── tools.go                # LLM tool definitions for OpenAI function calling
├── prompts.go              # All LLM prompts for fact extraction and decisions
├── *_test.go               # Comprehensive test suite
└── storage/
    └── storage.go          # Storage abstraction (WeaviateStorage implementation)
```

### Key Files Explained

**evolvingmemory.go** - Start here! Main types and public interface:
- `StorageImpl` - Public API implementation
- `MemoryStorage` - Main interface for external consumers
- `Dependencies` - Dependency injection structure
- `New()` - Constructor with full validation

**engine.go** - Pure business logic (no infrastructure concerns):
- `MemoryEngine` - Core business operations
- `ExtractFacts()` - LLM-based fact extraction
- `ProcessFact()` - Memory decision making
- `ExecuteDecision()` - Memory updates

**orchestrator.go** - Infrastructure coordination:
- `MemoryOrchestrator` - Coordinates workers and channels
- `ProcessDocuments()` - Main processing pipeline
- Worker management and progress reporting

**storage/storage.go** - Hot-swappable storage abstraction:
- `Interface` - Storage abstraction
- `WeaviateStorage` - Current implementation
- Easy to extend with new backends

## Common Tasks

### Adding a new storage backend

1. Implement `storage.Interface` in a new package:
```go
type RedisStorage struct { ... }
func (r *RedisStorage) Query(ctx context.Context, queryText string) (memory.QueryResult, error) { ... }
// ... implement all interface methods
```

2. Update your application to use the new storage:
```go
redisStorage := redis.NewStorage(redisClient, logger, embeddingsService)
storage, err := evolvingmemory.New(evolvingmemory.Dependencies{
    Storage: redisStorage,  // Just swap this!
    // ... other deps unchanged
})
```

### Adding a new document type

1. Implement the `memory.Document` interface
2. Add a case in `engine.ExtractFacts()` 
3. Create extraction logic following existing patterns

### Changing how facts are extracted

Look in `engine.go`:
- `extractFactsFromConversation()` - For chat conversations
- `extractFactsFromTextDocument()` - For plain text
- Prompts are defined in `prompts.go`

### Modifying memory decision logic

Check `engine.go`:
- `DecideAction()` - Builds prompt and calls LLM
- `buildDecisionPrompt()` - Constructs the decision prompt
- `parseToolCallResponse()` - Parses LLM's decision

### Debugging issues

1. **Fact extraction failing?** → Check logs in `orchestrator.extractFactsWorker()`
2. **Memories not updating?** → Look at `orchestrator.processFactsWorker()` logs
3. **Validation errors?** → See `ValidateMemoryOperation()` in `pure.go`
4. **Storage failing?** → Check storage implementation logs

## Configuration

```go
config := evolvingmemory.Config{
    // Parallelism
    Workers:               4,    // Number of parallel workers
    FactsPerWorker:        10,   // Facts processed per worker batch
    
    // Batching
    BatchSize:            100,   // Memories per storage batch
    FlushInterval:        5 * time.Second,
    
    // Timeouts
    FactExtractionTimeout: 30 * time.Second,
    MemoryDecisionTimeout: 30 * time.Second,
    StorageTimeout:       30 * time.Second,
    
    // Features
    EnableRichContext:      true,  // Include rich document context
    ParallelFactExtraction: true,  // Process facts in parallel
    StreamingProgress:      true,  // Stream progress updates
}
```

## Testing

The test suite is comprehensive and focuses on different aspects:
- `evolvingmemory_test.go` - Integration tests and main interface
- `engine_test.go` - Business logic tests
- `orchestrator_test.go` - Coordination logic tests
- `pipeline_test.go` - Pipeline processing tests
- `pure_test.go` - Pure function tests (fast, no external dependencies)
- `adapters_test.go` - Interface compliance and dependency validation

Run with: `go test ./pkg/agent/memory/evolvingmemory/...`

Most tests gracefully skip when AI services aren't configured, allowing for fast iteration.

## Common Patterns

### Processing flow for a conversation:
1. `ConversationDocument` arrives at `StorageImpl.StoreV2()`
2. `MemoryOrchestrator.ProcessDocuments()` coordinates the pipeline
3. Document gets prepared with metadata in `PrepareDocuments()`
4. `MemoryEngine.ExtractFacts()` extracts facts: "User likes pizza", "Alice is a developer"
5. For each fact, `MemoryEngine.ProcessFact()`:
   - Searches for similar memories via storage
   - LLM decides what to do
   - Validates the operation
   - Executes (immediate for UPDATE/DELETE, batched for ADD)
6. `MemoryOrchestrator` batches new memories and flushes to storage

### Error handling:
- Each stage returns errors through channels
- Timeouts on all external operations (LLM calls, storage operations)
- Validation prevents illegal operations (cross-speaker modifications)
- Errors are logged but processing continues for other documents

## Gotchas

1. **UPDATE/DELETE are immediate** - They happen right away, not batched
2. **Speaker validation is strict** - Can't modify other people's memories
3. **Facts can be empty** - Not all documents produce extractable facts
4. **Channels close in order** - Progress channel closes before error channel
5. **Storage abstraction** - Don't depend on Weaviate-specific features

## Architecture Benefits

### Clean Separation of Concerns
- **StorageImpl**: Simple public API, no business logic
- **MemoryOrchestrator**: Infrastructure concerns only
- **MemoryEngine**: Pure business logic, easily testable
- **storage.Interface**: Hot-swappable storage backends

### Hot-Swappable Storage
The storage abstraction allows easy switching between backends:
```go
// Current: Weaviate
weaviateStorage := storage.New(weaviateClient, logger, embeddingsService)

// Future: Redis, Postgres, etc.
redisStorage := redis.New(redisClient, logger, embeddingsService)
pgStorage := postgres.New(pgClient, logger, embeddingsService)

// Just change the dependency injection!
storage, _ := evolvingmemory.New(evolvingmemory.Dependencies{
    Storage: redisStorage,  // or weaviateStorage, or pgStorage
    // ... other dependencies unchanged
})
```

### Testability
- Pure functions in `pure.go` are fast to test
- Business logic in `MemoryEngine` can be tested with mocks
- Integration tests can use real or mock storage backends
- No global state or hard dependencies

### Backward Compatibility
All existing APIs are preserved:
- `Store()` method works unchanged
- `StoreConversations()` alias maintained
- `Query()` and `QueryWithDistance()` delegated to storage
- Zero breaking changes for existing consumers

## Recent Changes

### Advanced Filtering System

**Major improvement**: Upgraded from JSON string pattern matching to proper object fields.

#### What Changed:
- **New Filter struct** with `Source`, `ContactName`, `Distance`, and `Limit` fields
- **Hybrid schema approach**: Direct object fields + legacy JSON metadata  
- **Performance boost**: `filters.Equal` on indexed fields vs `LIKE *pattern*` on JSON strings
- **Auto-migration**: Existing schemas get new fields added automatically
- **100% backward compatible**: All existing code works unchanged

#### Benefits:
- ✅ **Faster queries** - Direct field filtering instead of JSON pattern matching
- ✅ **Cleaner API** - Structured Filter instead of complex query building  
- ✅ **Better indexing** - Weaviate can properly index direct fields
- ✅ **Future-proof** - Easy to add new filter fields
- ✅ **Zero breaking changes** - Pass `nil` filter for original behavior

#### Usage:
```go
// Old way (still works)
result, err := storage.Query(ctx, "query text", nil)

// New way (recommended)  
filter := &memory.Filter{
    Source: stringPtr("conversations"),
    Limit:  intPtr(5),
}
result, err := storage.Query(ctx, "query text", filter)
```

### Phase 3 Cleanup (January 2025)

Completed architectural cleanup for production readiness: 