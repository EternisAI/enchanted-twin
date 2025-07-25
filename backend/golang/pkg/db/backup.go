package db

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Logger interface for backup operations.
type Logger interface {
	Printf(format string, v ...interface{})
	Println(v ...interface{})
}

type DefaultLogger struct{}

func (dl *DefaultLogger) Printf(format string, v ...interface{}) {
	// Check if format string already ends with newline to avoid double newlines
	if strings.HasSuffix(format, "\n") {
		fmt.Printf("[DB-BACKUP] "+format, v...)
	} else {
		fmt.Printf("[DB-BACKUP] "+format+"\n", v...)
	}
}

func (dl *DefaultLogger) Println(v ...interface{}) {
	args := append([]interface{}{"[DB-BACKUP]"}, v...)
	fmt.Println(args...)
}

type BackupInfo struct {
	Path      string
	Timestamp time.Time
	Size      int64
	Valid     bool
}

type DailyBackupManager struct {
	dbPath   string
	db       *sql.DB
	logger   Logger
	stopChan chan struct{}
	started  bool
}

func NewDailyBackupManager(dbPath string, logger Logger) *DailyBackupManager {
	return &DailyBackupManager{
		dbPath:   dbPath,
		logger:   logger,
		stopChan: make(chan struct{}),
	}
}

func (dbm *DailyBackupManager) SetDatabase(db *sql.DB) {
	dbm.db = db
}

// CalculateNextBackup calculates the next backup time based on current time.
func (dbm *DailyBackupManager) CalculateNextBackup(now time.Time) time.Time {
	if now.Hour() < 2 {
		// If before 2 AM, schedule for today at 2 AM
		return time.Date(now.Year(), now.Month(), now.Day(), 2, 0, 0, 0, now.Location())
	} else {
		// If at or after 2 AM, schedule for tomorrow at 2 AM
		return time.Date(now.Year(), now.Month(), now.Day()+1, 2, 0, 0, 0, now.Location())
	}
}

func (dbm *DailyBackupManager) Start(ctx context.Context) {
	// Ensure database is set before starting
	if dbm.db == nil {
		dbm.logger.Printf("Cannot start backup manager: database not set")
		return
	}

	now := time.Now()
	nextBackup := dbm.CalculateNextBackup(now)

	// Do catch-up backup if we missed today's backup
	if now.Hour() >= 2 && !dbm.todaysBackupExists() {
		go func() {
			if err := dbm.CreateBackup(ctx); err != nil {
				dbm.logger.Printf("Initial backup failed: %v", err)
			}
		}()
	}

	dbm.started = true
	go dbm.runDailyBackups(ctx, nextBackup)
}

func (dbm *DailyBackupManager) Stop() {
	dbm.started = false
	close(dbm.stopChan)
}

// GetStatus returns the current status of the backup manager.
func (dbm *DailyBackupManager) GetStatus() string {
	if dbm.db == nil {
		return "Backup system not initialized - database not set"
	}
	if !dbm.started {
		return "Backup system not started"
	}

	// Check if today's backup exists
	if dbm.todaysBackupExists() {
		return "Daily backup system active - today's backup completed, next scheduled for 2AM"
	}

	now := time.Now()
	if now.Hour() >= 2 {
		return "Daily backup system active - today's backup pending, next scheduled for 2AM"
	}

	return "Daily backup system active - scheduled for 2AM today"
}

func (dbm *DailyBackupManager) runDailyBackups(ctx context.Context, nextBackup time.Time) {
	timer := time.NewTimer(time.Until(nextBackup))
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-dbm.stopChan:
			return
		case <-timer.C:
			if err := dbm.CreateBackup(ctx); err != nil {
				dbm.logger.Printf("Daily backup failed: %v", err)
			}

			timer.Reset(24 * time.Hour)
		}
	}
}

