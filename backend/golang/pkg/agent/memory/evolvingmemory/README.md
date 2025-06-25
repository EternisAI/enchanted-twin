# Evolving Memory Package

## What is this?

This package stores and retrieves user memories using hot-swappable storage backends (currently Weaviate). It processes documents, extracts memory facts using LLMs with **structured extraction**, and stores them directly without complex decision-making processes.

## Architecture Overview

The package follows clean architecture principles with a **simplified 2-layer approach** that eliminates computational explosion:

```
┌─────────────────┐    ┌──────────────────────┐
│   StorageImpl   │───▶│ MemoryOrchestrator   │
│ (Public API)    │    │   (Coordination)     │
└─────────────────┘    └──────────────────────┘
         │                       │
         │                       ▼
         │             ┌──────────────────────┐    ┌──────────────────┐
         │             │   Workers &          │───▶│ storage.Interface│ 
         │             │   Direct Storage     │    └──────────────────┘
         │             └──────────────────────┘             │
         │                                                  ▼
         ▼ (Clean Public Interface)                ┌──────────────────┐
   ┌─────────────────┐                             │ WeaviateStorage  │
   │  MemoryStorage  │                             │ RedisStorage     │
   │   (Interface)   │                             │ PostgresStorage  │
   └─────────────────┘                             └──────────────────┘
```

### Layer Responsibilities

1. **StorageImpl** - Thin public API maintaining interface compatibility
2. **MemoryOrchestrator** - Coordinates document processing, fact extraction, and direct storage
3. **storage.Interface** - Hot-swappable storage abstraction

### Simplified Flow

The new architecture eliminates the computational explosion that occurred during complex decision-making:

**Before**: Document → Facts → **FOR EACH FACT**: Search Similar → LLM Decision → Execute → Store  
**After**: Document → Facts → **Direct Batch Storage**

This reduces the operations from **300+ LLM calls** per document to **1 LLM call per document chunk**.

## Memory Fact Structure 🧠

The evolving memory system uses **advanced structured extraction** to create rich, categorized memories with privacy controls and importance scoring.

### MemoryFact Structure

Each memory fact contains:

```go
type MemoryFact struct {
    // Identity and display
    ID        string    `json:"id"`
    Content   string    `json:"content"` // Searchable content
    Timestamp time.Time `json:"timestamp"`

    // Structured fields
    Category        string  `json:"category"`         // Semantic category
    Subject         string  `json:"subject"`          // Who/what the fact is about
    Attribute       string  `json:"attribute"`        // Property being described
    Value           string  `json:"value"`            // The actual fact content
    TemporalContext *string `json:"temporal_context"` // When this happened (optional)
    Sensitivity     string  `json:"sensitivity"`      // Privacy level: high/medium/low
    Importance      int     `json:"importance"`       // Priority score: 1-3

    // Source tracking
    Source             string   `json:"source"`              // Source document type
    DocumentReferences []string `json:"document_references"` // Source document IDs
    Tags               []string `json:"tags"`                // Categorization tags

    // Legacy support
    Metadata map[string]string `json:"metadata,omitempty"` // Additional metadata
}
```

### Semantic Categories

Facts are automatically classified into **10 semantic categories**:

| Category | Description | Example |
|----------|-------------|---------|
| `profile_stable` | Core personal attributes | "User: software engineer with 10 years experience" |
| `preference` | Likes, dislikes, choices | "User: prefers dark roast coffee over light roast" |
| `goal_plan` | Objectives and plans | "User: planning to learn Rust programming language" |
| `routine` | Regular activities | "User: exercises every morning at 6 AM" |
| `skill` | Abilities and competencies | "User: proficient in Python and machine learning" |
| `relationship` | Connections with others | "User: close friend of Alice, works with Bob" |
| `health` | Medical and wellness info | "User: has seasonal allergies in spring" |
| `context_env` | Environmental/situational | "User: works from home office with standing desk" |
| `affective_marker` | Emotional responses | "User: feels stressed about upcoming presentation" |
| `event` | Specific occurrences | "User: attended React conference in San Francisco" |

### Privacy & Importance Controls

**Sensitivity Levels** (automatic GDPR compliance):
- **`high`** - Personal health, financial, sensitive topics
- **`medium`** - Work details, relationships, specific preferences  
- **`low`** - General interests, public information

**Importance Scoring** (memory prioritization):
- **`3`** - Life-changing events, major decisions, core identity
- **`2`** - Significant preferences, important relationships
- **`1`** - Minor details, temporary states

### Quality-First Extraction

The system prioritizes **quality over quantity** with:

- **Confidence threshold**: Only facts with 7+ confidence (1-10 scale)
- **Life milestone focus**: Major events, job changes, health developments
- **Filtering out**: Routine activities, temporary moods, one-off experiences
- **Rich context**: 8-30 word descriptive phrases with temporal information

### Extraction Process

```go
// Example extraction from conversation
Input: "I've been using high temperature settings with GPT-4 for creative writing"

Extracted MemoryFact:
{
    Category: "preference",
    Subject: "user", 
    Attribute: "llm_settings",
    Value: "prefers high temperature settings with GPT-4 for creative writing tasks",
    TemporalContext: "2025-01-15",
    Sensitivity: "low",
    Importance: 2,
    Content: "user - prefers high temperature settings with GPT-4 for creative writing tasks",
    Source: "conversation",
    // ... other fields auto-populated
}
```

### Storage Architecture

Memory facts are stored with all structured fields directly accessible:

