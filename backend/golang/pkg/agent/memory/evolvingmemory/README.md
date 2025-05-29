# Memory System: Unified Document Interface

## TL;DR for Contributors 🚀

The memory system now uses a **unified `Document` interface** that supports both legacy text documents and structured conversations. This means:

- ✅ **All existing code continues to work** (backward compatibility)
- ✅ **New structured conversation data gets rich processing** with facts about all participants
- ✅ **Data sources can be upgraded gradually** (no big bang migration)
- ✅ **Type-safe at compile time** (Go interfaces FTW)
- ✅ **Ultra-simple implementation** - JSON marshaling, static prompts, no over-engineering

### Quick Start

#### Storing Documents

```go
// For simple text (emails, articles, notes)
textDocs := []memory.TextDocument{doc1, doc2}
err := storage.Store(ctx, memory.TextDocumentsToDocuments(textDocs), nil)

// For conversations (WhatsApp, Slack, Discord)  
convDocs := []memory.ConversationDocument{chat1, chat2}
err := storage.Store(ctx, memory.ConversationDocumentsToDocuments(convDocs), nil)

// Mix both types
var allDocs []memory.Document
allDocs = append(allDocs, memory.TextDocumentsToDocuments(textDocs)...)
allDocs = append(allDocs, memory.ConversationDocumentsToDocuments(convDocs)...)
err := storage.Store(ctx, allDocs, nil)
```

#### Processing Documents

The system automatically routes documents based on type:

```go
// Internal routing in Store method:
switch typedDoc := doc.(type) {
case *memory.ConversationDocument:
    // Rich conversation processing: extracts facts about primaryUser AND other participants
    facts := s.extractFactsFromConversation(ctx, *typedDoc, speakerID, currentDate, docDate)
case *memory.TextDocument:
    // Legacy text processing: simple fact extraction
    facts := s.extractFactsFromTextDocument(ctx, *typedDoc, speakerID, currentDate, docDate)
}
```

---

## Overview

The memory system uses a **unified `Document` interface** that allows both `TextDocument` (legacy) and `ConversationDocument` (structured) to be stored and processed seamlessly. This provides:

- ✅ **Backward Compatibility**: Existing text-based ingestion continues to work
- ✅ **Forward Compatibility**: New structured conversation data gets rich processing
- ✅ **Social Network Extraction**: Facts about primaryUser AND their contacts/friends
- ✅ **Gradual Migration**: Data sources can be upgraded one-by-one to structured format
- ✅ **Type Safety**: Go interfaces ensure compile-time safety
- ✅ **Simple Implementation**: No over-engineering, just JSON marshaling and static prompts

## Document Interface

All documents implement this unified interface:

```go
type Document interface {
    ID() string
    Content() string              // Flattened content for search
    Timestamp() *time.Time
    Tags() []string
    Metadata() map[string]string
    Source() string
}
```

## Document Types

### TextDocument (Legacy) 
**Use for:** Single-author content, unstructured text, quick integration

```go
type TextDocument struct {
    FieldID        string            `json:"id"`
    FieldContent   string            `json:"content"`
    FieldTimestamp *time.Time        `json:"timestamp"`
    FieldSource    string            `json:"source,omitempty"`
    FieldTags      []string          `json:"tags,omitempty"`
    FieldMetadata  map[string]string `json:"metadata,omitempty"`
}
```

**Use Cases:**
- Email content (non-threaded)
- Article/blog content
- Notes and documents
- Social media posts (single author)
- Any unstructured text data

### ConversationDocument (Structured)
**Use for:** Multi-participant conversations, structured chat data

```go
type ConversationDocument struct {
    FieldID       string                `json:"id"`
    FieldSource   string                `json:"source"`       // "whatsapp", "slack", etc.
    People        []string              `json:"people"`       // All participants
    User          string                `json:"user"`         // Primary user
    Conversation  []ConversationMessage `json:"conversation"` // Messages
    FieldTags     []string              `json:"tags,omitempty"`
    FieldMetadata map[string]string     `json:"metadata,omitempty"`
}

type ConversationMessage struct {
    Speaker string    `json:"speaker"`  // Must be in People array
    Content string    `json:"content"`  // Message text
    Time    time.Time `json:"time"`     // When sent
}
```

**Use Cases:**
- WhatsApp chats
- Slack conversations
- Discord threads
- Email threads (when parsed)
- Any multi-participant conversation

## Storage Interface

The unified storage interface accepts any document type:

```go
type Storage interface {
    Store(ctx context.Context, documents []Document, progressCallback ProgressCallback) error
    Query(ctx context.Context, query string) (QueryResult, error)
}
```

## Helper Functions

Convert slices to the unified interface:

