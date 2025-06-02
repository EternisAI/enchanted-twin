# Logic Design

## (1) Target Design

### Core Architecture
```
┌─────────────────────────────────────────────────────────────┐
│                     PUBLIC API                              │
├─────────────────────────────────────────────────────────────┤
│  Store(ctx, []Document) (<-chan Progress, <-chan Error)     │
└───────────────┬─────────────────────────────────────────────┘
                │
┌───────────────▼─────────────────────────────────────────────┐
│              PURE FUNCTIONAL CORE                           │
├─────────────────────────────────────────────────────────────┤
│  • PrepareDocuments([]Document) []PreparedDocument          │
│  • DistributeWork([]PreparedDocument, workers) [][]Prepared │
│  • ValidateMemoryOperation(ValidationRule) error            │
│  • CreateMemoryObject(fact, metadata) *models.Object        │
│  • BatchDocuments([]*models.Object, size) [][]*models.Object│
└─────────────────────────────────────────────────────────────┘
                │
┌───────────────▼─────────────────────────────────────────────┐
│           IO BOUNDARIES (Interfaces)                        │
├─────────────────────────────────────────────────────────────┤
│  FactExtractor:                                             │
│    • ExtractFacts(ctx, doc) ([]ExtractedFact, error)        │
│                                                             │
│  MemoryOperations:                                          │
│    • SearchSimilar(ctx, fact, speaker) ([]Memory, error)    │
│    • DecideAction(ctx, fact, similar) (Decision, error)     │
│    • UpdateMemory(ctx, id, content) error                   │
│    • DeleteMemory(ctx, id) error                            │
│                                                             │
│  StorageOperations:                                         │
│    • StoreBatch(ctx, []*models.Object) error                │
└─────────────────────────────────────────────────────────────┘
```

### Data Flow Types
```go
// Input processing
type PreparedDocument struct {
    Original    memory.Document
    Type        DocumentType
    SpeakerID   string
    Timestamp   time.Time
    DateString  string  // Pre-formatted
}

// Fact with full context
type ExtractedFact struct {
    Content     string
    SpeakerID   string
    Source      PreparedDocument
}

// Memory decision
type MemoryDecision struct {
    Action      MemoryAction
    TargetID    string  // For UPDATE/DELETE
    Reason      string
    Confidence  float64
}

// Processing result
type FactResult struct {
    Fact        ExtractedFact
    Decision    MemoryDecision
    Object      *models.Object  // Only for ADD
    Error       error
}

// Document result
type DocumentResult struct {
    DocumentID  string
    Facts       []FactResult
    Error       error
}
```

### Configuration
```go
type Config struct {
    // Parallelism
    Workers            int
    FactsPerWorker     int
    
    // Batching
    BatchSize          int
    FlushInterval      time.Duration
    
    // Timeouts
    FactExtractionTimeout   time.Duration
    MemoryDecisionTimeout   time.Duration
    StorageTimeout          time.Duration
    
    // Features
    EnableRichContext       bool
    ParallelFactExtraction  bool
    StreamingProgress       bool
}
```