func (dbm *DailyBackupManager) CreateBackup(ctx context.Context) error {
	if dbm.db == nil {
		return fmt.Errorf("cannot create backup: database not set")
	}

	dbm.logger.Println("Starting daily backup...")

	if err := dbm.checkIntegrity(ctx); err != nil {
		return fmt.Errorf("integrity check failed, backup aborted: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	backupPath := fmt.Sprintf("%s.backup.%s", dbm.dbPath, timestamp)
	tempBackupPath := backupPath + ".tmp"

	if err := dbm.createBackupFile(ctx, tempBackupPath); err != nil {
		_ = os.Remove(tempBackupPath)
		return fmt.Errorf("backup creation failed: %w", err)
	}

	if err := dbm.verifyBackup(ctx, tempBackupPath); err != nil {
		_ = os.Remove(tempBackupPath)
		return fmt.Errorf("backup verification failed: %w", err)
	}

	if err := os.Rename(tempBackupPath, backupPath); err != nil {
		_ = os.Remove(tempBackupPath)
		return fmt.Errorf("backup file rename failed: %w", err)
	}

	if stat, err := os.Stat(backupPath); err == nil {
		sizeMB := float64(stat.Size()) / (1024 * 1024)
		dbm.logger.Printf("Daily backup completed successfully (%.2f MB)", sizeMB)
	} else {
		dbm.logger.Println("Daily backup completed successfully")
	}

	return nil
}

func (dbm *DailyBackupManager) checkIntegrity(ctx context.Context) error {
	row := dbm.db.QueryRowContext(ctx, "PRAGMA integrity_check")
	var result string
	if err := row.Scan(&result); err != nil {
		return fmt.Errorf("integrity check query failed: %w", err)
	}

	if result != "ok" {
		return fmt.Errorf("database corruption detected: %s", result)
	}

	dbm.logger.Println("Database integrity check passed")
	return nil
}

// escapeSQLitePath safely escapes a file path for use in SQLite queries.
func escapeSQLitePath(path string) string {
	// Escape single quotes by doubling them, as per SQLite standard
	return strings.ReplaceAll(path, "'", "''")
}

func (dbm *DailyBackupManager) createBackupFile(ctx context.Context, backupPath string) error {
	escapedPath := escapeSQLitePath(backupPath)
	query := fmt.Sprintf("VACUUM INTO '%s'", escapedPath)
	_, err := dbm.db.ExecContext(ctx, query)
	return err
}

func (dbm *DailyBackupManager) verifyBackup(ctx context.Context, backupPath string) error {
	backupDB, err := sql.Open("sqlite3_safe", backupPath+"?mode=ro")
	if err != nil {
		return err
	}
	defer func() { _ = backupDB.Close() }()

	row := backupDB.QueryRowContext(ctx, "PRAGMA quick_check")
	var result string
	if err := row.Scan(&result); err != nil {
		return err
	}

	if result != "ok" {
		return fmt.Errorf("backup verification failed: %s", result)
	}

	dbm.logger.Println("Backup verification passed")
	return nil
}

func (dbm *DailyBackupManager) backupExists() bool {
	pattern := dbm.dbPath + ".backup.*"
	matches, err := filepath.Glob(pattern)
	return err == nil && len(matches) > 0
}

// todaysBackupExists checks if a backup was created today (within last 24 hours).
func (dbm *DailyBackupManager) todaysBackupExists() bool {
	pattern := dbm.dbPath + ".backup.*"
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return false
	}

	now := time.Now()
	twentyFourHoursAgo := now.Add(-24 * time.Hour)

	// Check if any backup file was created within the last 24 hours
	for _, backupPath := range matches {
		stat, err := os.Stat(backupPath)
		if err != nil {
			continue
		}

		// If backup was created within last 24 hours, consider it today's backup
		if stat.ModTime().After(twentyFourHoursAgo) {
			return true
		}
	}

	return false
}

func (dbm *DailyBackupManager) GetBackupInfo() (*BackupInfo, error) {
	pattern := dbm.dbPath + ".backup.*"
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to search for backup files: %w", err)
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no backup exists")
	}

	sort.Strings(matches)
	latestBackup := matches[len(matches)-1]

	stat, err := os.Stat(latestBackup)
	if err != nil {
		return nil, err
	}

	return &BackupInfo{
		Path:      latestBackup,
		Timestamp: stat.ModTime(),
		Size:      stat.Size(),
		Valid:     true,
	}, nil
}

type StartupRecoveryManager struct {
	dbPath        string
	backupManager *DailyBackupManager
	logger        Logger
}

func NewStartupRecoveryManager(dbPath string, backupMgr *DailyBackupManager, logger Logger) *StartupRecoveryManager {
	return &StartupRecoveryManager{
		dbPath:        dbPath,
		backupManager: backupMgr,
		logger:        logger,
	}
}

