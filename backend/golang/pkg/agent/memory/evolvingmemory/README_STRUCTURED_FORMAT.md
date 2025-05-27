# Structured Conversation Format

This document describes the new explicit, structured conversation format for the memory system, replacing the previous implicit string-based format.

## Overview

The new format uses a well-defined JSON structure that eliminates parsing ambiguity and provides explicit metadata about conversations, speakers, and timing.

## Format Specification

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

## Usage Examples

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

## Implementation

### Go Types

```go
type ConversationMessage struct {
    Speaker string    `json:"speaker"`
    Content string    `json:"content"`
    Time    time.Time `json:"time"`
}

type StructuredConversation struct {
    Source       string                `json:"source"`
    People       []string              `json:"people"`
    User         string                `json:"user"`
    Conversation []ConversationMessage `json:"conversation"`
}

type ConversationDocument struct {
    ID           string                 `json:"id"`
    Conversation StructuredConversation `json:"conversation"`
    Tags         []string               `json:"tags,omitempty"`
    Metadata     map[string]string      `json:"metadata,omitempty"`
}
```

### Storage Interface

```go
type StructuredStorage interface {
    StoreConversations(ctx context.Context, documents []ConversationDocument, progressChan chan<- ProgressUpdate) error
    Query(ctx context.Context, query string) (QueryResult, error)
}
```

### Usage

```go
// Parse from JSON
doc, err := ParseStructuredConversationFromJSON(jsonData)
if err != nil {
    return err
}

// Validate
if err := ValidateConversationDocument(doc); err != nil {
    return err
}

// Store
docs := []memory.ConversationDocument{*doc}
err = storage.StoreConversations(ctx, docs, progressChan)
```

## Validation Rules

The system enforces these validation rules:

1. **Required Fields**: `id`, `source`, `people`, `user`, `conversation`
2. **User in People**: The `user` must be included in the `people` array
3. **Speaker Validation**: Each message `speaker` must be in the `people` array
4. **Non-Empty Content**: Message `content` cannot be empty
5. **Valid Timestamps**: Message `time` must be a valid timestamp
6. **Minimum Messages**: At least one message must be present

## Migration from Old Format

The system provides backward compatibility through automatic conversion:

```go
// Convert structured to legacy format
legacyDoc := conversationDoc.ToTextDocument()

// Use existing storage methods
storage.Store(ctx, []memory.TextDocument{legacyDoc}, progressChan)
```

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