package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/log"
)

// LoggerAdapter adapts charmbracelet/log.Logger to our Logger interface.
type LoggerAdapter struct {
	logger *log.Logger
}

func NewLoggerAdapter(logger *log.Logger) Logger {
	return &LoggerAdapter{logger: logger}
}

func (l *LoggerAdapter) Printf(format string, v ...interface{}) {
	l.logger.Infof(format, v...)
}

func (l *LoggerAdapter) Println(v ...interface{}) {
	l.logger.Info(fmt.Sprint(v...))
}

func (srm *StartupRecoveryManager) archiveCorruptedDatabase() (string, error) {
	timestamp := time.Now().Format("20060102_150405")
	hostname := getHostname()

	archiveName := fmt.Sprintf("%s.corrupted.%s.%s.%d.db",
		strings.TrimSuffix(filepath.Base(srm.dbPath), ".db"),
		timestamp,
		hostname,
		os.Getpid(),
	)

	archiveDir := filepath.Join(filepath.Dir(srm.dbPath), "corrupted_archives")
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create archive directory: %w", err)
	}

	archivePath := filepath.Join(archiveDir, archiveName)

	if err := srm.safeCopy(srm.dbPath, archivePath); err != nil {
		return "", fmt.Errorf("failed to archive corrupted database: %w", err)
	}

	srm.archiveAuxiliaryFiles(archivePath, timestamp, hostname)

	// Create archive metadata file
	srm.createArchiveMetadata(archivePath, timestamp)

	return archivePath, nil
}

func (srm *StartupRecoveryManager) archiveAuxiliaryFiles(archivePath, timestamp, hostname string) {
	// Archive WAL file if exists
	walPath := srm.dbPath + "-wal"
	if _, err := os.Stat(walPath); err == nil {
		walArchivePath := strings.Replace(archivePath, ".db", "-wal.db", 1)
		_ = srm.safeCopy(walPath, walArchivePath)
	}

	// Archive SHM file if exists
	shmPath := srm.dbPath + "-shm"
	if _, err := os.Stat(shmPath); err == nil {
		shmArchivePath := strings.Replace(archivePath, ".db", "-shm.db", 1)
		_ = srm.safeCopy(shmPath, shmArchivePath)
	}
}

func (srm *StartupRecoveryManager) createArchiveMetadata(archivePath, timestamp string) {
	metadataPath := archivePath + ".metadata.txt"

	metadata := fmt.Sprintf(`SQLite Corruption Archive Metadata
Created: %s
Original Path: %s
Archive Path: %s
Host: %s
Process ID: %d
SQLite Version: %s
Recovery Attempt: Startup
Go Version: %s
`,
		timestamp,
		srm.dbPath,
		archivePath,
		getHostname(),
		os.Getpid(),
		getSQLiteVersion(),
		getGoVersion(),
	)

	_ = os.WriteFile(metadataPath, []byte(metadata), 0o644)
}

func getHostname() string {
	if hostname, err := os.Hostname(); err == nil {
		return hostname
	}
	return "unknown"
}

func getSQLiteVersion() string {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return "unknown"
	}
	defer func() { _ = db.Close() }()

	var version string
	if err := db.QueryRow("SELECT sqlite_version()").Scan(&version); err != nil {
		return "unknown"
	}
	return version
}

func getGoVersion() string {
	return runtime.Version()
}

func (srm *StartupRecoveryManager) ListArchivedCorruptedDatabases() ([]string, error) {
	archiveDir := filepath.Join(filepath.Dir(srm.dbPath), "corrupted_archives")

	if _, err := os.Stat(archiveDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	pattern := filepath.Join(archiveDir, "*.corrupted.*.db")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to list archived databases: %w", err)
	}

	sort.Slice(matches, func(i, j int) bool {
		statI, errI := os.Stat(matches[i])
		statJ, errJ := os.Stat(matches[j])
		if errI != nil || errJ != nil {
			return false
		}
		return statI.ModTime().After(statJ.ModTime())
	})

	return matches, nil
}