func (srm *StartupRecoveryManager) RecoverCorruptedDatabase(ctx context.Context) error {
	srm.logger.Println("Starting comprehensive database recovery...")

	archivePath, err := srm.archiveCorruptedDatabase()
	if err != nil {
		srm.logger.Printf("Warning: Failed to archive corrupted database: %v", err)
	} else {
		srm.logger.Printf("Corrupted database archived to: %s", archivePath)
	}

	recoveryStrategies := []struct {
		name        string
		description string
		fn          func(context.Context) error
	}{
		{
			name:        "REINDEX Recovery",
			description: "Fix index corruption using REINDEX",
			fn:          srm.attemptReindexRecovery,
		},
		{
			name:        "VACUUM Recovery",
			description: "Rebuild database using VACUUM",
			fn:          srm.attemptVacuumRecovery,
		},
		{
			name:        "VACUUM INTO Recovery",
			description: "Create clean copy using VACUUM INTO",
			fn:          srm.attemptVacuumIntoRecovery,
		},
		{
			name:        "SQLite DUMP Recovery",
			description: "Export and reimport using .dump",
			fn:          srm.attemptDumpRecovery,
		},
		{
			name:        "SQLite RECOVER Recovery",
			description: "Data salvage using .recover",
			fn:          srm.attemptRecoverCommand,
		},
		{
			name:        "Backup Restoration",
			description: "Restore from daily backup",
			fn:          srm.restoreFromBackup,
		},
		{
			name:        "New Database Creation",
			description: "Create new empty database (last resort)",
			fn:          srm.createNewDatabase,
		},
	}

	for _, strategy := range recoveryStrategies {
		srm.logger.Printf("Attempting recovery strategy: %s - %s", strategy.name, strategy.description)

		if err := strategy.fn(ctx); err != nil {
			srm.logger.Printf("Strategy '%s' failed: %v", strategy.name, err)
			continue
		}

		// Verify recovery was successful
		if err := srm.verifyRecovery(ctx); err != nil {
			srm.logger.Printf("Recovery verification failed after '%s': %v", strategy.name, err)
			continue
		}

		srm.logger.Printf("Recovery successful using strategy: %s", strategy.name)
		return nil
	}

	return fmt.Errorf("all recovery strategies failed")
}

func (srm *StartupRecoveryManager) attemptReindexRecovery(ctx context.Context) error {
	srm.logger.Println("Attempting REINDEX recovery...")

	db, err := sql.Open("sqlite3", srm.dbPath+"?_busy_timeout=30000")
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }() // Try REINDEX to fix index corruption
	if _, err := db.ExecContext(ctx, "REINDEX"); err != nil {
		return err
	}

	return nil
}

func (srm *StartupRecoveryManager) attemptVacuumRecovery(ctx context.Context) error {
	srm.logger.Println("Attempting VACUUM recovery...")

	db, err := sql.Open("sqlite3", srm.dbPath+"?_busy_timeout=30000")
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	if _, err := db.ExecContext(ctx, "VACUUM"); err != nil {
		return err
	}

	return nil
}

func (srm *StartupRecoveryManager) attemptVacuumIntoRecovery(ctx context.Context) error {
	srm.logger.Println("Attempting VACUUM INTO recovery...")

	tempPath := srm.dbPath + ".vacuum_recovery"

	db, err := sql.Open("sqlite3", srm.dbPath+"?_busy_timeout=30000")
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	escapedTempPath := escapeSQLitePath(tempPath)
	query := fmt.Sprintf("VACUUM INTO '%s'", escapedTempPath)
	if _, err := db.ExecContext(ctx, query); err != nil {
		_ = os.Remove(tempPath)
		return err
	}

	if err := srm.verifyDatabase(ctx, tempPath); err != nil {
		_ = os.Remove(tempPath)
		return err
	}

	if err := srm.replaceDatabase(tempPath); err != nil {
		_ = os.Remove(tempPath)
		return err
	}

	return nil
}

func (srm *StartupRecoveryManager) attemptDumpRecovery(ctx context.Context) error {
	srm.logger.Println("Attempting .dump recovery...")

	dumpPath := srm.dbPath + ".dump.sql"
	recoveredPath := srm.dbPath + ".dump_recovery"

	defer func() {
		_ = os.Remove(dumpPath)
		if err := recover(); err != nil {
			_ = os.Remove(recoveredPath)
			panic(err)
		}
	}()

	if err := srm.executeSQLiteDump(dumpPath); err != nil {
		return err
	}

	if err := srm.createDatabaseFromDump(ctx, dumpPath, recoveredPath); err != nil {
		_ = os.Remove(recoveredPath)
		return err
	}

	if err := srm.verifyDatabase(ctx, recoveredPath); err != nil {
		_ = os.Remove(recoveredPath)
		return err
	}

	return srm.replaceDatabase(recoveredPath)
}

func (srm *StartupRecoveryManager) attemptRecoverCommand(ctx context.Context) error {
	srm.logger.Println("Attempting .recover command recovery...")

	recoverPath := srm.dbPath + ".recover.sql"
	recoveredPath := srm.dbPath + ".recover_recovery"

	defer func() {
		_ = os.Remove(recoverPath)
		if err := recover(); err != nil {
			_ = os.Remove(recoveredPath)
			panic(err)
		}
	}()

	// Execute SQLite .recover command
	if err := srm.executeSQLiteRecover(recoverPath); err != nil {
		return err
	}

	if err := srm.createDatabaseFromDump(ctx, recoverPath, recoveredPath); err != nil {
		_ = os.Remove(recoveredPath)
		return err
	}

	if err := srm.verifyDatabase(ctx, recoveredPath); err != nil {
		_ = os.Remove(recoveredPath)
		return err
	}

	return srm.replaceDatabase(recoveredPath)
}