**Database Storage** (direct fields for precise filtering):
```go
// Direct storage in Weaviate with structured fields
{
    "content": fact.GenerateContent(), // Auto-generated searchable string
    "factCategory":        fact.Category,
    "factSubject":         fact.Subject,
    "factAttribute":       fact.Attribute, 
    "factValue":           fact.Value,
    "factSensitivity":     fact.Sensitivity,
    "factImportance":      fact.Importance,
    "factTemporalContext": fact.TemporalContext,
    "documentReferences":  fact.DocumentReferences,
    "tags":                fact.Tags,
    "source":              fact.Source,
    "timestamp":           fact.Timestamp,
    // Legacy metadata stored as JSON
}
```

**Unified Type Benefits**:
- **No conversions**: MemoryFact used throughout the pipeline
- **Type safety**: All fields properly typed, no string metadata parsing
- **Direct queries**: Filter by any structured field without JSON extraction
- **Performance**: Indexed fields for fast filtering

### Query Capabilities 🚀

With structured facts, you can now filter by **any combination** of fact properties:

```go
// ✅ By category - all implemented categories
filter := &memory.Filter{
    FactCategory: helpers.Ptr("preference"),
}
result, err := storage.Query(ctx, "user preferences", filter)

// ✅ By importance level - precise control
filter := &memory.Filter{
    FactImportance: helpers.Ptr(3), // importance = 3
}
result, err := storage.Query(ctx, "life-changing events", filter)

// ✅ By sensitivity (GDPR-ready privacy controls)
filter := &memory.Filter{
    FactSensitivity: helpers.Ptr("high"), // sensitivity = "high"
}
result, err := storage.Query(ctx, "sensitive information", filter)

// ✅ By subject - facts about specific people/entities
filter := &memory.Filter{
    FactSubject: helpers.Ptr("alice"),
}
result, err := storage.Query(ctx, "facts about alice", filter)

// ✅ By attribute - specific properties
filter := &memory.Filter{
    FactAttribute: helpers.Ptr("health_metric"),
}
result, err := storage.Query(ctx, "health measurements", filter)

// ✅ By temporal context - time-based filtering
filter := &memory.Filter{
    FactTemporalContext: helpers.Ptr("2025-01"),
}
result, err := storage.Query(ctx, "january events", filter)

// ✅ By fact value - partial text matching
filter := &memory.Filter{
    FactValue: helpers.Ptr("coffee"),
}
result, err := storage.Query(ctx, "coffee-related preferences", filter)

// ✅ Combined semantic + structured search - the power combo!
filter := &memory.Filter{
    Source:            helpers.Ptr("conversations"),
    FactCategory:      helpers.Ptr("preference"),
    FactImportanceMin: helpers.Ptr(2),
    Distance:          0.8,
    Limit:             helpers.Ptr(10),
}
result, err := storage.Query(ctx, "important preferences from conversations", filter)
```

**Performance Benefits:**
- **Direct field indexing** - No more JSON pattern matching
- **Semantic + structured hybrid** - Best of both worlds
- **Privacy-first querying** - Built-in GDPR compliance
- **Importance-based prioritization** - Focus on what matters

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

// Store documents with progress callback
err := storage.Store(ctx, docs, func(processed, total int) {
    log.Printf("Progress: %d/%d", processed, total)
})
if err != nil {
    log.Fatal("Failed to store documents:", err)
}

// 🚀 NEW: Intelligent 3-stage query with consolidation-first approach
intelligentResult, err := storage.IntelligentQuery(ctx, "category theory", &memory.Filter{
    Distance: 0.7,
})
if err != nil {
    log.Fatal("Intelligent query failed:", err)
}

// Results organized by intelligence: insights → evidence → context
fmt.Printf("Insights: %d, Evidence: %d, Context: %d\n", 
    len(intelligentResult.ConsolidatedInsights),
    len(intelligentResult.CitedEvidence), 
    len(intelligentResult.AdditionalContext))

