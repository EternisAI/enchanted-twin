# Memory System: Unified Document Interface

## TL;DR for Contributors 🚀

The memory system now uses a **unified `Document` interface** that supports both legacy text documents and structured conversations. This means:

- ✅ **All existing code continues to work** (backward compatibility)
- ✅ **New structured conversation data gets rich processing** 
- ✅ **Data sources can be upgraded gradually** (no big bang migration)
- ✅ **Type-safe at compile time** (Go interfaces FTW)

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

```go
for _, doc := range documents {
    if doc.IsConversation() {
        conv, _ := doc.AsConversation()
        // Rich conversation processing: speaker analysis, relationships, etc.
        processConversation(conv)
    } else {
        text, _ := doc.AsText()
        // Simple text processing: keywords, sentiment, etc.
        processText(text)
    }
}
```

---

## Overview

The memory system uses a **unified `Document` interface** that allows both `TextDocument` (legacy) and `ConversationDocument` (structured) to be stored and processed seamlessly. This provides:

- ✅ **Backward Compatibility**: Existing text-based ingestion continues to work
- ✅ **Forward Compatibility**: New structured conversation data gets rich processing
- ✅ **Gradual Migration**: Data sources can be upgraded one-by-one to structured format
- ✅ **Type Safety**: Go interfaces ensure compile-time safety

## Document Interface

All documents implement this unified interface:

```go
type Document interface {
    GetID() string
    GetContent() string              // Flattened content for search
    GetTimestamp() *time.Time
    GetTags() []string
    GetMetadata() map[string]string
    
    // Type discrimination methods
    IsConversation() bool
    AsConversation() (*ConversationDocument, bool)
    AsText() (*TextDocument, bool)
}
```

## Document Types

### TextDocument (Legacy) 
**Use for:** Single-author content, unstructured text, quick integration

```go
type TextDocument struct {
    ID        string            `json:"id"`
    Content   string            `json:"content"`
    Timestamp *time.Time        `json:"timestamp,omitempty"`
    Tags      []string          `json:"tags,omitempty"`
    Metadata  map[string]string `json:"metadata,omitempty"`
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
    ID           string                 `json:"id"`
    Conversation StructuredConversation `json:"conversation"`
    Tags         []string               `json:"tags,omitempty"`
    Metadata     map[string]string      `json:"metadata,omitempty"`
}

type StructuredConversation struct {
    Source       string                `json:"source"`        // "whatsapp", "slack", etc.
    People       []string              `json:"people"`        // All participants
    User         string                `json:"user"`          // Primary user
    Conversation []ConversationMessage `json:"conversation"`  // Messages
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
    Store(ctx context.Context, documents []Document, progressChan chan<- ProgressUpdate) error
    Query(ctx context.Context, query string) (QueryResult, error)
}
```

## Helper Functions

Convert slices to the unified interface:

```go
// Convert TextDocument slice
docs := memory.TextDocumentsToDocuments(textDocs)
err := storage.Store(ctx, docs, progressChan)

// Convert ConversationDocument slice  
docs := memory.ConversationDocumentsToDocuments(convDocs)
err := storage.Store(ctx, docs, progressChan)
```

## Processing Logic

The memory system automatically handles both document types:

```go
func (s *WeaviateStorage) Store(ctx context.Context, documents []Document, progressChan chan<- ProgressUpdate) error {
    for _, doc := range documents {
        if doc.IsConversation() {
            // Rich conversation processing
            convDoc, _ := doc.AsConversation()
            facts := s.extractFactsFromConversation(ctx, *convDoc, currentDate)
            s.updateMemories(ctx, facts, *convDoc)
        } else {
            // Simple text processing
            textDoc, _ := doc.AsText()
            // Convert to simple conversation for consistent processing
            simpleConv := convertTextToSimpleConversation(*textDoc)
            facts := s.extractFactsFromConversation(ctx, simpleConv, currentDate)
            s.updateMemories(ctx, facts, simpleConv)
        }
    }
}
```

## Migration Strategy

### Phase 1: Interface Implementation ✅ COMPLETE
- [x] Create unified `Document` interface
- [x] Implement interface on both document types
- [x] Update storage to accept `[]Document`
- [x] Add helper conversion functions
- [x] Update all existing callers

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
- Speaker-specific fact extraction
- Conversation context analysis
- Relationship mapping between participants
- Temporal conversation analysis

## Benefits by Document Type

| Feature | TextDocument | ConversationDocument |
|---------|-------------|---------------------|
| Backward Compatible | ✅ | ✅ |
| Quick Integration | ✅ | ⚠️ Requires parsing |
| Speaker Analysis | ❌ | ✅ |
| Conversation Context | ❌ | ✅ |
| Relationship Tracking | ❌ | ✅ |
| Temporal Analysis | ❌ | ✅ |
| Rich Fact Extraction | ⚠️ Basic | ✅ Advanced |