### Main Flow (Parallel & Streaming)
```go
func (s *Storage) Store(ctx context.Context, documents []memory.Document) (<-chan Progress, <-chan error) {
    progressCh := make(chan Progress, 100)
    errorCh := make(chan error, 100)
    
    go func() {
        defer close(progressCh)
        defer close(errorCh)
        
        // Stage 1: Prepare (Pure)
        prepared, prepErrors := PrepareDocuments(documents, time.Now())
        
        // Stage 2: Distribute (Pure)
        workChunks := DistributeWork(prepared, s.config.Workers)
        
        // Stage 3: Parallel Processing Pipeline
        factStream := make(chan ExtractedFact, 1000)
        resultStream := make(chan FactResult, 1000)
        objectStream := make(chan []*models.Object, 100)
        
        // Workers: Document → Facts
        var extractWg sync.WaitGroup
        for _, chunk := range workChunks {
            extractWg.Add(1)
            go s.extractFactsWorker(ctx, chunk, factStream, &extractWg)
        }
        
        // Workers: Facts → Decisions → Results
        var processWg sync.WaitGroup
        for i := 0; i < s.config.Workers; i++ {
            processWg.Add(1)
            go s.processFactsWorker(ctx, factStream, resultStream, &processWg)
        }
        
        // Aggregator: Results → Batches
        go s.aggregateResults(ctx, resultStream, objectStream)
        
        // Storage: Batches → Weaviate
        go s.streamingStore(ctx, objectStream, progressCh, errorCh)
        
        // Close channels in order
        extractWg.Wait()
        close(factStream)
        processWg.Wait()
        close(resultStream)
    }()
    
    return progressCh, errorCh
}
```

## (2) Transformation Strategy

### Phase 1: Create Pure Functions & Types
```go
// New files:
// - pure.go          (all pure business logic)
// - types.go         (all type definitions)
// - validation.go    (validation rules)
// - interfaces.go    (IO boundaries)
```

### Phase 2: Extract IO Operations
```go
// Implement interfaces wrapping existing code:
type weaviateMemoryOps struct {
    storage *WeaviateStorage
}

func (w *weaviateMemoryOps) SearchSimilar(ctx context.Context, fact string, speakerID string) ([]ExistingMemory, error) {
    // Wrap existing Query method
    result, err := w.storage.Query(ctx, fact)
    // Transform to new type
}

func (w *weaviateMemoryOps) DecideAction(ctx context.Context, fact string, similar []ExistingMemory) (MemoryDecision, error) {
    // Extract LLM decision logic from updateMemories
}
```

### Phase 3: Build Worker Pipeline
```go
// Worker functions that compose IO operations with pure logic:

func (s *Storage) extractFactsWorker(ctx context.Context, docs []PreparedDocument, out chan<- ExtractedFact, wg *sync.WaitGroup) {
    defer wg.Done()
    
    for _, doc := range docs {
        // IO: Extract facts
        facts, err := s.factExtractor.ExtractFacts(ctx, doc)
        if err != nil {
            continue // or send to error channel
        }
        
        // Pure: Transform to ExtractedFact
        for _, factContent := range facts {
            fact := ExtractedFact{
                Content:   factContent,
                SpeakerID: doc.SpeakerID,
                Source:    doc,
            }
            
            select {
            case out <- fact:
            case <-ctx.Done():
                return
            }
        }
    }
}

func (s *Storage) processFactsWorker(ctx context.Context, facts <-chan ExtractedFact, out chan<- FactResult, wg *sync.WaitGroup) {
    defer wg.Done()
    
    for fact := range facts {
        // IO: Search similar
        similar, _ := s.memoryOps.SearchSimilar(ctx, fact.Content, fact.SpeakerID)
        
        // IO: Decide action
        decision, _ := s.memoryOps.DecideAction(ctx, fact.Content, similar)
        
        // Pure: Validation
        if err := ValidateMemoryOperation(ValidationRule{
            IsDocumentLevel: fact.SpeakerID == "",
            TargetSpeakerID: decision.TargetID,
            Action:         decision.Action,
        }); err != nil {
            out <- FactResult{Fact: fact, Error: err}
            continue
        }
        
        // IO: Execute immediate actions
        switch decision.Action {
        case UPDATE:
            err := s.memoryOps.UpdateMemory(ctx, decision.TargetID, fact.Content)
            out <- FactResult{Fact: fact, Decision: decision, Error: err}
        case DELETE:
            err := s.memoryOps.DeleteMemory(ctx, decision.TargetID)
            out <- FactResult{Fact: fact, Decision: decision, Error: err}
        case ADD:
            // Pure: Create object
            obj := CreateMemoryObject(fact, decision)
            out <- FactResult{Fact: fact, Decision: decision, Object: obj}
        }
    }
}
```