// Legacy semantic query (still available)
filter := &memory.Filter{
    Source:      helpers.Ptr("conversations"),
    Subject:     helpers.Ptr("alice"),
    Distance:    0.7,  // Max semantic distance
    Limit:       helpers.Ptr(10),
}
result, err := storage.Query(ctx, "work discussions", filter)
```

## Document Storage & References

### Overview

The evolving memory system includes a sophisticated document storage architecture that provides:

- **Multiple document references per memory** - Each memory fact can reference multiple source documents
- **Separate document table** - Documents stored in dedicated `SourceDocument` table with deduplication
- **Content deduplication** - Documents with identical content stored only once using SHA256 hashing
- **Intelligent chunking for large content** - Documents and conversations exceeding 20,000 characters are intelligently split into smaller chunks that preserve structure and meaning, with special handling for oversized individual messages
- **Backward compatibility** - Existing memories continue working seamlessly

### Storage Architecture

#### Memory Table (`TextDocument`)
- Stores memory facts and metadata with **structured fact fields**
- Contains `documentReferences` property with array of document IDs
- Maintains backward compatibility with old `sourceDocumentId` metadata
- **New**: Direct fields for category, subject, attribute, value, sensitivity, importance

#### Document Table (`SourceDocument`) 
- Stores unique documents with SHA256-based deduplication
- Properties:
  - `content`: Full document content
  - `contentHash`: SHA256 hash for deduplication
  - `documentType`: Type of document (text, conversation, etc.)
  - `originalId`: Original ID from source system
  - `metadata`: JSON-encoded metadata
  - `createdAt`: Storage timestamp

### Document Storage Flow

```go
// When creating a memory, documents are stored separately first
func (e *MemoryEngine) CreateMemoryObject(ctx context.Context, fact MemoryFact, source memory.Document, decision MemoryDecision) (*models.Object, error) {
    // 1. Store document separately with automatic deduplication
    documentID, err := e.storage.StoreDocument(
        ctx,
        source.Content(),    // Full document content
        docType,             // Document type (text, conversation, etc.)
        source.ID(),         // Original source ID
        source.Metadata(),   // Source metadata
    )
    
    // 2. Create memory object with document reference + structured fact fields
    properties := map[string]interface{}{
        "content":            fact.GenerateContent(), // Searchable content string
        "documentReferences": []string{documentID},
        // Structured fact fields for precise filtering
        "factCategory":       fact.Category,
        "factSubject":        fact.Subject,
        "factAttribute":      fact.Attribute,
        "factValue":          fact.Value,
        "factSensitivity":    fact.Sensitivity,
        "factImportance":     fact.Importance,
    }
    
    // 3. Generate embedding and return
    // Memory object contains structured data + references, not full document
}
```

### Storage Optimization

**Before (with document duplication):**
```
Document: 5KB → 10 facts extracted → 50KB total storage (10x multiplication)
Each fact stored the full document content inline
```

**After (with deduplication + structured facts):**
```
Document: 5KB → 10 facts extracted → ~7KB total storage (1.4x multiplication)  
Document stored once, facts reference by ID with rich structured metadata
```

**Real-world impact:**
- Typical user: ~35MB → ~5MB (85% storage reduction)
- Large datasets: Even greater savings due to content deduplication
- Faster queries due to smaller memory objects + structured field indexing
- Precise filtering by category, importance, sensitivity

### API Usage

#### Retrieving Document References

```go
// Get all document references (multiple references support)
docRefs, err := storage.GetDocumentReferences(ctx, memoryID)
if err != nil {
    log.Printf("Error: %v", err)
}
for _, ref := range docRefs {
    fmt.Printf("Document %s: %s\n", ref.ID, ref.Content[:100])
}
```

#### Document Reference Structure

```go
type DocumentReference struct {
    ID      string `json:"id"`      // Original document ID from source
    Content string `json:"content"` // Full document content  
    Type    string `json:"type"`    // Document type (conversation, text, etc.)
}
```

### Deduplication Logic

The system automatically deduplicates documents based on content:

```go
// Content hashing for deduplication
contentHash := sha256.New()
contentHash.Write([]byte(content))
contentHashStr := hex.EncodeToString(contentHash.Sum(nil))

// Check if document with this content hash already exists
existingDoc, err := storage.findDocumentByContentHash(ctx, contentHashStr)
if err == nil && existingDoc != nil {
    // Document already exists, return existing ID
    return existingDoc.ID, nil
}

// Otherwise store new document
```

### Backward Compatibility

The system maintains full backward compatibility:

#### Old Format Support
```go
// Old format in metadata
{
    "sourceDocumentId": "old-doc-123",
    "sourceDocumentType": "conversation"
}

// Automatically converted when retrieved
func GetDocumentReferences(ctx context.Context, memoryID string) ([]*DocumentReference, error) {
    // Try new format first
    if documentIDs := getNewFormatReferences(memory); len(documentIDs) > 0 {
        return getDocumentsFromTable(documentIDs)
    }
    
    // Fallback to old format
    if oldDocID := getOldFormatReference(memory); oldDocID != "" {
        return []*DocumentReference{{
            ID:      oldDocID,
            Content: "", // Empty in old format  
            Type:    memory.FieldMetadata["sourceDocumentType"],
        }}, nil
    }
}
```

#### Memory Object Structure

**New Format (supports multiple document references + structured facts):**
```go
{
    "content": "User: prefers high temperature settings with GPT-4 for creative writing (preference) (2025-01-15)",
    "documentReferences": ["doc-id-1", "doc-id-2", "doc-id-3"],
    "factCategory": "preference",
    "factSubject": "user", 
    "factAttribute": "llm_settings",
    "factValue": "prefers high temperature settings with GPT-4 for creative writing tasks",
    "factSensitivity": "low",
    "factImportance": 2,
    "factTemporalContext": "2025-01-15",
    "metadataJson": "{\"speakerID\":\"user\"}",
    "timestamp": "2025-01-01T12:00:00Z"
}
```

**Old Format (still supported):**
```go
{
    "content": "User loves pizza", 
    "metadataJson": "{\"sourceDocumentId\":\"old-doc-123\",\"sourceDocumentType\":\"conversation\"}",
    "timestamp": "2025-01-01T12:00:00Z"
}
```

## Intelligent Query System 🧠

The memory system now features a **revolutionary 3-stage intelligent query system** that provides insights with evidence, not just search results.

### 🎯 IntelligentQuery() - Consolidation-First Approach

```go
// Execute intelligent 3-stage query
result, err := storage.IntelligentQuery(ctx, "category theory", &memory.Filter{
    Distance: 0.7,
    Source:   helpers.Ptr("conversations"),
})