### TextDocument Processing
- ✅ Backward compatible
- ✅ Simple content indexing
- ✅ Basic fact extraction
- ⚠️ Limited context understanding

### ConversationDocument Processing  
- ✅ Rich speaker-specific facts
- ✅ Conversation context preservation
- ✅ Multi-participant relationship tracking
- ✅ Temporal message analysis
- ✅ Source-aware processing

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
err := storage.Store(ctx, docs, progressChan)
```

### Type-Specific Processing

```go
for _, doc := range documents {
    if doc.IsConversation() {
        conv, _ := doc.AsConversation()
        log.Printf("Processing conversation with %d participants", len(conv.Conversation.People))
        
        // Rich conversation analysis
        for _, msg := range conv.Conversation.Conversation {
            analyzeSpeakerSentiment(msg.Speaker, msg.Content)
        }
    } else {
        text, _ := doc.AsText()
        log.Printf("Processing text document: %s", text.ID)
        
        // Simple text analysis
        extractKeywords(text.Content)
    }
}
```

### Data Source Integration Patterns

```go
// Option 1: Start simple (TextDocument)
func ProcessDataSource(rawData []byte) []memory.TextDocument {
    // Quick and dirty: flatten to text
    return []memory.TextDocument{
        {ID: "doc1", Content: "flattened content"},
    }
}