### Phase 4: Replace Existing Store
```go
// Complete rewrite of store.go with new architecture
// Delete old processConversationForSpeaker, processTextDocumentForSpeaker
// Replace with clean pipeline
```

## (3) Green Field Benefits

Since we're going green field:

1. **Clean API**: Return channels for streaming progress/errors
2. **No Callbacks**: Pure channel-based communication
3. **Required Config**: No defaults, explicit configuration
4. **Structured Errors**: Rich error types with context
5. **Metrics Built-in**: Processing rates, memory decisions, etc.

```go
// Example clean API usage:
config := memory.Config{
    Workers:               8,
    BatchSize:            100,
    FactExtractionTimeout: 30 * time.Second,
}

storage := memory.NewStorage(weaviateClient, llmService, config)

progressCh, errorCh := storage.Store(ctx, documents)

// Monitor in parallel
go func() {
    for progress := range progressCh {
        fmt.Printf("Processed %d/%d documents\n", progress.Processed, progress.Total)
    }
}()

// Collect errors
var errors []error
for err := range errorCh {
    errors = append(errors, err)
}
```

# Codebase Design


# Memory Storage Refactoring Report

## Executive Summary

This report outlines the module design for refactoring the `evolvingmemory` package from a monolithic, IO-heavy implementation to a clean functional architecture with clear separation of concerns, as outlined in [PLAN.md](PLAN.md).

## Module Architecture

### Core Design Principles (from PLAN.md)
1. **Pure Functional Core**: Business logic as deterministic functions
2. **IO at Boundaries**: All external operations isolated in interfaces
3. **Parallel Processing**: Concurrent document processing via workers
4. **Streaming Results**: Channels for progress and error reporting

### Module Structure

```
evolvingmemory/
├── evolvingmemory.go     # Core types, interfaces, and initialization
├── crud.go               # Database operations (IO boundary)
├── pure.go               # Pure functional transformations
├── pipeline.go           # Orchestration and workers
└── [existing files]      # Gradually refactored
```

## Module Specifications

### 1. `evolvingmemory.go` - Core Types & Contracts
**Purpose**: Define the domain model and contracts for the system

**Types** (from PLAN.md lines 30-77):
```go
// Configuration
type Config struct {
    Workers               int
    BatchSize            int
    FactExtractionTimeout time.Duration
    MemoryDecisionTimeout time.Duration
    StorageTimeout       time.Duration
}

// Document preparation
type PreparedDocument struct {
    Original    memory.Document
    Type        DocumentType
    SpeakerID   string
    Timestamp   time.Time
    DateString  string
}

// Processing pipeline types
type ExtractedFact struct {
    Content     string
    SpeakerID   string
    Source      PreparedDocument
}

type MemoryDecision struct {
    Action      MemoryAction
    TargetID    string
    Reason      string
    Confidence  float64
}

type FactResult struct {
    Fact        ExtractedFact
    Decision    MemoryDecision
    Object      *models.Object
    Error       error
}

// Progress reporting
type Progress struct {
    Processed int
    Total     int
    Stage     string
}
```

**Interfaces** (from PLAN.md lines 20-28):
```go
type FactExtractor interface {
    ExtractFacts(ctx context.Context, doc PreparedDocument) ([]string, error)
}

type MemoryOperations interface {
    SearchSimilar(ctx context.Context, fact string, speakerID string) ([]ExistingMemory, error)
    DecideAction(ctx context.Context, fact string, similar []ExistingMemory) (MemoryDecision, error)
    UpdateMemory(ctx context.Context, memoryID string, newContent string) error
    DeleteMemory(ctx context.Context, memoryID string) error
}

type StorageOperations interface {
    StoreBatch(ctx context.Context, objects []*models.Object) error
}
```

**Functions**:
- `New()` - Constructor for WeaviateStorage