// Structured results with full audit trail
type IntelligentQueryResult struct {
    Query               string              `json:"query"`
    ConsolidatedInsights []memory.MemoryFact `json:"consolidated_insights"`  // Stage 1
    CitedEvidence       []memory.MemoryFact `json:"cited_evidence"`         // Stage 2  
    AdditionalContext   []memory.MemoryFact `json:"additional_context"`     // Stage 3
    Metadata            QueryMetadata       `json:"metadata"`
}
```

### 🔄 3-Stage Execution Flow

**Stage 1: Find Consolidated Insights**
```go
// Vector search on consolidated facts only
consolidatedResults := storage.Query(ctx, query, &memory.Filter{
    Tags: &memory.TagsFilter{Any: []string{"consolidated"}},
})
```

**Stage 2: Retrieve Cited Evidence**
```go
// Extract source fact IDs from consolidation metadata
for key, value := range fact.Metadata {
    if strings.HasPrefix(key, "source_fact_") {
        citedFactIDs = append(citedFactIDs, value)
    }
}
// Batch retrieve actual source facts that led to insights
citedFacts := storage.GetFactsByIDs(ctx, citedFactIDs)
```

**Stage 3: Additional Context with Deduplication**
```go
// Query raw facts, remove duplicates from previous stages
rawResults := storage.Query(ctx, query, rawFilter)
additionalContext := removeDuplicates(rawResults, existingIDs)
```

### 📊 Example Results

Query: `"category theory"` → 117 total results organized intelligently:

```
🧠 Intelligent Query Results for: "category theory"
📊 Total: 117 | 🔗 Insights: 12 | 🔗 Evidence: 37 | 📄 Context: 68

🔗 Top Consolidated Insights:
  1. primaryUser - actively participates in academic communities focused on category theory
  2. primaryUser - exhibits sustained interest in philosophy of science and category theory
  3. primaryUser - regularly teaches advanced mathematical concepts like category theory

📋 Supporting Evidence:
  1. David Spivak - discussed potential involvement in category theory platform project
  2. Dmitry Vagner - receives academic advice regarding research focus and teaching

📄 Additional Context:
  1. sub@cs.cmu.edu - discussed renaming 'applied category theory' to 'categorical design'
  2. Dmitry Vagner - discussed analogies between internal and enriched categories
```

### 🎯 Key Benefits

- **Insights First**: High-level synthesized knowledge takes priority
- **Evidence Trail**: Supporting facts show how insights were derived  
- **Smart Deduplication**: No fact appears in multiple sections
- **Performance**: Pure vector search, no LLM calls during querying
- **Audit Trail**: Full traceability from insights back to source conversations

### 🔄 Usage Patterns

```go
// Quick intelligent query
result, _ := storage.IntelligentQuery(ctx, "machine learning", nil)

// With custom filtering  
result, _ := storage.IntelligentQuery(ctx, "career decisions", &memory.Filter{
    Source:   helpers.Ptr("conversations"),
    Distance: 0.8,
})

// Access structured results
insights := result.ConsolidatedInsights      // High-level knowledge
evidence := result.CitedEvidence            // Supporting facts
context := result.AdditionalContext         // Related information
metadata := result.Metadata                 // Query execution details
```

## Advanced Filtering

The memory system supports powerful filtering capabilities for precise memory retrieval with **hybrid schema approach** for optimal performance:

### Filter Structure 🔍

The `memory.Filter` struct provides flexible query options:

```go
type Filter struct {
    // Basic filters
    Source      *string     // Filter by document source
    Subject     *string     // Filter by fact subject (maps to factSubject field)
    Tags        *TagsFilter // Complex tag filtering with boolean logic
    Distance    float32     // Maximum semantic distance (0 = disabled)
    Limit       *int        // Maximum number of results
    
    // Structured fact filters (ONLY indexed fields for performance)
    FactCategory        *string   // Filter by fact category
    FactAttribute       *string   // Filter by fact attribute
    FactImportance      *int      // Filter by exact importance score (1, 2, 3)
    
    // Range filters
    FactImportanceMin   *int      // Minimum importance (inclusive)
    FactImportanceMax   *int      // Maximum importance (inclusive)
    
    // Timestamp filters
    TimestampAfter      *time.Time // Facts created after this time
    TimestampBefore     *time.Time // Facts created before this time
    
    // Document reference filters
    DocumentReferences  []string   // Filter by source document IDs
}
```

**Note**: Fields like `factValue`, `factTemporalContext`, and `factSensitivity` are stored in the database for context and display but are NOT filterable. This is a performance optimization - these fields contain rich descriptive text that would be expensive to index.

### Schema Approach

The filtering system uses **direct object fields** for performance while maintaining backward compatibility:

**Direct Object Fields** (fast, indexed):
- `source` - Document source as a direct field
- `speakerID` - Speaker/contact ID as a direct field  
- `factCategory`, `factSubject`, etc. - Structured fact fields
- Uses exact matching with `filters.Equal` operator

**Legacy JSON Metadata** (backward compatible):
- `metadataJson` - Still maintained for complex metadata
- Automatically merges direct fields into metadata maps
- Zero breaking changes for existing code

**Migration Behavior:**
- New schemas get both direct fields and JSON metadata
- Existing schemas automatically get new fields added
- All existing code continues to work unchanged

### Usage Examples

#### Basic Filtering (Backward Compatible)
```go
// Helper functions for pointer creation
func helpers.Ptr(s string) *string { return &s }
func helpers.Ptr(i int) *int { return &i }

// Filter by source
filter := &memory.Filter{
    Source: helpers.Ptr("email"),
}
result, err := storage.Query(ctx, "work meetings", filter)

