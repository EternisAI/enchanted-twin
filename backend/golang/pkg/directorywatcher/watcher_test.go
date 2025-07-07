package directorywatcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/mocks"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory/storage"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

// mockMemoryStorage is a simple mock implementation
type mockMemoryStorage struct{}

func (m *mockMemoryStorage) Store(ctx context.Context, documents []memory.Document, progressCallback memory.ProgressCallback) error {
	return nil
}

func (m *mockMemoryStorage) Query(ctx context.Context, queryText string, filter *memory.Filter) (memory.QueryResult, error) {
	return memory.QueryResult{}, nil
}

func (m *mockMemoryStorage) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockMemoryStorage) GetDocumentReferences(ctx context.Context, memoryID string) ([]*storage.DocumentReference, error) {
	return nil, nil
}

func (m *mockMemoryStorage) StoreFactsDirectly(ctx context.Context, facts []*memory.MemoryFact, progressCallback memory.ProgressCallback) error {
	return nil
}

func (m *mockMemoryStorage) GetFactsByIDs(ctx context.Context, factIDs []string) ([]*memory.MemoryFact, error) {
	return nil, nil
}

func (m *mockMemoryStorage) IntelligentQuery(ctx context.Context, queryText string, filter *memory.Filter) (*memory.IntelligentQueryResult, error) {
	return nil, nil
}

func (m *mockMemoryStorage) RunConsolidation(ctx context.Context) error {
	return nil
}

func TestRecursiveDirectoryWatching(t *testing.T) {
	// Create temporary directory structure for testing
	tempDir := t.TempDir()
	subDir1 := filepath.Join(tempDir, "subdir1")
	subDir2 := filepath.Join(tempDir, "subdir2")
	deepDir := filepath.Join(subDir1, "deep")

	require.NoError(t, os.MkdirAll(subDir1, 0o755))
	require.NoError(t, os.MkdirAll(subDir2, 0o755))
	require.NoError(t, os.MkdirAll(deepDir, 0o755))

	// Create mock dependencies
	mockStore := &db.Store{}
	mockMemoryStorage := &mockMemoryStorage{}
	logger := log.New(os.Stdout)
	mockTemporalClient := &mocks.Client{}

	// Create directory watcher
	dw, err := NewDirectoryWatcher(mockStore, mockMemoryStorage, logger, mockTemporalClient)
	require.NoError(t, err)
	defer dw.Stop()

	// Test addDirectoryRecursively
	err = dw.addDirectoryRecursively(tempDir)
	require.NoError(t, err)

	// Verify all directories are being watched
	watchList := dw.watcher.WatchList()
	expectedDirs := []string{tempDir, subDir1, subDir2, deepDir}

	assert.Len(t, watchList, len(expectedDirs))

	for _, expectedDir := range expectedDirs {
		found := false
		for _, watchedDir := range watchList {
			if watchedDir == expectedDir {
				found = true
				break
			}
		}
		assert.True(t, found, "Directory %s should be watched", expectedDir)
	}
}

func TestDirectoryCreationDetection(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()

	// Create mock dependencies
	mockStore := &db.Store{}
	mockMemoryStorage := &mockMemoryStorage{}
	logger := log.New(os.Stdout)
	mockTemporalClient := &mocks.Client{}

	// Create directory watcher
	dw, err := NewDirectoryWatcher(mockStore, mockMemoryStorage, logger, mockTemporalClient)
	require.NoError(t, err)
	defer dw.Stop()

	// Add initial directory
	err = dw.addDirectoryRecursively(tempDir)
	require.NoError(t, err)

	initialWatchCount := len(dw.watcher.WatchList())

	// Start the watcher
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go func() {
		err := dw.Start(ctx)
		if err != nil {
			t.Logf("Watcher start error: %v", err)
		}
	}()

	// Give the watcher time to start
	time.Sleep(100 * time.Millisecond)

	// Create a new subdirectory
	newSubDir := filepath.Join(tempDir, "newsubdir")
	require.NoError(t, os.MkdirAll(newSubDir, 0o755))

	// Give the watcher time to detect the new directory
	time.Sleep(200 * time.Millisecond)

	// Verify the new directory was added to the watch list
	finalWatchCount := len(dw.watcher.WatchList())
	assert.Greater(t, finalWatchCount, initialWatchCount, "New directory should be added to watch list")
}