### 2. `crud.go` - Database Operations
**Purpose**: Isolate all Weaviate-specific IO operations

**Functions** (moved from evolvingmemory.go):
```go
// Schema operations
func EnsureSchemaExistsInternal(client *weaviate.Client, logger *log.Logger) error

// CRUD operations
func (s *WeaviateStorage) GetByID(ctx context.Context, id string) (*memory.TextDocument, error)
func (s *WeaviateStorage) Update(ctx context.Context, id string, doc memory.TextDocument, vector []float32) error
func (s *WeaviateStorage) Delete(ctx context.Context, id string) error
func (s *WeaviateStorage) DeleteAll(ctx context.Context) error

// Implements StorageOperations interface
func (s *WeaviateStorage) StoreBatch(ctx context.Context, objects []*models.Object) error
```

### 3. `pure.go` - Pure Functional Core
**Purpose**: All deterministic, side-effect-free business logic

**Functions** (from PLAN.md architectural vision):
```go
// Document preparation
func PrepareDocuments(docs []memory.Document, currentTime time.Time) ([]PreparedDocument, []error)

// Work distribution  
func DistributeWork(docs []PreparedDocument, workers int) [][]PreparedDocument

// Validation
type ValidationRule struct {
    IsDocumentLevel bool
    TargetSpeakerID string
    Action          MemoryAction
}
func ValidateMemoryOperation(rule ValidationRule) error

// Object creation
func CreateMemoryObject(fact ExtractedFact, decision MemoryDecision) *models.Object

// Batching
func BatchObjects(objects []*models.Object, size int) [][]*models.Object

// Helpers
func firstNChars(s string, n int) string
func getCurrentDateForPrompt() string
```

### 4. `pipeline.go` - Orchestration Layer
**Purpose**: Implement the parallel processing pipeline

**Main Function** (from PLAN.md lines 97-142):
```go
func (s *WeaviateStorage) StoreV2(ctx context.Context, documents []memory.Document) (<-chan Progress, <-chan error)
```

**Worker Functions** (from PLAN.md lines 180-266):
```go
// Document → Facts extraction
func (s *WeaviateStorage) extractFactsWorker(
    ctx context.Context, 
    docs []PreparedDocument, 
    out chan<- ExtractedFact, 
    wg *sync.WaitGroup,
)

// Facts → Memory decisions → Results
func (s *WeaviateStorage) processFactsWorker(
    ctx context.Context,
    facts <-chan ExtractedFact,
    out chan<- FactResult,
    wg *sync.WaitGroup,
)

// Results → Batches aggregation
func (s *WeaviateStorage) aggregateResults(
    ctx context.Context,
    results <-chan FactResult,
    out chan<- []*models.Object,
)

// Batches → Storage
func (s *WeaviateStorage) streamingStore(
    ctx context.Context,
    batches <-chan []*models.Object,
    progress chan<- Progress,
    errors chan<- error,
)
```

**Interface Adapters**:
```go
// Wraps existing fact extraction to match FactExtractor interface
type factExtractorAdapter struct {
    storage *WeaviateStorage
}

func (f *factExtractorAdapter) ExtractFacts(ctx context.Context, doc PreparedDocument) ([]string, error) {
    // Routes to extractFactsFromConversation or extractFactsFromTextDocument
}

// Wraps existing memory operations to match MemoryOperations interface
type memoryOperationsAdapter struct {
    storage *WeaviateStorage
}

func (m *memoryOperationsAdapter) SearchSimilar(...) ([]ExistingMemory, error) {
    // Wraps Query()
}

func (m *memoryOperationsAdapter) DecideAction(...) (MemoryDecision, error) {
    // Extracts decision logic from updateMemories()
}
```

## Consistency Analysis

### 1. **Separation of Concerns** ✓
- **Pure functions** in `pure.go` have no dependencies on external systems
- **IO operations** clearly bounded in interfaces and isolated in specific modules
- **Orchestration** separated from business logic in `pipeline.go`