// Filter by subject
filter := &memory.Filter{
    Subject: helpers.Ptr("alice"),
}
result, err := storage.Query(ctx, "alice's preferences", filter)

// Limit results with semantic distance
filter := &memory.Filter{
    Distance: 0.7,
    Limit:    helpers.Ptr(5),
}
result, err := storage.Query(ctx, "recent activities", filter)
```

#### Structured Fact Filtering 🔥

```go
// Filter by fact category - get all preferences
filter := &memory.Filter{
    FactCategory: helpers.Ptr("preference"),
}
result, err := storage.Query(ctx, "user preferences", filter)

// Filter by importance - only critical facts (importance = 3)
filter := &memory.Filter{
    FactImportance: helpers.Ptr(3),
}
result, err := storage.Query(ctx, "life events", filter)

// High importance facts only (importance >= 2)
filter := &memory.Filter{
    FactImportanceMin: helpers.Ptr(2),
}
result, err := storage.Query(ctx, "significant memories", filter)

// Filter by subject - facts about Alice
filter := &memory.Filter{
    Subject: helpers.Ptr("alice"),
}
result, err := storage.Query(ctx, "alice information", filter)

// Filter by attribute - all health metrics
filter := &memory.Filter{
    FactAttribute: helpers.Ptr("health_metric"),
}
result, err := storage.Query(ctx, "health data", filter)

// Filter by timestamp range - facts from last 30 days
now := time.Now()
thirtyDaysAgo := now.AddDate(0, 0, -30)
filter := &memory.Filter{
    TimestampAfter: &thirtyDaysAgo,
}
result, err := storage.Query(ctx, "recent activities", filter)

// Filter by document references - facts from specific documents
filter := &memory.Filter{
    DocumentReferences: []string{"doc-id-1", "doc-id-2"},
}
result, err := storage.Query(ctx, "facts from specific documents", filter)
```

#### Advanced Combined Filtering

```go
// Preferences from conversations with high importance
filter := &memory.Filter{
    FactCategory:   helpers.Ptr("preference"),
    FactImportance: helpers.Ptr(3),
    Source:         helpers.Ptr("conversations"),
}
result, err := storage.Query(ctx, "important preferences", filter)

// Recent health facts (last 7 days) with medium-high importance
now := time.Now()
sevenDaysAgo := now.AddDate(0, 0, -7)
filter := &memory.Filter{
    FactCategory:      helpers.Ptr("health"),
    FactImportanceMin: helpers.Ptr(2),
    TimestampAfter:    &sevenDaysAgo,
}
result, err := storage.Query(ctx, "recent health updates", filter)

// Facts about specific person from multiple documents
filter := &memory.Filter{
    Subject:            helpers.Ptr("bob"),
    DocumentReferences: []string{"conv-123", "conv-456"},
    FactCategory:       helpers.Ptr("preference"),
}
result, err := storage.Query(ctx, "bob's preferences from conversations", filter)
```

#### Importance-Based Querying

```go
// Life-changing events only (importance = 3)
filter := &memory.Filter{
    FactImportance: helpers.Ptr(3),
}
result, err := storage.Query(ctx, "major life events", filter)

// Meaningful information affecting decisions (importance >= 2)
filter := &memory.Filter{
    FactImportanceMin: helpers.Ptr(2),
}
result, err := storage.Query(ctx, "decision-relevant facts", filter)

// Importance range: meaningful to critical (2-3)
filter := &memory.Filter{
    FactImportanceMin: helpers.Ptr(2),
    FactImportanceMax: helpers.Ptr(3),
}
result, err := storage.Query(ctx, "significant facts", filter)
```

#### Category-Specific Queries

```go
// All user preferences
filter := &memory.Filter{
    FactCategory: helpers.Ptr("preference"),
    Subject:      helpers.Ptr("user"),
}
result, err := storage.Query(ctx, "user preferences", filter)

// Goals and plans
filter := &memory.Filter{
    FactCategory: helpers.Ptr("goal_plan"),
    Distance:     0.9,
}
result, err := storage.Query(ctx, "future plans", filter)

// Relationship information
filter := &memory.Filter{
    FactCategory:   helpers.Ptr("relationship"),
    FactImportance: helpers.Ptr(2),
}
result, err := storage.Query(ctx, "important relationships", filter)

// Health and wellness data
filter := &memory.Filter{
    FactCategory:    helpers.Ptr("health"),
    FactImportance: helpers.Ptr(3),  // Critical health facts
}
result, err := storage.Query(ctx, "health information", filter)
```

### Performance Benefits

- **Before**: `LIKE *"source":"value"*` pattern matching on JSON strings
- **After**: Direct field queries with proper indexing + structured fact fields
- **Result**: Faster queries, cleaner Weaviate GraphQL, precise fact filtering

**Backward Compatibility**: 100% maintained - pass `nil` filter to use original behavior.

## Advanced Tags Filtering 🏷️

The evolvingmemory package supports **polynomial boolean logic** for tags filtering, enabling complex search patterns beyond simple AND operations.

### Simple Usage

```go
// Simple AND (backward compatible): Find documents with ALL tags
filter := &memory.Filter{
    Tags: memory.NewTagsFilterAll("work", "important"),
    Limit: helpers.Ptr(10),
}

