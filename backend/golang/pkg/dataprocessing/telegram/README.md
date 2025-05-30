# Telegram Data Processing

## Overview

The Telegram data processor extracts messages, contacts, and user information from Telegram export files. It supports automatic username extraction and comprehensive user metadata capture.

## Features

### ✅ Automatic Username Extraction
- Extracts username from `personal_information.username` field
- Captures user ID, first name, last name, phone number, and bio
- Stores username information in database for reuse
- Smart fallback to provided username if extraction fails

### ✅ Message Processing
- Processes all messages from chats
- Correctly identifies user's own messages using extracted username
- Handles text entities and message metadata
- Supports forwarded and saved messages

### ✅ Contact Processing
- Extracts contact information with timestamps
- Includes first name, last name, and phone numbers

## Usage

### Basic Processing (Legacy)
```go
source := telegram.New()
records, err := source.ProcessFile("telegram_export.json", "username")
```

### Enhanced Processing with Username Extraction
```go
import (
    "context"
    "github.com/EternisAI/enchanted-twin/pkg/dataprocessing/telegram"
    "github.com/EternisAI/enchanted-twin/pkg/db"
)

ctx := context.Background()
store, err := db.NewStore(ctx, "app.db")
if err != nil {
    log.Fatal(err)
}
defer store.Close()

source := telegram.New()

// Process with automatic username extraction
records, err := source.ProcessFileWithStore(ctx, store, "telegram_export.json", "")
if err != nil {
    log.Fatal(err)
}

// Retrieve extracted username
sourceUsername, err := store.GetSourceUsername(ctx, "telegram")
if err != nil {
    log.Fatal(err)
}

if sourceUsername != nil {
    fmt.Printf("Extracted username: %s\n", sourceUsername.Username)
}
```

### Using with DataProcessing Interface
```go
import "github.com/EternisAI/enchanted-twin/pkg/dataprocessing"

success, err := dataprocessing.ProcessSourceWithStore(
    ctx,
    store,
    "telegram",
    "telegram_export.json",
    "output.json",
    "",  // Username will be extracted automatically
    nil,
    "",
)
```

## Expected File Format

Telegram exports should include a `personal_information` section:

```json
{
  "personal_information": {
    "user_id": 1601587058,
    "first_name": "JohnDoe",
    "last_name": "",
    "phone_number": "+33 6 16 87 45 98",
    "username": "@JohnDoe",
    "bio": "User bio"
  },
  "contacts": {
    "about": "Contact list description",
    "list": [
      {
        "first_name": "Contact",
        "last_name": "Name",
        "phone_number": "+1 555 0123",
        "date": "2023-01-15T10:30:00",
        "date_unixtime": "1673776200"
      }
    ]
  },
  "chats": {
    "about": "Chat list description",
    "list": [
      {
        "type": "personal_chat",
        "id": 123456,
        "name": "Chat Name",
        "messages": [
          {
            "id": 1,
            "type": "message",
            "date": "2023-01-15T10:30:00",
            "date_unixtime": "1673776200",
            "from": "JohnDoe",
            "from_id": "user1601587058",
            "text_entities": [
              {
                "type": "plain",
                "text": "Message content"
              }
            ]
          }
        ]
      }
    ]
  }
}
```

## Output Records

### Contact Record
```json
{
  "data": {
    "type": "contact",
    "firstName": "Alice",
    "lastName": "Smith",
    "phoneNumber": "+1 555 0123"
  },
  "timestamp": "2023-01-15T10:30:00Z",
  "source": "telegram"
}
```

### Message Record
```json
{
  "data": {
    "type": "message",
    "messageId": 1,
    "messageType": "message",
    "from": "JohnDoe",
    "to": "Alice Smith",
    "text": "Hello Alice!",
    "chatType": "personal_chat",
    "chatId": 123456,
    "myMessage": true
  },
  "timestamp": "2023-01-15T10:30:00Z",
  "source": "telegram"
}
```

## Username Storage

When using `ProcessFileWithStore`, extracted usernames are stored in the `source_usernames` table:

```sql
SELECT * FROM source_usernames WHERE source = 'telegram';
```

Returns:
- `id` - Unique identifier
- `source` - Always "telegram"
- `username` - Extracted username (e.g., "@JohnDoe")
- `user_id` - Telegram user ID
- `first_name` - User's first name
- `last_name` - User's last name (if available)
- `phone_number` - User's phone number
- `bio` - User's bio/description
- `created_at` - When first stored
- `updated_at` - When last updated

## Testing

Run the comprehensive test suite:

```bash
go test ./pkg/dataprocessing/telegram
```

Tests include:
- Username extraction functionality
- Message attribution accuracy
- Fallback behavior when username is missing
- Database integration
- Example usage patterns

## Error Handling

- **Missing personal_information**: Processing continues, no username stored
- **Invalid JSON**: Returns parsing error
- **Missing username**: Falls back to provided username parameter
- **Database errors**: Logged as warnings, processing continues
- **Invalid timestamps**: Logged as warnings, record skipped

## Performance

- Processes large Telegram exports efficiently
- Minimal memory footprint with streaming JSON parsing
- Database operations are batched where possible
- Timestamp parsing optimized with multiple format support 