```go
// Convert TextDocument slice
docs := memory.TextDocumentsToDocuments(textDocs)
err := storage.Store(ctx, docs, nil)

// Convert ConversationDocument slice  
docs := memory.ConversationDocumentsToDocuments(convDocs)
err := storage.Store(ctx, docs, nil)
```

## Processing Logic (Simplified!)

The memory system automatically handles both document types with **ultra-simple** processing:

```go
func (s *WeaviateStorage) Store(ctx context.Context, documents []Document, progressCallback ProgressCallback) error {
    for _, doc := range documents {
        switch typedDoc := doc.(type) {
        case *memory.ConversationDocument:
            // Rich conversation processing - extracts facts about ALL participants
            conversationJSON, _ := normalizeAndFormatConversation(*typedDoc)
            // Send JSON + static prompt to LLM - no templating, no complex context building
            facts := s.extractFactsFromConversation(ctx, *typedDoc, speakerID, currentDate, docDate)
            
        case *memory.TextDocument:
            // Legacy text processing
            facts := s.extractFactsFromTextDocument(ctx, *typedDoc, speakerID, currentDate, docDate)
        }
    }
}
```

### Conversation Processing Details

**What makes it simple:**
1. **Normalize**: Replace primary user name with "primaryUser" 
2. **JSON Marshal**: Convert entire conversation to clean JSON
3. **Static Prompt**: No templating, no complex context building
4. **LLM Call**: Send JSON + prompt, get facts about ALL participants

```go
// Ultra-simple conversation processing
func normalizeAndFormatConversation(convDoc memory.ConversationDocument) (string, error) {
    // Replace primary user name with "primaryUser"
    normalized := replaceUserNames(convDoc)
    
    // Just JSON marshal the whole thing - done!
    return json.MarshalIndent(normalized, "", "  ")
}
```

**What the LLM receives:**
```json
{
  "id": "chat_001",
  "source": "whatsapp", 
  "people": ["primaryUser", "Alice", "Bob"],
  "user": "primaryUser",
  "conversation": [
    {
      "speaker": "primaryUser",
      "content": "Hey everyone, want to grab dinner?",
      "time": "2024-01-14T14:30:15Z"
    },
    {
      "speaker": "Alice", 
      "content": "Sure! Where were you thinking?",
      "time": "2024-01-14T14:31:02Z"
    }
  ]
}
```

### Fact Extraction (Comprehensive Social Network)

The system extracts facts about **ALL participants**, not just the primary user:

**🔥 PRIMARY FOCUS - primaryUser** (extensive):
- Direct facts: what they said, felt, planned, experienced
- Interaction facts: how others responded to them
- Conversation facts: their role, outcomes involving them

**👥 SECONDARY FOCUS - Other Participants** (important details):
- Personal info they shared (work, family, interests)
- Their preferences, opinions, experiences
- Their relationship context with primaryUser
- Plans, activities, commitments they mentioned
- Life events and updates they shared

**🤝 RELATIONSHIP FACTS**:
- How each person relates to primaryUser
- Social dynamics between all participants
- Shared experiences or connections
- Communication patterns

## Migration Strategy

### Phase 1: Interface Implementation ✅ COMPLETE
- [x] Create unified `Document` interface
- [x] Implement interface on both document types
- [x] Update storage to accept `[]Document`
- [x] Add helper conversion functions
- [x] Update all existing callers
- [x] Simplify fact extraction (remove over-engineering)

### Phase 2: Gradual Data Source Migration (Future)
Upgrade data sources one-by-one to produce `ConversationDocument`:

```go
// Before (produces TextDocument)
func ProcessWhatsApp(data []byte) []memory.TextDocument {
    // Parse and flatten to text
}

// After (produces ConversationDocument)  
func ProcessWhatsApp(data []byte) []memory.ConversationDocument {
    // Parse into structured conversations
}
```

**Priority Order:**
1. **WhatsApp** → Already structured, easy win
2. **Slack** → Rich threading, high value  
3. **Discord** → Similar to Slack
4. **Email** → When threading available
5. **Telegram** → Chat-based conversations

### Phase 3: Enhanced Processing (Future)
- Advanced relationship mapping
- Temporal conversation analysis
- Cross-conversation participant tracking
- Sentiment analysis per participant

## Benefits by Document Type

| Feature | TextDocument | ConversationDocument |
|---------|-------------|---------------------|
| Backward Compatible | ✅ | ✅ |
| Quick Integration | ✅ | ⚠️ Requires parsing |
| Speaker Analysis | ❌ | ✅ |
| Social Network Facts | ❌ | ✅ |
| Relationship Tracking | ❌ | ✅ |
| Temporal Analysis | ❌ | ✅ |
| Rich Fact Extraction | ⚠️ Basic | ✅ Advanced |
| Implementation Complexity | ✅ Simple | ✅ Simple (now!) |