### 2. **Parallel Processing** ✓
The pipeline design directly implements the architecture from PLAN.md:
```
Documents → [Workers] → Facts → [Workers] → Decisions → Aggregator → Storage
```

### 3. **Streaming Architecture** ✓
- Progress reported via channels, not callbacks
- Errors streamed as they occur
- No blocking on large document sets

### 4. **Testability** ✓
- Pure functions can be tested with simple input/output assertions
- IO boundaries can be mocked via interfaces
- Workers can be tested independently

### 5. **Type Safety** ✓
- Rich types (PreparedDocument, ExtractedFact, etc.) carry context through pipeline
- No string maps or untyped data flowing between stages
- Explicit error handling at each stage

## Migration Path

1. **Phase 1**: Add new files without breaking existing code
   - Create `evolvingmemory.go` types/interfaces
   - Implement `pure.go` functions
   - Move CRUD to `crud.go`

2. **Phase 2**: Build parallel infrastructure
   - Implement `pipeline.go` with StoreV2()
   - Create interface adapters for existing code
   - Test StoreV2() alongside existing Store()

3. **Phase 3**: Switch over
   - Update callers to use StoreV2()
   - Remove old Store() implementation
   - Clean up unused code

## Conclusion

This module design achieves the vision outlined in PLAN.md by:
- Creating a **pure functional core** that's easy to reason about and test
- Establishing **clear IO boundaries** through well-defined interfaces
- Enabling **parallel processing** through a channel-based worker pipeline
- Providing **streaming feedback** for better user experience

The design maintains consistency by ensuring each module has a single, clear responsibility while working together to form a cohesive, functional architecture.


You're absolutely right! I completely missed the heart of the system - the memory update logic. Let me revise the report to properly address this critical component:

# Memory Storage Refactoring Report (REVISED)

## The Memory Update Flow - The Core Logic

### Current State (in `memoryprocessing.go`)
The monolithic `updateMemories()` function handles:
1. **Search** for similar memories (Weaviate IO)
2. **Decide** action via LLM (OpenAI IO) 
3. **Execute** UPDATE/DELETE immediately (Weaviate IO)
4. **Return** ADD objects for batching

### New Architecture for Memory Updates

From PLAN.md, the memory update logic is distributed across the pipeline:

```
For Each Fact:
├── Search Similar Memories (IO)
├── Decide Action (IO)
├── Validate Operation (Pure)
├── Execute if UPDATE/DELETE (IO)
└── Create Object if ADD (Pure)
```

## Revised Module Specifications

### 1. `evolvingmemory.go` - Additional Types for Memory Logic

**Memory-specific types**:
```go
// Memory search results
type ExistingMemory struct {
    ID        string
    Content   string
    Timestamp time.Time
    Score     float64
    Metadata  map[string]string  // Contains speakerID if present
}

// Memory actions
type MemoryAction string
const (
    ADD    MemoryAction = "ADD"
    UPDATE MemoryAction = "UPDATE" 
    DELETE MemoryAction = "DELETE"
    NONE   MemoryAction = "NONE"
)

// Validation rules
type ValidationRule struct {
    CurrentSpeakerID string      // Who's processing this fact
    IsDocumentLevel  bool        // Is current context document-level?
    TargetMemoryID   string      // For UPDATE/DELETE
    TargetSpeakerID  string      // Speaker of target memory
    Action           MemoryAction
}
```

### 2. `pure.go` - Memory Validation Logic

