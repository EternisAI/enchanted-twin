# Telegram Package Changelog

## [Enhanced] - 2024-12-30

### Added
- **Username Extraction**: Automatic extraction of usernames from `personal_information.username`
- **Enhanced Database Integration**: New `ProcessFileWithStore()` method with database support
- **Comprehensive User Metadata**: Extraction of user ID, names, phone number, and bio
- **Smart Message Attribution**: Automatic identification of user's own messages using extracted username
- **Comprehensive Test Suite**: Full test coverage including username extraction, fallback scenarios, and examples

### Enhanced
- **Message Processing**: Now uses extracted username for accurate message attribution
- **Error Handling**: Graceful fallback when username extraction fails
- **Documentation**: Complete package documentation with usage examples

### Technical Details
- Added `ProcessFileWithStore(ctx, store, filepath, userName)` method
- Enhanced `TelegramData` struct with `PersonalInformation` field
- Integrated with `source_usernames` database table
- Backward compatible with existing `ProcessFile()` method

### Files
- `telegram.go` - Core processing logic with username extraction
- `telegram_username_test.go` - Comprehensive test suite
- `README.md` - Complete package documentation

### Usage
```go
// New enhanced usage with automatic username extraction
source := telegram.New()
records, err := source.ProcessFileWithStore(ctx, store, "export.json", "")

// Legacy usage (still supported)
records, err := source.ProcessFile("export.json", "username")
```

### Database Schema
The package now integrates with the `source_usernames` table:
- Stores extracted usernames centrally
- Enables reuse across processing sessions
- Supports rich user metadata storage

### Testing
- ✅ Username extraction accuracy
- ✅ Message attribution correctness  
- ✅ Database integration
- ✅ Fallback behavior
- ✅ Example usage patterns

Run tests with: `go test ./pkg/dataprocessing/telegram` 