### TextDocument Processing
- ✅ Backward compatible
- ✅ Simple content indexing
- ✅ Basic fact extraction
- ⚠️ Limited context understanding

### ConversationDocument Processing  
- ✅ Rich speaker-specific facts
- ✅ **Social network extraction** (facts about all participants)
- ✅ Conversation context preservation
- ✅ Multi-participant relationship tracking
- ✅ Temporal message analysis
- ✅ Source-aware processing
- ✅ **Ultra-simple implementation** (no over-engineering)

## Usage Examples

### Storing Mixed Document Types

```go
// Mix of document types
var docs []memory.Document

// Add some text documents
textDocs := []memory.TextDocument{emailDoc, articleDoc}
docs = append(docs, memory.TextDocumentsToDocuments(textDocs)...)

// Add some conversation documents  
convDocs := []memory.ConversationDocument{whatsappConv, slackThread}
docs = append(docs, memory.ConversationDocumentsToDocuments(convDocs)...)

// Store everything together
err := storage.Store(ctx, docs, nil)
```

### Data Source Integration Patterns

```go
// Option 1: Start simple (TextDocument)
func ProcessDataSource(rawData []byte) []memory.TextDocument {
    // Quick and dirty: flatten to text
    return []memory.TextDocument{
        {FieldID: "doc1", FieldContent: "flattened content"},
    }
}

// Option 2: Rich integration (ConversationDocument)  
func ProcessDataSource(rawData []byte) []memory.ConversationDocument {
    // Parse structure: speakers, timing, etc.
    return []memory.ConversationDocument{
        {
            FieldID: "conv1",
            FieldSource: "my_platform",
            People: []string{"Alice", "Bob"},
            User: "Alice",
            Conversation: []memory.ConversationMessage{
                {Speaker: "Alice", Content: "Hello", Time: time.Now()},
                {Speaker: "Bob", Content: "Hi there", Time: time.Now()},
            },
        },
    }
}
```

### Storage Calls

```go
// Before (old way - still works!)
textDocs := processDataAsText(data)
err := storage.Store(ctx, memory.TextDocumentsToDocuments(textDocs), nil)

// After (new way - when ready)
convDocs := processDataAsConversations(data)  
err := storage.Store(ctx, memory.ConversationDocumentsToDocuments(convDocs), nil)
```

## Best Practices

### For Data Source Developers

1. **Choose the Right Type**:
   - Use `ConversationDocument` for multi-participant conversations
   - Use `TextDocument` for single-author content or unstructured text

2. **Gradual Migration**:
   - Start with `TextDocument` for quick integration
   - Upgrade to `ConversationDocument` when conversation structure is available

3. **Consistent Metadata**:
   - Use consistent metadata keys across document types
   - Include source information in metadata

### For Memory System Developers

1. **Type Checking**:
   ```go
   switch typedDoc := doc.(type) {
   case *memory.ConversationDocument:
       // Handle rich conversation logic
   case *memory.TextDocument:
       // Handle simple text logic
   }
   ```

2. **Graceful Degradation**:
   - Always provide fallback processing for `TextDocument`
   - Don't assume conversation structure exists

3. **Performance**:
   - Batch process documents of the same type when possible
   - Use type discrimination efficiently

## What We Simplified

The memory system used to be over-engineered with complex context building, template replacement, and elaborate data structures. **We fixed that!**

### Before (Complex) ❌
- `ConversationContext` struct with 8+ fields
- `EnrichedMessage` with timing calculations
- `buildConversationContext()` - 50+ lines of complexity
- Template replacement with 10+ variables
- Custom text formatting
- JSON marshaling of complex context objects

### After (Simple) ✅  
- **One function**: `normalizeAndFormatConversation()`
- **Static prompt**: No templating whatsoever
- **JSON marshal**: Direct conversion of conversation structure
- **~30 lines** vs 150+ lines before

**Result**: Same powerful conversation-aware fact extraction, but **90% less complexity**!

## Key Files

- `pkg/agent/memory/memory.go` - Interface definitions
- `pkg/agent/memory/evolvingmemory/store.go` - Unified storage logic
- `pkg/agent/memory/evolvingmemory/factextraction.go` - Simplified fact extraction
- `pkg/agent/memory/evolvingmemory/prompts.go` - Static prompts (no templating)

## Summary

The interface approach gives us the best of both worlds:
- **Existing code keeps working** (no breaking changes)
- **New code gets rich features** (when using ConversationDocument)
- **Social network extraction** (facts about all participants)
- **Ultra-simple implementation** (no over-engineering)
- **Gradual migration** (upgrade data sources one by one)
- **Type safety** (compile-time guarantees)

This design enables a smooth transition from simple text-based memory to rich, structured conversation memory while maintaining full backward compatibility and keeping the implementation refreshingly simple! 🎉 