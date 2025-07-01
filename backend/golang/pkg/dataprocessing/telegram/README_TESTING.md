# Telegram Data Processing Testing Guide

This guide explains how to test and debug Telegram data processing functionality, including how to repeat the test that resolved the "null output" issue.

## Prerequisites

1. **Sample Data**: Ensure you have telegram export data at `input_data/telegram_export_sample.json`
2. **Go Backend**: The backend server should be buildable and runnable
3. **GraphQL Access**: Access to the GraphQL endpoint for triggering data processing

## Setup

### 1. Build the Backend

```bash
cd backend/golang
make build
```

### 2. Prepare Test Data

Ensure your telegram export file is available:
```bash
ls -la input_data/telegram_export_sample.json
```

The sample data should contain multiple participants and various message types.

### 3. Start the Backend Server

```bash
cd backend/golang
go run cmd/server/main.go
```

The server will start on port 44999 (or as configured). Look for:
- NATS server starting
- Temporal worker starting  
- GraphQL server ready at `/query`

## Testing Process

### 1. Add Data Source via GraphQL

Use the GraphQL endpoint at `http://localhost:44999/query` to add a telegram data source:

```graphql
mutation {
  addDataSource(input: {
    name: "Telegram"
    path: "input_data/telegram_export_sample.json"
  }) {
    id
    name
    path
    isIndexed
    hasError
  }
}
```

### 2. Start Indexing Workflow

Trigger the data processing workflow:

```graphql
mutation {
  startIndexingData {
    success
    message
  }
}
```

### 3. Monitor Processing

Watch the server logs for:

#### Data Processing Phase
```
Processing Telegram data source: input_data/telegram_export_sample.json
Found X chats in telegram export
Processing chat: {chat_name}
Found X messages in chat
Processing conversation with X messages
```

#### Indexing Phase
```
Indexing batch size Telegram 3
Batch indexed successfully dataSource=Telegram batch=1 total=3 documentsStored=X
```

### 4. Verify Output

Check the generated JSONL file:
```bash
ls -la /path/to/app/data/Telegram_*.jsonl
cat /path/to/app/data/Telegram_*.jsonl | head -5
```

The output should contain actual conversation data, not `null` values.

## Expected Results

### Successful Processing

**Logs should show:**
- Chat type detection (private_supergroup, private, etc.)
- Message processing with participant detection
- Conversation extraction with multiple participants
- Successful indexing with document storage

**Output file should contain:**
- JSON objects with conversation data
- Real message content and timestamps
- Participant information
- Conversation metadata

### Sample Output Structure
```json
{
  "id": "conversation_id",
  "content": "extracted conversation text",
  "metadata": {
    "source": "telegram",
    "participants": ["User1", "User2"],
    "timestamp": "2024-01-01T12:00:00Z"
  }
}
```

## Common Issues and Debugging

### Issue 1: "No batches found for data source"

**Symptoms:**
- Log shows: `No batches found for data source dataSource=Telegram`
- `totalDocuments=0` in processing completion

**Debugging Steps:**
1. Check if filters are skipping all conversations:
   ```bash
   grep -A 5 -B 5 "private_supergroup" backend/golang/pkg/dataprocessing/telegram/telegram.go
   ```

2. Look for "only user messages" filter logs:
   ```bash
   # Check logs for participant detection
   grep "people=" server_logs.txt
   ```

3. Verify chat processing:
   ```bash
   # Look for chat type filtering
   grep "chat.Type" server_logs.txt
   ```

### Issue 2: All messages attributed to same user

**Symptoms:**
- Processing succeeds but all messages show same speaker
- Participant detection shows only 1 person

**Check:**
- Message attribution logic in `extractConversationFromMessages`
- `msg.From` field mapping
- Username resolution

### Issue 3: NATS Connection Issues

**Symptoms:**
- `NATS connection is not connected`
- Port binding errors

**Solutions:**
1. Check if NATS port is already in use:
   ```bash
   lsof -i :4222
   ```

2. Kill existing NATS processes:
   ```bash
   pkill -f nats-server
   ```

## Debug Mode

To enable detailed logging, add debug statements:

```go
// In telegram.go processChat function
log.Printf("DEBUG: Processing chat %s with type %s", chat.Name, chat.Type)
log.Printf("DEBUG: Found %d messages in chat", len(chat.Messages))
log.Printf("DEBUG: Participant detection: people=%v peopleCount=%d", people, len(people))
```

## File Locations

- **Source data**: `input_data/telegram_export_sample.json`
- **Processor code**: `backend/golang/pkg/dataprocessing/telegram/telegram.go`
- **Output files**: App data directory (usually `~/.eternis/data/`)
- **Server logs**: Console output or configured log file

## Workflow Architecture

```
1. GraphQL Mutation (addDataSource)
   ↓
2. Database: Store data source record
   ↓  
3. GraphQL Mutation (startIndexingData)
   ↓
4. Temporal Workflow: InitializeWorkflow
   ↓
5. Activity: ProcessDataActivity
   ↓ 
6. Telegram Processor: Parse and convert to JSONL
   ↓
7. Activity: IndexBatchActivity  
   ↓
8. Memory Storage: Index documents in vector database
```

## Success Criteria

✅ **Processing Phase:**
- No "null" entries in output JSONL
- All eligible chats processed (not filtered out)
- Conversation extraction successful
- Multiple participants detected correctly

✅ **Indexing Phase:**
- Batches created successfully
- Documents stored in memory system
- No high failure rates
- Progress tracking works

✅ **End-to-End:**
- Data source marked as processed and indexed
- Vector database contains searchable conversation data
- No error states in database records 