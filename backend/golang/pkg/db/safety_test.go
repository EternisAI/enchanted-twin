package db_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/log"

	"github.com/EternisAI/enchanted-twin/pkg/db"
)

func TestSQLiteSafetyFeatures(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test_safety_*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	_ = tmpFile.Close()

	logger := log.New(os.Stdout)
	store, err := db.NewStoreWithLogger(context.Background(), tmpFile.Name(), logger)
	if err != nil {
		t.Fatal("Failed to create store:", err)
	}
	defer func() { _ = store.Close() }()

	t.Run("ForeignKeysEnabled", func(t *testing.T) {
		var enabled int
		err := store.DB().QueryRow("PRAGMA foreign_keys").Scan(&enabled)
		if err != nil {
			t.Fatal("Failed to check foreign keys:", err)
		}
		if enabled != 1 {
			t.Error("Foreign keys are not enabled")
		}
	})

	t.Run("WALModeEnabled", func(t *testing.T) {
		var mode string
		err := store.DB().QueryRow("PRAGMA journal_mode").Scan(&mode)
		if err != nil {
			t.Fatal("Failed to check journal mode:", err)
		}
		if mode != "wal" {
			t.Error("WAL mode is not enabled, got:", mode)
		}
	})

	t.Run("BusyTimeoutSet", func(t *testing.T) {
		var timeout int
		err := store.DB().QueryRow("PRAGMA busy_timeout").Scan(&timeout)
		if err != nil {
			t.Fatal("Failed to check busy timeout:", err)
		}
		if timeout < 5000 {
			t.Error("Busy timeout is too low:", timeout)
		}
	})
}

func TestConcurrentAccess(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test_concurrent_*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	_ = tmpFile.Close()

	logger := log.New(os.Stdout)
	store, err := db.NewStoreWithLogger(context.Background(), tmpFile.Name(), logger)
	if err != nil {
		t.Fatal("Failed to create store:", err)
	}
	defer func() { _ = store.Close() }()

	_, err = store.DB().Exec(`
		CREATE TABLE test_concurrent (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			value TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatal("Failed to create test table:", err)
	}

	t.Run("ConcurrentWrites", func(t *testing.T) {
		const numWorkers = 10
		const insertsPerWorker = 100

		var wg sync.WaitGroup
		errChan := make(chan error, numWorkers)

		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				for j := 0; j < insertsPerWorker; j++ {
					value := fmt.Sprintf("worker_%d_insert_%d", workerID, j)
					_, err := store.DB().Exec("INSERT INTO test_concurrent (value) VALUES (?)", value)
					if err != nil {
						errChan <- fmt.Errorf("worker %d insert %d failed: %w", workerID, j, err)
						return
					}
				}
			}(i)
		}

		// Wait for all workers or timeout
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success
		case err := <-errChan:
			t.Fatal("Concurrent write failed:", err)
		case <-time.After(30 * time.Second):
			t.Fatal("Test timed out - possible deadlock")
		}

		// Verify all inserts succeeded
		var count int
		err := store.DB().QueryRow("SELECT COUNT(*) FROM test_concurrent").Scan(&count)
		if err != nil {
			t.Fatal("Failed to count records:", err)
		}

		expected := numWorkers * insertsPerWorker
		if count != expected {
			t.Errorf("Expected %d records, got %d", expected, count)
		}
	})
}
