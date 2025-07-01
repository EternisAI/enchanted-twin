# Directory Watcher

The Directory Watcher automatically monitors a specified directory for new files and triggers data processing workflows when supported file types are detected.

## Features

- **Automatic File Detection**: Monitors a fixed directory for new files in real-time
- **File Type Recognition**: Automatically determines data source type based on filename patterns
- **Event Buffering**: Batches rapid file system events to prevent duplicate processing
- **Graceful Error Handling**: Continues monitoring even if individual files fail to process
- **Integration with Temporal**: Automatically triggers processing workflows for new data sources

## Supported File Types

The watcher supports the following data source file types:

| File Pattern | Data Source | Description |
|-------------|-------------|-------------|
| `*whatsapp*.db`, `*whatsapp*.sqlite` | WhatsApp | WhatsApp database files |
| `*telegram*.json` | Telegram | Telegram export JSON files |
| `*.mbox`, `*gmail*` | Gmail | Gmail MBOX files |
| `*slack*.zip` | Slack | Slack workspace exports |
| `*twitter*.zip`, `*x*.zip` | X (Twitter) | Twitter/X data archives |
| `*chatgpt*.json`, `*chatgpt*.zip` | ChatGPT | ChatGPT conversation exports |
| `*.json` | Telegram | Default JSON files (fallback) |
| `*.zip` | X | Default ZIP files (fallback) |

## Configuration

The directory watcher is configured via environment variables:

- `WATCH_DIRECTORY_PATH`: The directory to monitor (default: `./input_data`)

## How It Works

1. **Initialization**: The watcher is started automatically when the server starts
2. **Directory Monitoring**: Uses `fsnotify` to watch for file system events
3. **File Filtering**: Only processes supported file types, ignoring hidden files and temporary files
4. **Event Buffering**: Groups rapid events (5-second window) to handle file operations that generate multiple events
5. **Data Source Creation**: Automatically creates database entries for new files
6. **Workflow Triggering**: Starts the `InitializeWorkflow` to process new data sources

## Usage

### Automatic Operation

The directory watcher starts automatically with the server. Simply place supported files in the configured watch directory:

```bash
# Default directory
cp your_telegram_export.json ./input_data/
cp your_whatsapp.db ./input_data/
```

### Manual Configuration

Set a custom watch directory:

```bash
export WATCH_DIRECTORY_PATH="/path/to/your/data"
```

## File Processing Flow

```
New File Detected
    ↓
File Type Validation
    ↓
Data Source Type Detection
    ↓
Database Entry Creation
    ↓
Temporal Workflow Triggered
    ↓
File Processing & Indexing
```

## Error Handling

- **Invalid Files**: Unsupported file types are ignored
- **Processing Failures**: Individual file failures don't stop the watcher
- **Duplicate Files**: Files already in the database are skipped
- **Directory Issues**: Watcher creates the directory if it doesn't exist

## Performance Considerations

- **Event Buffering**: 5-second buffer prevents excessive processing of rapid file changes
- **Goroutine Safety**: Thread-safe file event handling
- **Resource Usage**: Minimal CPU usage when idle, efficient file system watching

## Troubleshooting

### Common Issues

1. **Files Not Detected**
   - Check file permissions
   - Verify file extensions match supported patterns
   - Ensure files aren't hidden (starting with `.`)

2. **Processing Failures**
   - Check server logs for error messages
   - Verify Temporal server is running
   - Ensure database connectivity

3. **Duplicate Processing**
   - The system automatically prevents duplicate processing
   - Check database for existing entries

### Logging

The watcher logs important events:

```
INFO  Directory watcher started watchDir=./input_data
INFO  Created data source for new file path=/path/to/file.json id=abc123 type=Telegram
INFO  Triggered processing workflow workflowID=auto-initialize-1234567890
```

## Testing

Run the test suite:

```bash
go test ./pkg/directorywatcher/...
```

The tests cover:
- File type detection
- Data source type determination  
- Event buffering functionality
- Basic initialization 