// Option 2: Rich integration (ConversationDocument)  
func ProcessDataSource(rawData []byte) []memory.ConversationDocument {
    // Parse structure: speakers, timing, etc.
    return []memory.ConversationDocument{
        {
            ID: "conv1",
            Conversation: memory.StructuredConversation{
                Source: "my_platform",
                People: []string{"Alice", "Bob"},
                User: "Alice",
                Conversation: []memory.ConversationMessage{
                    {Speaker: "Alice", Content: "Hello", Time: time.Now()},
                    {Speaker: "Bob", Content: "Hi there", Time: time.Now()},
                },
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

### Type-Safe Processing

```go
func processDocument(doc memory.Document) error {
    // Always safe - no casting needed
    id := doc.GetID()
    content := doc.GetContent()
    
    // Type-specific processing
    if doc.IsConversation() {
        conv, ok := doc.AsConversation()
        if !ok {
            return fmt.Errorf("failed to convert to conversation")
        }
        
        // Rich processing
        for _, msg := range conv.Conversation.Conversation {
            analyzeSpeaker(msg.Speaker, msg.Content)
        }
    } else {
        text, ok := doc.AsText()
        if !ok {
            return fmt.Errorf("failed to convert to text")
        }
        
        // Simple processing
        extractKeywords(text.Content)
    }
    
    return nil
}
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
   if doc.IsConversation() {
       // Handle rich conversation logic
   } else {
       // Handle simple text logic
   }
   ```

2. **Graceful Degradation**:
   - Always provide fallback processing for `TextDocument`
   - Don't assume conversation structure exists

3. **Performance**:
   - Batch process documents of the same type when possible
   - Use type discrimination efficiently

## Error Handling

```go
// Safe type conversion
if conv, ok := doc.AsConversation(); ok {
    // Process conversation
    processConversation(conv)
} else if text, ok := doc.AsText(); ok {
    // Process text
    processText(text)
} else {
    return fmt.Errorf("unknown document type")
}
```

## Future Enhancements

The interface-based approach enables future document types:

- `EmailThreadDocument` - Rich email threading
- `VideoCallDocument` - Transcribed video calls  
- `DocumentCollectionDocument` - Related document sets
- `TimelineDocument` - Chronological event sequences

Each new type would implement the `Document` interface and get automatic storage support.

---

## ConversationDocument Format Details

### Core Structure

```json
{
  "id": "unique_conversation_identifier",
  "conversation": {
    "source": "source_system_name",
    "people": ["person1", "person2", "..."],
    "user": "primary_user_name",
    "conversation": [
      {
        "speaker": "speaker_name",
        "content": "message_content",
        "time": "2024-01-15T10:30:00Z"
      }
    ]
  },
  "tags": ["tag1", "tag2"],
  "metadata": {
    "key": "value"
  }
}
```

### Field Descriptions

#### Root Level
- `id` (string, required): Unique identifier for the conversation document
- `conversation` (object, required): The structured conversation data
- `tags` (array, optional): Tags for categorization
- `metadata` (object, optional): Additional key-value metadata

#### Conversation Object
- `source` (string, required): Identifier for the source system (e.g., "slack", "discord", "whatsapp")
- `people` (array, required): List of all participants in the conversation
- `user` (string, required): The primary user (must be included in the `people` array)
- `conversation` (array, required): Array of conversation messages

#### Message Object
- `speaker` (string, required): Name of the message sender (must be in `people` array)
- `content` (string, required): The message content
- `time` (timestamp, required): When the message was sent (RFC3339 format)

## Benefits Over Previous Format

### Old Format Issues
```
Content: "Alice: I love pizza\\nBob: Me too\\nAlice: Especially margherita"
Metadata: {"dataset_speaker_a": "Alice", "dataset_speaker_b": "Bob"}
```

**Problems:**
- Implicit parsing with `\\n` and `:` separators
- No per-message timestamps
- Limited to 2 speakers
- Error-prone parsing
- No source tracking
- Ambiguous speaker identification

### New Format Advantages

✅ **Explicit Structure**: No parsing required, direct JSON access  
✅ **Per-Message Timestamps**: Precise timing information  
✅ **Multiple Speakers**: Support for any number of participants  
✅ **Validation**: Built-in validation ensures data integrity  
✅ **Source Tracking**: Clear identification of conversation origin  
✅ **User Identification**: Explicit primary user designation  
✅ **Schema-Friendly**: Compatible with JSON Schema validation  
✅ **Extensible**: Easy to add new fields without breaking changes  

## Detailed Examples

### Basic Two-Person Conversation

```json
{
  "id": "chat_001",
  "conversation": {
    "source": "messaging_app",
    "people": ["Alice", "Bob"],
    "user": "Alice",
    "conversation": [
      {
        "speaker": "Alice",
        "content": "Hey, want to grab lunch?",
        "time": "2024-01-15T12:00:00Z"
      },
      {
        "speaker": "Bob",
        "content": "Sure! How about that new pizza place?",
        "time": "2024-01-15T12:01:00Z"
      },
      {
        "speaker": "Alice",
        "content": "Perfect! I love pizza. See you at 1pm?",
        "time": "2024-01-15T12:02:00Z"
      }
    ]
  },
  "tags": ["food", "plans"],
  "metadata": {
    "platform": "whatsapp",
    "session_type": "casual_chat"
  }
}
```

### Multi-Person Group Chat

```json
{
  "id": "team_standup_001",
  "conversation": {
    "source": "slack",
    "people": ["Alice", "Bob", "Charlie", "Diana"],
    "user": "Alice",
    "conversation": [
      {
        "speaker": "Alice",
        "content": "Good morning team! Ready for standup?",
        "time": "2024-01-15T09:00:00Z"
      },
      {
        "speaker": "Bob",
        "content": "Yes! I finished the API integration yesterday.",
        "time": "2024-01-15T09:01:00Z"
      },
      {
        "speaker": "Charlie",
        "content": "Great work Bob! I'm working on the frontend today.",
        "time": "2024-01-15T09:02:00Z"
      },
      {
        "speaker": "Diana",
        "content": "I'll be reviewing the test cases this morning.",
        "time": "2024-01-15T09:03:00Z"
      }
    ]
  },
  "tags": ["work", "standup", "team"],
  "metadata": {
    "platform": "slack",
    "channel": "dev-team",
    "meeting_type": "daily_standup"
  }
}
```

## Validation Rules

The system enforces these validation rules:

1. **Required Fields**: `id`, `source`, `people`, `user`, `conversation`
2. **User in People**: The `user` must be included in the `people` array
3. **Speaker Validation**: Each message `speaker` must be in the `people` array
4. **Non-Empty Content**: Message `content` cannot be empty
5. **Valid Timestamps**: Message `time` must be a valid timestamp
6. **Minimum Messages**: At least one message must be present

## Error Handling

Common validation errors and their meanings:

- `"document ID is required"`: Missing or empty `id` field
- `"user 'X' must be included in the people list"`: User not in people array
- `"message N: speaker 'X' must be included in the people list"`: Invalid speaker
- `"conversation must contain at least one message"`: Empty conversation array

## Best Practices

1. **Consistent Speaker Names**: Use the same name format throughout
2. **Proper Timestamps**: Use RFC3339 format with timezone information
3. **Meaningful IDs**: Use descriptive, unique conversation identifiers
4. **Source Tracking**: Always specify the source system
5. **Validation First**: Always validate before storing
6. **Error Handling**: Handle validation errors gracefully

## Future Enhancements

Potential future additions to the format:

- Message threading/replies
- Message reactions/emojis
- File attachments metadata
- Message editing history
- Read receipts/status
- Message priority levels
- Custom message types (system messages, etc.)

The structured format is designed to be extensible, allowing these features to be added without breaking existing implementations.

---

## Key Files

- `pkg/agent/memory/memory.go` - Interface definitions
- `pkg/agent/memory/evolvingmemory/store.go` - Unified storage logic

## Summary

The interface approach gives us the best of both worlds:
- **Existing code keeps working** (no breaking changes)
- **New code gets rich features** (when using ConversationDocument)
- **Gradual migration** (upgrade data sources one by one)
- **Type safety** (compile-time guarantees)

This design enables a smooth transition from simple text-based memory to rich, structured conversation memory while maintaining full backward compatibility! 🎉 