func (srm *StartupRecoveryManager) RestoreFromBackup(ctx context.Context) error {
	backupInfo, err := srm.backupManager.GetBackupInfo()
	if err != nil {
		return fmt.Errorf("no backup available: %w", err)
	}

	srm.logger.Printf("Starting direct backup restoration from backup created: %s", backupInfo.Timestamp.Format("2006-01-02 15:04:05"))

	// Archive the current corrupted database before restoration
	archivePath, err := srm.archiveCorruptedDatabase()
	if err != nil {
		srm.logger.Printf("Warning: Failed to archive corrupted database: %v", err)
	} else {
		srm.logger.Printf("Corrupted database archived to: %s", archivePath)
	}

	// Directly restore from backup
	srm.logger.Printf("Copying backup file %s to %s", backupInfo.Path, srm.dbPath)
	if err := srm.safeCopy(backupInfo.Path, srm.dbPath); err != nil {
		return fmt.Errorf("failed to restore from backup: %w", err)
	}

	// Verify the restored database
	if err := srm.verifyDatabase(ctx, srm.dbPath); err != nil {
		return fmt.Errorf("backup restoration verification failed: %w", err)
	}

	srm.logger.Printf("Direct backup restoration completed successfully")
	return nil
}

// Database Manager with Enhanced Recovery Capabilities.
type DatabaseManagerWithStartupRecovery struct {
	startupRecovery *StartupRecoveryManager
	logger          Logger
}

func NewDatabaseManagerWithStartupRecovery(dbPath string, backupMgr *DailyBackupManager, logger Logger) *DatabaseManagerWithStartupRecovery {
	return &DatabaseManagerWithStartupRecovery{
		startupRecovery: NewStartupRecoveryManager(dbPath, backupMgr, logger),
		logger:          logger,
	}
}

// Enhanced open with corruption detection and automatic recovery.
func (dm *DatabaseManagerWithStartupRecovery) OpenWithRecovery(ctx context.Context, dbPath string) (*sql.DB, error) {
	// First, try to open the database normally
	db, err := sql.Open("sqlite3_safe", dbPath)
	if err != nil {
		dm.logger.Printf("Initial database open failed: %v", err)

		// Attempt comprehensive recovery
		if recoveryErr := dm.startupRecovery.RecoverCorruptedDatabase(ctx); recoveryErr != nil {
			// Recovery failed - provide user with detailed error
			archives, _ := dm.startupRecovery.ListArchivedCorruptedDatabases()

			errorMsg := fmt.Sprintf(`Database recovery failed: %v

The corrupted database has been archived for potential manual recovery.
Recent corrupted archives: %d

Options:
1. Try manual recovery from archived database
2. Restore from backup (if available) 
3. Start with new empty database

Please contact support if this problem persists.`, recoveryErr, len(archives))

			return nil, fmt.Errorf("%s", errorMsg)
		}

		dm.logger.Println("Database recovered successfully, attempting to open again...")

		// Try to open again after recovery
		db, err = sql.Open("sqlite3_safe", dbPath)
		if err != nil {
			return nil, fmt.Errorf("database still failed to open after recovery: %w", err)
		}
	}

	// Perform basic integrity check on opened database
	row := db.QueryRowContext(ctx, "PRAGMA quick_check")
	var result string
	if err := row.Scan(&result); err != nil {
		dm.logger.Printf("Integrity check failed: %v", err)
		_ = db.Close()

		// Attempt recovery and reopen
		return dm.attemptRecoveryAndReopen(ctx, dbPath, "integrity check query failed")
	}

	if result != "ok" {
		dm.logger.Printf("Database integrity check returned: %s", result)
		_ = db.Close()

		// Attempt recovery and reopen
		return dm.attemptRecoveryAndReopen(ctx, dbPath, fmt.Sprintf("integrity check failed: %s", result))
	}

	dm.logger.Println("Database opened successfully with integrity verified")
	return db, nil
}

// attemptRecoveryAndReopen performs recovery and attempts to reopen the database.
func (dm *DatabaseManagerWithStartupRecovery) attemptRecoveryAndReopen(ctx context.Context, dbPath string, reason string) (*sql.DB, error) {
	dm.logger.Printf("Attempting recovery due to: %s", reason)

	// Attempt recovery
	if recoveryErr := dm.startupRecovery.RecoverCorruptedDatabase(ctx); recoveryErr != nil {
		return nil, fmt.Errorf("database recovery failed (%s): %w", reason, recoveryErr)
	}

	// Try to open again after recovery
	db, err := sql.Open("sqlite3_safe", dbPath)
	if err != nil {
		return nil, fmt.Errorf("database failed to open after recovery (%s): %w", reason, err)
	}

	dm.logger.Printf("Database recovery completed successfully for: %s", reason)
	return db, nil
}
