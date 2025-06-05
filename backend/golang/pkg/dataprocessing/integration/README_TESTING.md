# Integration Testing

This directory contains integration tests for the memory system using Go's standard testing framework.

## Overview

The integration tests have been refactored from a single monolithic function to use Go's standard `testing` package, providing better organization, error reporting, and test isolation.

## Running Tests

### All tests (including integration)
```bash
make test
```

### Only integration tests
```bash
make test-integration
```

### Individual test functions
```bash
# Run only the main memory integration test
go test -v ./pkg/dataprocessing/integration/... -run TestMemoryIntegration

# Run only structured fact filtering tests
go test -v ./pkg/dataprocessing/integration/... -run TestStructuredFactFiltering

# Run specific sub-tests
go test -v ./pkg/dataprocessing/integration/... -run TestMemoryIntegration/BasicQuerying
```

### Legacy integration test (using cmd/memory-test)
```bash
make test-memory
```

## Environment Variables

The integration tests use the same environment variable loading mechanism as the main application (via `.env` file and `config.LoadConfig()`):

**API Keys (at least one method required):**
- **Option 1**: Use your existing `.env` file with `COMPLETIONS_API_KEY` and `EMBEDDINGS_API_KEY`
- **Option 2**: Set test-specific variables `TEST_COMPLETIONS_API_KEY` and `TEST_EMBEDDINGS_API_KEY`
- **Option 3**: Export environment variables directly

**Optional test overrides (with defaults):**
- `TEST_SOURCE` - Source type (defaults to "chatgpt")
- `TEST_INPUT_PATH` - Path to test data (defaults to "testdata/chatgpt.zip")
- `TEST_OUTPUT_PATH` - Path for processed output (defaults to "./output/{source}_{uuid}.jsonl")
- `TEST_COMPLETIONS_MODEL` - Model to use for completions (defaults to "gpt-4o-mini")
- `TEST_COMPLETIONS_API_URL` - URL for completions service (defaults to "https://openrouter.ai/api/v1")
- `TEST_EMBEDDINGS_MODEL` - Model to use for embeddings (defaults to "text-embedding-3-small")
- `TEST_EMBEDDINGS_API_URL` - URL for embeddings service (defaults to "https://api.openai.com/v1")

The tests will automatically use your existing `.env` file (the same one used by `make run` or `make test-memory`). If you want to override any settings specifically for tests, use the `TEST_*` prefixed variables.

## Test Structure

### Test Environment
The tests use a `testEnvironment` struct that handles:
- Weaviate server setup and teardown
- Database initialization
- Memory service configuration
- Document loading and storage

### Test Organization
Tests are organized into logical groups:

1. **TestMemoryIntegration** - Main integration test with sub-tests:
   - `DataProcessingAndStorage` - Tests document processing and storage
   - `BasicQuerying` - Tests basic memory queries
   - `DocumentReferences` - Tests document reference functionality
   - `SourceFiltering` - Tests filtering by source
   - `DistanceFiltering` - Tests distance-based filtering

2. **TestStructuredFactFiltering** - Tests for structured fact filtering:
   - Category filtering
   - Subject filtering  
   - Importance filtering (exact and range)
   - Sensitivity filtering
   - Combined filtering scenarios
   - Value partial matching
   - Temporal context filtering

### Benefits of the New Structure

1. **Better Test Isolation** - Each test function runs independently
2. **Granular Testing** - Run specific test scenarios
3. **Better Error Reporting** - Failed tests are clearly identified
4. **Standard Tooling** - Works with all Go testing tools
5. **Setup/Teardown** - Proper resource management per test
6. **Skip Logic** - Tests skip gracefully when requirements aren't met
7. **Parallel Execution** - Tests can run in parallel when appropriate

## Example Usage

```bash
# Method 1: Use existing .env file (no additional setup needed)
# Your .env file already has COMPLETIONS_API_KEY and EMBEDDINGS_API_KEY
make test-integration

# Method 2: Override API keys for testing
export TEST_COMPLETIONS_API_KEY="your-test-completions-key"
export TEST_EMBEDDINGS_API_KEY="your-test-embeddings-key"
make test-integration

# Method 3: Override specific test settings while using .env for API keys
export TEST_SOURCE="telegram"
export TEST_INPUT_PATH="path/to/telegram-data.zip"
make test-integration

# Method 4: Export environment variables directly (no .env file needed)
export COMPLETIONS_API_KEY="your-key"
export EMBEDDINGS_API_KEY="your-key"
make test-integration
```

## Test Data

Integration tests expect test data to be available at the path specified by `TEST_INPUT_PATH`. By default, this is `testdata/chatgpt.zip` (located in the same directory as the test files). The format should match the expected input format for the specified `TEST_SOURCE` type.

The `testdata/` directory contains sample data for different sources:
- `chatgpt.zip` - Sample ChatGPT export
- `slack_export_sample.zip` - Sample Slack export
- `x_export_sample.zip` - Sample X (Twitter) export
- `google_export_sample.zip` - Sample Google export
- `telegram_export_sample.json` - Sample Telegram export
- `misc.zip` - Miscellaneous sample data

## Default Values

The tests use the same defaults as `cmd/memory-test/main.go`:

- **Source**: "chatgpt"
- **Input Path**: "testdata/chatgpt.zip"
- **Output Path**: "./output/chatgpt_{uuid}.jsonl"
- **Completions Model**: "gpt-4o-mini"
- **Completions API URL**: "https://openrouter.ai/api/v1"
- **Embeddings Model**: "text-embedding-3-small"
- **Embeddings API URL**: "https://api.openai.com/v1" 