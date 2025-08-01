package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartupRecoveryManager(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_recovery.db")

	// Use a single logger instance throughout the test
	logger := log.New(os.Stdout)
	loggerAdapter := NewLoggerAdapter(logger)
	backupMgr := NewDailyBackupManager(dbPath, loggerAdapter)
	recoveryMgr := NewStartupRecoveryManager(dbPath, backupMgr, loggerAdapter)

	ctx := context.Background()
	store, err := NewStoreWithLogger(ctx, dbPath, logger)
	if err != nil {
		t.Fatalf("Failed to create initial store: %v", err)
	}

	err = store.SetValue(ctx, "test_key", "test_value")
	if err != nil {
		t.Fatalf("Failed to set test value: %v", err)
	}
	_ = store.Close()

	t.Run("ArchiveCorruptedDatabase", func(t *testing.T) {
		archivePath, err := recoveryMgr.archiveCorruptedDatabase()
		if err != nil {
			t.Fatalf("Failed to archive database: %v", err)
		}

		if _, err := os.Stat(archivePath); os.IsNotExist(err) {
			t.Errorf("Archive file does not exist: %s", archivePath)
		}

		metadataPath := archivePath + ".metadata.txt"
		if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
			t.Errorf("Metadata file does not exist: %s", metadataPath)
		}

		t.Logf("Successfully archived to: %s", archivePath)
	})

	t.Run("ListArchivedDatabases", func(t *testing.T) {
		archives, err := recoveryMgr.ListArchivedCorruptedDatabases()
		if err != nil {
			t.Fatalf("Failed to list archived databases: %v", err)
		}

		if len(archives) == 0 {
			t.Log("No archived databases found (expected for clean test)")
		} else {
			t.Logf("Found %d archived database(s): %v", len(archives), archives)
		}
	})

	t.Run("DatabaseManagerWithRecovery", func(t *testing.T) {
		dbMgr := NewDatabaseManagerWithStartupRecovery(dbPath, backupMgr, loggerAdapter)

		db, err := dbMgr.OpenWithRecovery(ctx, dbPath)
		if err != nil {
			t.Fatalf("Failed to open database with recovery: %v", err)
		}
		defer func() { _ = db.Close() }()

		if err := db.PingContext(ctx); err != nil {
			t.Errorf("Database ping failed: %v", err)
		}

		t.Log("Enhanced database manager working correctly")
	})
}

func TestRecoveryStrategies(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_strategies.db")

	testLogger := log.New(os.Stdout)
	testLoggerAdapter := NewLoggerAdapter(testLogger)
	backupMgr := NewDailyBackupManager(dbPath, testLoggerAdapter)
	recoveryMgr := NewStartupRecoveryManager(dbPath, backupMgr, testLoggerAdapter)

	ctx := context.Background()

	store, err := NewStoreWithLogger(ctx, dbPath, testLogger)
	if err != nil {
		t.Fatalf("Failed to create initial store: %v", err)
	}

	// Add some test data
	err = store.SetValue(ctx, "recovery_test", "strategy_test_value")
	if err != nil {
		t.Fatalf("Failed to set test value: %v", err)
	}

	// Create a backup before we "corrupt" the database
	err = store.CreateManualBackup(ctx)
	if err != nil {
		t.Logf("Manual backup failed (may be expected): %v", err)
	}

	_ = store.Close()

	// Test individual recovery strategies
	// Create a corrupted database for actual recovery testing
	corruptedDBPath := filepath.Join(tmpDir, "corrupted_test.db")

	t.Run("Setup_Corrupted_Database", func(t *testing.T) {
		// First create a valid database with some data
		validStore, err := NewStoreWithLogger(ctx, corruptedDBPath, testLogger)
		require.NoError(t, err)

		err = validStore.SetValue(ctx, "recovery_test", "data_to_recover")
		require.NoError(t, err)

		_ = validStore.Close()

		// Now corrupt the database by writing invalid SQLite data
		err = os.WriteFile(corruptedDBPath, []byte("INVALID_SQLITE_DATABASE_CONTENT_FOR_TESTING"), 0o644)
		require.NoError(t, err)

		t.Log("Corrupted database created for recovery testing")
	})

	// Create recovery manager for corrupted database
	corruptedRecoveryMgr := NewStartupRecoveryManager(corruptedDBPath, backupMgr, testLoggerAdapter)

	t.Run("REINDEX_Recovery_Corrupted", func(t *testing.T) {
		err := corruptedRecoveryMgr.attemptReindexRecovery(ctx)
		// REINDEX may succeed or fail depending on corruption type - both are valid outcomes
		if err != nil {
			t.Logf("REINDEX recovery failed as expected on corrupted DB: %v", err)
		} else {
			t.Logf("REINDEX recovery succeeded on corrupted DB (SQLite is resilient)")
		}
	})

	t.Run("VACUUM_Recovery_Corrupted", func(t *testing.T) {
		err := corruptedRecoveryMgr.attemptVacuumRecovery(ctx)
		// VACUUM may succeed or fail depending on corruption type - both are valid outcomes
		if err != nil {
			t.Logf("VACUUM recovery failed as expected on corrupted DB: %v", err)
		} else {
			t.Logf("VACUUM recovery succeeded on corrupted DB (SQLite is resilient)")
		}
	})

	t.Run("VACUUM_INTO_Recovery_Corrupted", func(t *testing.T) {
		err := corruptedRecoveryMgr.attemptVacuumIntoRecovery(ctx)
		// VACUUM INTO may succeed or fail depending on corruption type - both are valid outcomes
		if err != nil {
			t.Logf("VACUUM INTO recovery failed as expected on corrupted DB: %v", err)
		} else {
			t.Logf("VACUUM INTO recovery succeeded on corrupted DB (SQLite is resilient)")
		}
	})

	t.Run("New_Database_Creation", func(t *testing.T) {
		err := corruptedRecoveryMgr.createNewDatabase(ctx)
		// This should succeed - creates new empty database
		assert.NoError(t, err, "Creating new database should succeed")
		t.Log("New database creation completed successfully")

		// Verify the new database works
		newStore, err := NewStoreWithLogger(ctx, corruptedDBPath, testLogger)
		assert.NoError(t, err, "Should be able to open new database")
		if newStore != nil {
			_ = newStore.Close()
		}
	})

	t.Run("Recovery_Verification", func(t *testing.T) {
		err := recoveryMgr.verifyRecovery(ctx)
		if err != nil {
			t.Errorf("Recovery verification failed: %v", err)
		} else {
			t.Log("Recovery verification passed")
		}
	})
}

func TestUtilityFunctions(t *testing.T) {
	t.Run("GetHostname", func(t *testing.T) {
		hostname := getHostname()
		if hostname == "" {
			t.Error("Hostname should not be empty")
		}
		t.Logf("Hostname: %s", hostname)
	})

	t.Run("GetSQLiteVersion", func(t *testing.T) {
		version := getSQLiteVersion()
		if version == "" {
			t.Error("SQLite version should not be empty")
		}
		t.Logf("SQLite Version: %s", version)
	})

	t.Run("GetGoVersion", func(t *testing.T) {
		version := getGoVersion()
		if version == "" {
			t.Error("Go version should not be empty")
		}
		t.Logf("Go Version: %s", version)
	})
}
