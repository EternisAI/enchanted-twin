# Testing Strategy for Memory System Refactoring

## Current State

### ✅ Phase 1 Complete: Pure Functions & Types

We've successfully implemented the pure functional core:

1. **Types Added to `evolvingmemory.go`** (following PLAN.md)
   - Configuration types (Config)
   - Processing pipeline types (PreparedDocument, ExtractedFact, MemoryDecision, etc.)
   - IO boundary interfaces (FactExtractor, MemoryOperations, StorageOperations)
   - Reused existing tool name constants for MemoryAction

2. **Pure Functions in `pure.go`**
   - `PrepareDocuments()` - Converts raw documents to prepared documents
   - `DistributeWork()` - Splits work among workers
   - `ValidateMemoryOperation()` - Enforces speaker validation rules
   - `CreateMemoryObject()` - Builds Weaviate objects for ADD operations
   - `BatchObjects()` - Groups objects into batches
   - Helper functions for metadata marshaling

3. **Tests Created** (`pure_test.go`)
   - All pure functions have comprehensive test coverage
   - Validation rules are thoroughly tested with table-driven tests
   - All tests pass ✅

### Original Baseline Tests

1. **Simple Unit Tests** (`evolvingmemory_test.go`)
   - Tests basic Document interface implementations
   - Verifies helper functions work correctly
   - Documents the speaker validation rules (to be implemented)
   - Confirms constants are properly defined

2. **Mock Infrastructure** (`mocks_test.go`)
   - Created mocks for AI services (ready for future use)
   - Currently not used due to the concrete `*ai.Service` dependency

## Running Tests

```bash
# Run just the memory tests
cd backend/golang
go test ./pkg/agent/memory/evolvingmemory -v

# Run all tests (includes memory tests)
make test

# Run the integration test with real services
make test-memory
```

## Next Steps for Testing

### Phase 2: IO Boundaries & Interface Adapters
Next, we need to:
1. Create `crud.go` - Move database operations from evolvingmemory.go
2. Implement interface adapters for fact extraction and memory operations
3. Test the adapters with mocks

### Phase 3: Pipeline Implementation
Then build the worker pipeline:
1. Create `pipeline.go` with StoreV2() method
2. Implement worker functions (extractFactsWorker, processFactsWorker, etc.)
3. Test channel communication and error propagation
4. Test graceful shutdown with context cancellation

### Phase 4: Parity Testing
Finally, ensure backward compatibility:
1. Run same test scenarios through both `Store()` and `StoreV2()`
2. Compare outputs to ensure identical behavior
3. Benchmark performance improvements

## Integration Testing

The existing `make test-memory` command runs a full integration test with:
- Real Weaviate instance
- Real OpenAI/embeddings services
- Actual Telegram data processing

This should continue to work after refactoring and serve as the final validation.

## Test Data

For comprehensive testing, consider creating test fixtures with:
- Various conversation formats (single speaker, multi-speaker)
- Edge cases (empty conversations, very long facts)
- Speaker validation scenarios
- Memory update/delete scenarios

## Notes

- Keep tests simple and focused on behavior, not implementation
- Use table-driven tests for validation rules
- Consider property-based testing for the pure functions
- Integration tests should use Docker for Weaviate consistency 