// Simple OR: Find documents with ANY of the tags  
filter := &memory.Filter{
    Tags: memory.NewTagsFilterAny("urgent", "deadline", "asap"),
    Limit: helpers.Ptr(20),
}
```

### Complex Boolean Expressions

```go
// Complex query: (work AND Q1) OR (personal AND urgent)
expr := memory.NewBooleanExpressionBranch(
    memory.OR,
    memory.NewBooleanExpressionLeaf(memory.AND, "work", "Q1"),
    memory.NewBooleanExpressionLeaf(memory.AND, "personal", "urgent"),
)

filter := &memory.Filter{
    Tags: memory.NewTagsFilterExpression(expr),
    Source: helpers.Ptr("conversations"),
    Distance: 0.8,
}

result, err := storage.Query(ctx, "project updates", filter)
```

### Advanced Nested Logic

```go
// Super complex: ((project AND alpha) OR (project AND beta)) AND important
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

## How does it work?

### The Simplified Flow

1. **Documents come in** → Text documents or conversation transcripts
2. **Store documents separately** → Documents stored in `SourceDocument` table with deduplication
3. **Extract memory facts** → MemoryOrchestrator uses LLM with advanced prompts to extract `MemoryFact` objects
4. **Store directly** → All extracted facts stored immediately with structured fields directly accessible

### Key Concepts

**Document Types:**
- `TextDocument` - Basic text with metadata (emails, notes, etc.)
- `ConversationDocument` - Structured chats with multiple speakers

**Simplified Storage:**
- All memory facts are stored directly without decision-making
- No complex UPDATE/DELETE operations during ingestion
- Focus on fast, reliable storage with structured data

**Hot-Swappable Storage:**
- Depends on `storage.Interface`, not specific implementations
- Currently supports WeaviateStorage
- Easy to add RedisStorage, PostgresStorage, etc.

### Performance Benefits

The simplified architecture provides:
- **85-95% reduction in LLM calls** - From 300+ calls to 1 call per document chunk
- **Faster ingestion** - No complex decision-making during storage
- **Predictable performance** - Linear scaling with document size
- **Reduced complexity** - Fewer failure points and edge cases

## Where to find things

```
evolvingmemory/
├── evolvingmemory.go       # StorageImpl, MemoryStorage interface, Dependencies
├── orchestrator.go         # MemoryOrchestrator - coordination & direct storage
├── pipeline.go             # Utilities, DefaultConfig, helper functions
├── pure.go                 # Pure functions (validation, object creation)
├── tools.go                # LLM tool definitions for structured fact extraction
├── prompts.go              # All LLM prompts for fact extraction
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

**orchestrator.go** - Simplified coordination:
- `MemoryOrchestrator` - Coordinates workers and direct storage
- `ProcessDocuments()` - Main processing pipeline (simplified)
- `storeFactsDirectly()` - Direct storage without decision-making
- Worker management and progress reporting

**tools.go** - Memory fact extraction tools:
- `extractFactsTool` - OpenAI function definition for memory extraction
- Tool argument definitions for extraction
- Category enums and validation schemas

**prompts.go** - LLM prompts:
- `FactExtractionPrompt` - Advanced structured fact extraction with quality thresholds
- Rich examples and category definitions

**storage/storage.go** - Hot-swappable storage abstraction:
- `Interface` - Storage abstraction with document operations
- `WeaviateStorage` - Current implementation with document table + structured fact fields
- `StoreDocument()` - Document storage with deduplication
- `GetStoredDocument()` - Document retrieval from document table
- Schema migration for structured fact fields
- Easy to extend with new backends

## Common Tasks

### Adding a new storage backend

1. Implement `storage.Interface` in a new package:
```go
type RedisStorage struct { ... }

// Core memory operations - now returns MemoryFact, not TextDocument!
func (r *RedisStorage) GetByID(ctx context.Context, id string) (*memory.MemoryFact, error) { ... }
func (r *RedisStorage) Update(ctx context.Context, id string, fact *memory.MemoryFact, vector []float32) error { ... }
func (r *RedisStorage) Query(ctx context.Context, queryText string, filter *memory.Filter, embeddingsModel string) (memory.QueryResult, error) { ... }

// Document storage operations
func (r *RedisStorage) StoreDocument(ctx context.Context, content, docType, originalID string, metadata map[string]string) (string, error) { ... }
func (r *RedisStorage) GetStoredDocument(ctx context.Context, documentID string) (*StoredDocument, error) { ... }

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
2. Add a case in `orchestrator.ProcessDocuments()` 
3. Create extraction logic following existing patterns

### Changing how facts are extracted

Look in `pure.go`:
- `ExtractFactsFromDocument()` - Main extraction router that returns `[]*memory.MemoryFact`
- `extractFactsFromConversation()` - Extracts MemoryFact objects from conversations
- `extractFactsFromTextDocument()` - Extracts MemoryFact objects from text documents

Supporting files:
- Prompts are defined in `prompts.go` (see `FactExtractionPrompt`)
- Tool definitions in `tools.go` (see `extractFactsTool` definition)

### Modifying storage behavior

Check `orchestrator.go`:
- `storeFactsDirectly()` - Direct storage of facts without decision-making
- `ProcessDocuments()` - Main pipeline coordination
- Worker management for parallel processing

### Working with memory facts