**Critical validation function** (extracted from current `updateMemories`):
```go
// ValidateMemoryOperation ensures speaker context rules are followed
func ValidateMemoryOperation(rule ValidationRule) error {
    switch rule.Action {
    case UPDATE, DELETE:
        // Document-level context cannot modify speaker-specific memories
        if rule.IsDocumentLevel && rule.TargetSpeakerID != "" {
            return fmt.Errorf("document-level context cannot %s speaker-specific memory", rule.Action)
        }
        
        // Speaker-specific context can only modify their own memories
        if !rule.IsDocumentLevel && rule.TargetSpeakerID != rule.CurrentSpeakerID {
            return fmt.Errorf("speaker %s cannot %s memory belonging to speaker %s", 
                rule.CurrentSpeakerID, rule.Action, rule.TargetSpeakerID)
        }
    }
    return nil
}

// CreateMemoryObject builds the Weaviate object for ADD operations
func CreateMemoryObject(fact ExtractedFact, decision MemoryDecision) *models.Object {
    metadata := make(map[string]string)
    for k, v := range fact.Source.Original.Metadata() {
        metadata[k] = v
    }
    
    if fact.SpeakerID != "" {
        metadata["speakerID"] = fact.SpeakerID
    }
    
    return &models.Object{
        Class: ClassName,
        Properties: map[string]interface{}{
            "content":      fact.Content,
            "metadataJson": marshalMetadata(metadata),
            "timestamp":    fact.Source.Timestamp.Format(time.RFC3339),
        },
    }
}
```

### 3. `pipeline.go` - Memory Update Integration

**The critical `processFactsWorker` with full memory logic**:
```go
func (s *WeaviateStorage) processFactsWorker(
    ctx context.Context,
    facts <-chan ExtractedFact,
    out chan<- FactResult,
    wg *sync.WaitGroup,
) {
    defer wg.Done()
    
    for fact := range facts {
        result := s.processSingleFact(ctx, fact)
        
        select {
        case out <- result:
        case <-ctx.Done():
            return
        }
    }
}

// processSingleFact encapsulates the memory update logic
func (s *WeaviateStorage) processSingleFact(ctx context.Context, fact ExtractedFact) FactResult {
    // Step 1: Search for similar memories (IO)
    similar, err := s.memoryOps.SearchSimilar(ctx, fact.Content, fact.SpeakerID)
    if err != nil {
        return FactResult{Fact: fact, Error: fmt.Errorf("search failed: %w", err)}
    }
    
    // Step 2: LLM decides action (IO)
    decision, err := s.memoryOps.DecideAction(ctx, fact.Content, similar)
    if err != nil {
        return FactResult{Fact: fact, Error: fmt.Errorf("decision failed: %w", err)}
    }
    
    // Step 3: Validate the operation (Pure)
    if decision.Action == UPDATE || decision.Action == DELETE {
        targetMemory := findMemoryByID(similar, decision.TargetID)
        if targetMemory == nil {
            return FactResult{Fact: fact, Error: fmt.Errorf("target memory %s not found", decision.TargetID)}
        }
        
        rule := ValidationRule{
            CurrentSpeakerID: fact.SpeakerID,
            IsDocumentLevel:  fact.SpeakerID == "",
            TargetMemoryID:   decision.TargetID,
            TargetSpeakerID:  targetMemory.Metadata["speakerID"],
            Action:          decision.Action,
        }
        
        if err := ValidateMemoryOperation(rule); err != nil {
            s.logger.Warnf("Validation failed: %v", err)
            return FactResult{Fact: fact, Decision: decision, Error: err}
        }
    }
    
    // Step 4: Execute the action
    switch decision.Action {
    case UPDATE:
        // Generate new embedding (IO)
        embedding, err := s.embeddingsService.Embedding(ctx, fact.Content, openAIEmbedModel)
        if err != nil {
            return FactResult{Fact: fact, Decision: decision, Error: err}
        }
        
        // Execute update (IO)
        err = s.memoryOps.UpdateMemory(ctx, decision.TargetID, fact.Content, embedding)
        return FactResult{Fact: fact, Decision: decision, Error: err}
        
    case DELETE:
        // Execute delete (IO)
        err := s.memoryOps.DeleteMemory(ctx, decision.TargetID)
        return FactResult{Fact: fact, Decision: decision, Error: err}
        
    case ADD:
        // Create object for batch insert (Pure)
        obj := CreateMemoryObject(fact, decision)
        
        // Generate embedding (IO) 
        embedding, err := s.embeddingsService.Embedding(ctx, fact.Content, openAIEmbedModel)
        if err != nil {
            return FactResult{Fact: fact, Decision: decision, Error: err}
        }
        obj.Vector = toFloat32(embedding)
        
        return FactResult{Fact: fact, Decision: decision, Object: obj}
        
    case NONE:
        s.logger.Debugf("No action taken for fact: %s", fact.Content)
        return FactResult{Fact: fact, Decision: decision}
        
    default:
        return FactResult{Fact: fact, Error: fmt.Errorf("unknown action: %s", decision.Action)}
    }
}
```

