# Evolving Memory Package

## What is this?

This package stores and retrieves user memories in Weaviate (a vector database). It processes documents, extracts facts using LLMs, and manages memory updates intelligently.

## Quick Start

```go
// Basic usage - store some documents
storage, _ := evolvingmemory.New(logger, weaviateClient, completionsAI, embeddingsAI)

docs := []memory.Document{
    &memory.TextDocument{...},           // Simple text
    &memory.ConversationDocument{...},   // Chat conversations
}

// Old way (still works)
err := storage.Store(ctx, docs, func(processed, total int) {
    log.Printf("Progress: %d/%d", processed, total)
})

// New way (channels)
progressCh, errorCh := storage.StoreV2(ctx, docs, config)
```

## How does it work?

### The Flow

1. **Documents come in** → Could be text documents or conversation transcripts
2. **Extract facts** → Use LLM to pull out interesting facts from the content
3. **Check existing memories** → Search Weaviate for similar facts
4. **Decide what to do** → LLM decides: ADD new memory, UPDATE existing, DELETE outdated, or NONE
5. **Execute decision** → Updates happen immediately, new memories get batched
6. **Store in Weaviate** → Batched inserts for efficiency

### Key Concepts

**Document Types:**
- `TextDocument` - Basic text with metadata (emails, notes, etc.)
- `ConversationDocument` - Structured chats with multiple speakers

**Memory Operations:**
- `ADD` - Create a new memory
- `UPDATE` - Replace an existing memory's content
- `DELETE` - Remove an outdated memory
- `NONE` - Do nothing (fact isn't worth remembering)

**Speaker Rules:**
- Document-level memories can't modify speaker-specific ones
- Speakers can only modify their own memories
- Validation happens before any UPDATE/DELETE

## Where to find things

```
evolvingmemory/
├── evolvingmemory.go    # Main types and interfaces
├── pipeline.go          # StoreV2() - the main processing pipeline
├── adapters.go          # Wrappers that connect to the old code
├── pure.go              # Business logic (no IO)
├── crud.go              # Weaviate operations
├── query.go             # Search functionality
├── factextraction.go    # LLM fact extraction (the old way)
├── prompts.go           # All the LLM prompts
└── tools.go             # OpenAI function definitions
```

### Key Files Explained

**pipeline.go** - Start here! This is where documents get processed:
- `StoreV2()` - Main entry point
- `extractFactsWorker()` - Extracts facts from documents
- `processFactsWorker()` - Decides what to do with each fact
- `aggregateResults()` - Batches new memories
- `streamingStore()` - Saves to Weaviate

**adapters.go** - Connects new architecture to old code:
- `factExtractorAdapter` - Wraps old fact extraction methods
- `memoryOperationsAdapter` - Wraps old memory decision logic

**pure.go** - Pure functions (easy to test):
- `PrepareDocuments()` - Standardizes document format
- `ValidateMemoryOperation()` - Checks if operation is allowed
- `CreateMemoryObject()` - Builds Weaviate object

## Common Tasks

### Adding a new document type

1. Implement the `Document` interface in `memory.go`
2. Add a case in `factExtractorAdapter.ExtractFacts()` 
3. Create extraction logic (like in `factextraction.go`)

### Changing how facts are extracted

Look in `factextraction.go`:
- `extractFactsFromConversation()` - For chats
- `extractFactsFromTextDocument()` - For text

The prompts are in `prompts.go`.

### Modifying memory decision logic

Check `adapters.go`:
- `DecideAction()` - Builds prompt and calls LLM
- `buildDecisionPrompt()` - Constructs the decision prompt
- `parseToolCallResponse()` - Parses LLM's decision

### Debugging issues

1. **Fact extraction failing?** → Check logs in `extractFactsWorker()`
2. **Memories not updating?** → Look at `processFactsWorker()` logs
3. **Validation errors?** → See `ValidateMemoryOperation()` in `pure.go`
4. **Storage failing?** → Check `crud.go` for Weaviate operations

## Configuration

```go
config := evolvingmemory.Config{
    Workers:               4,              // Parallel workers
    BatchSize:            100,             // Memories per batch
    FactExtractionTimeout: 30 * time.Second,
    MemoryDecisionTimeout: 30 * time.Second,
    StorageTimeout:       30 * time.Second,
}
```

## Testing

Each component has its own test file:
- `pure_test.go` - Tests pure functions
- `pipeline_test.go` - Tests the processing pipeline
- `adapters_test.go` - Tests the adapter logic

Run with: `go test ./pkg/agent/memory/evolvingmemory/...`

## Common Patterns

### Processing flow for a conversation:
1. `ConversationDocument` arrives
2. Gets prepared with metadata in `PrepareDocuments()`
3. Worker extracts facts: "User likes pizza", "Alice is a developer"
4. For each fact:
   - Search for similar memories
   - LLM decides what to do
   - Validate the operation
   - Execute (immediate for UPDATE/DELETE, batched for ADD)

### Error handling:
- Each stage returns errors through channels
- Timeouts on all external operations
- Validation prevents illegal operations
- Errors logged but processing continues

## Gotchas

1. **UPDATE/DELETE are immediate** - They happen right away, not batched
2. **Speaker validation is strict** - Can't modify other people's memories
3. **Facts can be empty** - Not all documents produce facts
4. **Channels close in order** - Important for the pipeline shutdown

## Future Improvements

- Better error aggregation (currently returns first error)
- Metrics/monitoring hooks
- Configurable retry logic
- Smarter batching strategies 

## Recent Changes

### Error Handling Improvements (2024-12-19)

Made the following improvements to error handling and initialization:

#### 1. Constructor Validation
- `NewFactExtractor()` and `NewMemoryOperations()` now return `(Interface, error)` instead of just `Interface`
- All dependency validation moved to initialization time instead of runtime
- Constructors validate that `storage`, `storage.client`, and `storage.completionsService` are not nil
- Fail-fast at application startup rather than during request processing

#### 2. Removed Runtime Nil Checks
- Removed all runtime nil checks from adapter methods (`ExtractFacts`, `SearchSimilar`, `DecideAction`, etc.)
- Removed nil checks from CRUD operations (`GetByID`, `Delete`, etc.)
- Code is now cleaner and more performant with guaranteed valid dependencies

#### 3. StoreBatch Error Handling
- Changed from error accumulation to fail-fast behavior
- Now returns immediately on first error instead of collecting all errors
- Provides faster feedback when batch operations fail

#### 4. Updated Tests
- Added comprehensive tests for constructor error cases
- All existing tests updated to handle new constructor signatures
- Added validation tests for nil storage, client, and service dependencies

#### Migration Guide

If you're using these constructors, update your code:

```go
// Old way:
factExtractor := NewFactExtractor(storage)
memoryOps := NewMemoryOperations(storage)

// New way:
factExtractor, err := NewFactExtractor(storage)
if err != nil {
    return fmt.Errorf("failed to create fact extractor: %w", err)
}

memoryOps, err := NewMemoryOperations(storage)
if err != nil {
    return fmt.Errorf("failed to create memory operations: %w", err)
}
```

These changes ensure that dependency issues are caught early during application startup, leading to more reliable runtime behavior. 