```go
// Creating a memory fact
fact := &memory.MemoryFact{
    Category:        "preference",
    Subject:         "user",
    Attribute:       "coffee_type", 
    Value:           "prefers dark roast over light roast",
    TemporalContext: helpers.Ptr("2025-01-15"),
    Sensitivity:     "low",
    Importance:      2,
    Source:          "conversation",
    Timestamp:       time.Now(),
}

// Generate searchable content
content := fact.GenerateContent() 
// Result: "user - prefers dark roast over light roast"

// Query by structured fields
filter := &memory.Filter{
    FactCategory: helpers.Ptr("preference"),
}
facts, err := storage.Query(ctx, "preferences", filter)

// Query by importance
filter := &memory.Filter{
    FactImportance: helpers.Ptr(3), // Critical facts only
}
facts, err := storage.Query(ctx, "critical facts", filter)
```

### Working with document references

```go
// Retrieve document references for investigation
refs, err := storage.GetDocumentReferences(ctx, memoryID)
if err != nil {
    log.Printf("Failed to get document references: %v", err)
    return
}

for i, ref := range refs {
    log.Printf("Document %d: ID=%s, Type=%s", i+1, ref.ID, ref.Type)
    log.Printf("Content preview: %s...", ref.Content[:min(100, len(ref.Content))])
}

// For backward compatibility, single reference access
ref, err := storage.GetDocumentReference(ctx, memoryID)
if err != nil {
    log.Printf("Failed to get primary document reference: %v", err)
    return
}
```

### Debugging issues

1. **Fact extraction failing?** → Check logs in `orchestrator.extractFactsWorker()`
2. **Storage failing?** → Check storage implementation logs and `storeFactsDirectly()` method
3. **Document references empty?** → Verify `GetStoredDocument()` implementation returns content
4. **Deduplication not working?** → Check SHA256 hashing in `StoreDocument()`
5. **Structured facts missing?** → Check if `factCategory`, `factSubject` etc. fields are stored
6. **Categories not recognized?** → Verify category enums in extraction prompts
7. **Performance issues?** → Check worker count and batch size in configuration

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
    StorageTimeout:        30 * time.Second,
    
    // Features
    EnableRichContext:      true,  // Include rich document context
    ParallelFactExtraction: true,  // Process facts in parallel
    StreamingProgress:      true,  // Stream progress updates
}
```

## Testing

The test suite is comprehensive and focuses on different aspects:
- `evolvingmemory_test.go` - Integration tests and main interface
- `orchestrator_test.go` - Coordination logic tests
- `pipeline_test.go` - Pipeline processing tests
- `pure_test.go` - Pure function tests (fast, no external dependencies)
- `adapters_test.go` - Interface compliance and dependency validation

Run with: `go test ./pkg/agent/memory/evolvingmemory/...`

Most tests gracefully skip when AI services aren't configured, allowing for fast iteration.

## Common Patterns

### Processing flow for a conversation:
1. `ConversationDocument` arrives at `StorageImpl.Store()`
2. `MemoryOrchestrator.ProcessDocuments()` coordinates the simplified pipeline
3. Documents are chunked if too large (via `doc.Chunk()`)
4. Document is stored separately in `SourceDocument` table with deduplication
5. `MemoryOrchestrator` extracts **MemoryFact objects**: `{Category: "preference", Subject: "user", Value: "likes pizza", ...}`
6. All extracted facts are stored directly via `storeFactsDirectly()` without decision-making
7. Memory facts are stored with all structured fields directly accessible
8. `MemoryOrchestrator` batches new memories and flushes to storage

### Error handling:
- Each stage returns errors through channels
- Timeouts on all external operations (LLM calls, storage operations)
- Simplified error handling with fewer failure points
- Errors are logged but processing continues for other documents

## Gotchas

1. **Direct storage only** - All facts are stored immediately without decision-making
2. **Facts can be empty** - Not all documents produce extractable facts
3. **Channels close in order** - Progress channel closes before error channel
4. **Storage abstraction** - Don't depend on Weaviate-specific features
5. **Document content vs hash** - Ensure `GetStoredDocument()` returns actual content, not content hash
6. **Multiple references** - Use `GetDocumentReferences()` for complete audit trail
7. **Backward compatibility** - Old format memories have empty content in document references
8. **Structured fact migration** - New installs get structured fact fields, existing schemas get them added automatically
9. **Content generation** - Structured facts generate rich searchable content strings automatically
10. **Performance scaling** - Linear scaling with document size, predictable resource usage

## Architecture Benefits

### Clean Separation of Concerns
- **StorageImpl**: Simple public API, no business logic
- **MemoryOrchestrator**: Coordinates extraction and direct storage
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

### Testability & Simplicity
- Pure functions in `pure.go` are fast to test
- Simplified orchestration logic is easier to test and debug
- Integration tests can use real or mock storage backends
- No global state or hard dependencies
- Predictable performance characteristics

### Backward Compatibility
All existing APIs are preserved:
- `Store()` method works unchanged
- `StoreConversations()` alias maintained
- `Query()` delegated to storage
- Zero breaking changes for existing consumers
- Old memory format automatically enhanced with structured fields where possible

### Quality & Performance Improvements

**Unified Type System:**
- ✅ **Single MemoryFact type** - No more conversions between MemoryFact and MemoryFact
- ✅ **Type safety throughout** - Structured fields properly typed, no string metadata parsing
- ✅ **Direct field access** - All fields accessible without JSON extraction
- ✅ **Clean architecture** - Documents → MemoryFacts → Storage (no intermediate types)

**Memory Extraction:**
- ✅ **10x richer memory data** - Semantic categories, privacy levels, importance scores
- ✅ **Quality-first approach** - Confidence thresholds filter low-quality facts  
- ✅ **Privacy compliance** - Automatic GDPR-ready sensitivity classification
- ✅ **Future-proof querying** - Structured fields enable precise filtering

**Storage & Performance:**
- ✅ **85-95% storage reduction** - Document deduplication + structured references
- ✅ **Faster queries** - Direct field indexing vs JSON pattern matching
- ✅ **Better search precision** - Rich content generation from structured data
- ✅ **Scalable architecture** - Clean separation enables easy backend swapping

## Memory Consolidation System 🧠

The evolvingmemory package includes a **standalone consolidation system** that synthesizes raw memory facts into high-quality consolidated insights.

### Overview

The consolidation system operates independently of the main storage pipeline, allowing for:
- **Scheduled consolidation** - Run consolidation as a separate process
- **Topic-based synthesis** - Consolidate memories around specific themes
- **Quality enhancement** - Transform raw facts into coherent insights
- **Export capabilities** - Generate JSON reports for analysis

### Core Components

```go
// ConsolidationReport - Complete consolidation output
type ConsolidationReport struct {
    Topic             string               `json:"topic"`
    Summary           string               `json:"summary"`           // Narrative overview
    ConsolidatedFacts []*ConsolidationFact `json:"consolidated_facts"` // Enhanced facts
    SourceFactCount   int                  `json:"source_fact_count"`
    GeneratedAt       time.Time            `json:"generated_at"`
}