func (srm *StartupRecoveryManager) restoreFromBackup(ctx context.Context) error {
	srm.logger.Println("Attempting backup restoration...")

	backupInfo, err := srm.backupManager.GetBackupInfo()
	if err != nil {
		return fmt.Errorf("no backup available: %w", err)
	}

	srm.logger.Printf("Restoring database from backup created: %s", backupInfo.Timestamp.Format("2006-01-02 15:04:05"))

	if err := srm.safeCopy(backupInfo.Path, srm.dbPath); err != nil {
		return fmt.Errorf("failed to restore from backup: %w", err)
	}

	return nil
}

func (srm *StartupRecoveryManager) createNewDatabase(ctx context.Context) error {
	srm.logger.Println("Creating new empty database as last resort...")

	// Remove corrupted database
	if err := os.Remove(srm.dbPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove corrupted database: %w", err)
	}

	db, err := sql.Open("sqlite3_safe", srm.dbPath)
	if err != nil {
		return fmt.Errorf("failed to create new database: %w", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("new database ping failed: %w", err)
	}

	srm.logger.Println("New empty database created successfully")
	return nil
}

func (srm *StartupRecoveryManager) executeSQLiteDump(dumpPath string) error {
	cmd := exec.Command("sqlite3", srm.dbPath, ".dump")

	outFile, err := os.Create(dumpPath)
	if err != nil {
		return err
	}
	defer func() { _ = outFile.Close() }()

	cmd.Stdout = outFile
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sqlite3 .dump failed: %w", err)
	}

	return nil
}

func (srm *StartupRecoveryManager) executeSQLiteRecover(recoverPath string) error {
	cmd := exec.Command("sqlite3", srm.dbPath, ".recover")

	outFile, err := os.Create(recoverPath)
	if err != nil {
		return err
	}
	defer func() { _ = outFile.Close() }()

	cmd.Stdout = outFile
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sqlite3 .recover failed: %w", err)
	}

	return nil
}

// Create database from SQL dump using sqlite3 command-line tool.
func (srm *StartupRecoveryManager) createDatabaseFromDump(ctx context.Context, dumpPath, newDbPath string) error {
	// Remove target database if it exists
	_ = os.Remove(newDbPath)

	// Use sqlite3 command-line tool to pipe the dump directly
	cmd := exec.CommandContext(ctx, "sqlite3", newDbPath)
	cmd.Stderr = os.Stderr

	// Open the dump file for reading
	dumpFile, err := os.Open(dumpPath)
	if err != nil {
		return fmt.Errorf("failed to open dump file: %w", err)
	}
	defer func() { _ = dumpFile.Close() }()

	// Pipe the dump file directly to sqlite3 stdin
	cmd.Stdin = dumpFile

	if err := cmd.Run(); err != nil {
		_ = os.Remove(newDbPath) // Clean up partial database
		return fmt.Errorf("failed to restore from SQL dump: %w", err)
	}

	return nil
}

// Verify database integrity and functionality.
func (srm *StartupRecoveryManager) verifyDatabase(ctx context.Context, dbPath string) error {
	db, err := sql.Open("sqlite3", dbPath+"?mode=ro")
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	// Quick integrity check
	row := db.QueryRowContext(ctx, "PRAGMA quick_check")
	var result string
	if err := row.Scan(&result); err != nil {
		return err
	}

	if result != "ok" {
		return fmt.Errorf("database verification failed: %s", result)
	}

	return nil
}

// Replace original database with recovered one.
func (srm *StartupRecoveryManager) replaceDatabase(recoveredPath string) error {
	// Remove original database
	if err := os.Remove(srm.dbPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	// Move recovered database to original location
	return os.Rename(recoveredPath, srm.dbPath)
}

// Enhanced verification that checks both integrity and basic functionality.
func (srm *StartupRecoveryManager) verifyRecovery(ctx context.Context) error {
	// Basic file verification
	if err := srm.verifyDatabase(ctx, srm.dbPath); err != nil {
		return err
	}

	// Test basic operations
	db, err := sql.Open("sqlite3", srm.dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	// Test that we can perform basic operations
	if err := db.PingContext(ctx); err != nil {
		return err
	}

	// Test schema access
	rows, err := db.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' LIMIT 1")
	if err != nil {
		return err
	}
	_ = rows.Close()

	srm.logger.Println("Recovery verification successful")
	return nil
}

// Safe copy method for recovery manager.
func (srm *StartupRecoveryManager) safeCopy(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = sourceFile.Close() }()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = destFile.Close() }()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		_ = os.Remove(dst) // Clean up partial file
		return err
	}

	return destFile.Sync() // Ensure data is written to disk
}