### 4. Memory Operations Interface Implementation

**Extracting from current `updateMemories` into clean interfaces**:
```go
type memoryOperationsAdapter struct {
    storage *WeaviateStorage
}

// SearchSimilar wraps the existing Query method
func (m *memoryOperationsAdapter) SearchSimilar(ctx context.Context, fact string, speakerID string) ([]ExistingMemory, error) {
    result, err := m.storage.Query(ctx, fact)
    if err != nil {
        return nil, err
    }
    
    // Transform QueryResult to []ExistingMemory
    memories := make([]ExistingMemory, len(result.Facts))
    for i, f := range result.Facts {
        memories[i] = ExistingMemory{
            ID:        f.ID,
            Content:   f.Content,
            Timestamp: f.Timestamp,
            Metadata:  f.Metadata,
            // Score would come from enhanced Query method
        }
    }
    return memories, nil
}

// DecideAction extracts the LLM decision logic
func (m *memoryOperationsAdapter) DecideAction(ctx context.Context, fact string, similar []ExistingMemory) (MemoryDecision, error) {
    // Build prompt with existing memories
    prompt := buildMemoryUpdatePrompt(fact, similar)
    
    // Call LLM with memory decision tools
    response, err := m.storage.completionsService.Completions(
        ctx,
        []openai.ChatCompletionMessageParamUnion{
            openai.SystemMessage(prompt),
        },
        memoryDecisionToolsList,
        openAIChatModel,
    )
    
    if err != nil {
        return MemoryDecision{}, err
    }
    
    // Parse tool call response
    return parseMemoryDecision(response)
}
```

## The Complete Memory Update Flow

```
┌────────────────────┐
│   ExtractedFact    │
└─────────┬──────────┘
          │
    ┌─────▼─────┐
    │  Search   │ ← Query Weaviate for similar memories
    │ Similar   │
    └─────┬─────┘
          │
    ┌─────▼─────┐
    │  Decide   │ ← LLM determines ADD/UPDATE/DELETE/NONE
    │  Action   │
    └─────┬─────┘
          │
    ┌─────▼─────┐
    │ Validate  │ ← Pure function checks speaker permissions
    │Operation  │
    └─────┬─────┘
          │
    ┌─────▼──────────────┐
    │ Execute Action     │
    ├────────────────────┤
    │ UPDATE → Immediate │ ← Update in Weaviate now
    │ DELETE → Immediate │ ← Delete from Weaviate now
    │ ADD → Create Obj   │ ← Return object for batching
    │ NONE → Skip        │
    └────────────────────┘
```

## Why This Design Works

1. **Preserves Business Logic**: The complex speaker validation rules are extracted into pure functions
2. **Immediate vs Batched**: UPDATE/DELETE execute immediately (as required), while ADDs are batched for efficiency
3. **Clear IO Boundaries**: Each IO operation is explicit and mockable
4. **Parallel Safe**: Each fact is processed independently with no shared state
5. **Maintainable**: The memory update logic is no longer buried in a 200+ line function

The memory update logic is THE CORE of this system - it's where all the intelligence lives. This refactoring makes it testable, understandable, and maintainable.