// ConsolidationFact - Enhanced memory fact with source tracking
type ConsolidationFact struct {
    memory.MemoryFact
    ConsolidatedFrom []string `json:"consolidated_from"` // Source fact IDs
}
```

### Usage Example

```go
// Set up consolidation dependencies
deps := ConsolidationDependencies{
    Logger:             logger,
    Storage:            storage,
    CompletionsService: aiService,
    CompletionsModel:   "gpt-4",
}

// Consolidate memories by tag
report, err := ConsolidateMemoriesByTag(ctx, "health", deps)
if err != nil {
    log.Fatal("Consolidation failed:", err)
}

// Export results
err = report.ExportJSON("health-consolidation-2025-01-15.json")
if err != nil {
    log.Fatal("Export failed:", err)
}

log.Printf("Consolidated %d facts into %d insights", 
    report.SourceFactCount, 
    len(report.ConsolidatedFacts))
```

### Consolidation Storage

The consolidation system includes powerful storage capabilities for persisting consolidated facts back into the memory system:

```go
// Store a single consolidation report
err = StoreConsolidationReport(ctx, report, memoryStorage)
if err != nil {
    log.Fatal("Storage failed:", err)
}

// Store multiple consolidation reports (batch processing)
reports := []*ConsolidationReport{report1, report2, report3}
err = StoreConsolidationReports(ctx, reports, memoryStorage, func(processed, total int) {
    log.Printf("Storage progress: %d/%d", processed, total)
})
if err != nil {
    log.Fatal("Batch storage failed:", err)
}

// Load consolidation reports from comprehensive JSON
reports, err := LoadConsolidationReportsFromJSON("X_3_consolidation_report.json")
if err != nil {
    log.Fatal("Loading failed:", err)
}

// Store the loaded reports
err = StoreConsolidationReports(ctx, reports, memoryStorage, nil)
if err != nil {
    log.Fatal("Storage failed:", err)
}
```

#### Storage Features

- **Structured metadata** - Each consolidated fact includes source tracking and consolidation metadata
- **Batch processing** - Efficiently store multiple reports with progress callbacks
- **Tag augmentation** - Automatically adds "consolidated" and topic tags for easy filtering
- **Summary facts** - Creates special summary facts for narrative overviews
- **Source traceability** - Maintains full audit trail to original facts

### Consolidation Flow

1. **Fact Retrieval** - Fetch all facts tagged with the specified topic
2. **LLM Processing** - Send facts to LLM with consolidation prompt
3. **Synthesis** - LLM generates summary + consolidated facts
4. **Source Tracking** - Link consolidated facts back to original sources
5. **Report Generation** - Package results with metadata
6. **Storage** - Persist consolidated facts back into memory system (optional)

### Architecture Benefits

- **Standalone Operation** - No impact on main storage pipeline
- **Clean Separation** - Pure functions separated from side effects
- **Consistent Patterns** - Follows same LLM tool call patterns as fact extraction
- **Export Ready** - JSON export for analysis and reporting
- **Source Traceability** - Full audit trail from consolidated facts to raw sources
- **Persistent Storage** - Store consolidated insights for enhanced querying

### Querying Consolidated Facts

After storage, consolidated facts can be queried alongside raw facts:

```go
// Query only consolidated facts
filter := &memory.Filter{
    Tags: &memory.TagsFilter{Any: []string{"consolidated"}},
}
result, err := storage.Query(ctx, "user preferences", filter)

// Query facts from specific consolidation topic
filter := &memory.Filter{
    Tags: &memory.TagsFilter{Any: []string{"Family & Close Kin"}},
}
result, err := storage.Query(ctx, "family relationships", filter)

// Query consolidated summaries only
filter := &memory.Filter{
    FactCategory: helpers.Ptr("summary"),
    Tags: &memory.TagsFilter{Any: []string{"consolidated"}},
}
result, err := storage.Query(ctx, "topic summaries", filter)
```

### Future Enhancements

- **Similarity Search** - Include semantically related facts beyond tags
- **Scheduled Processing** - Automated consolidation workflows
- **Multi-topic Synthesis** - Cross-topic relationship discovery
- **Intelligent Querying** - Hybrid raw + consolidated fact ranking