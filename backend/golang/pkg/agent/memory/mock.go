package memory

import "context"

type MockMemory struct{}

func (m *MockMemory) Store(ctx context.Context, documents []ConversationDocument, progressChan chan<- ProgressUpdate) error {
	return nil
}

func (m *MockMemory) StoreRawData(ctx context.Context, documents []TextDocument, progressCallback ProgressCallback) error {
	return nil
}

func (m *MockMemory) Query(ctx context.Context, query string) (QueryResult, error) {
	return QueryResult{}, nil
}
