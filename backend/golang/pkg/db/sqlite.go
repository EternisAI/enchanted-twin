// Owner: johan@eternis.ai
package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/mattn/go-sqlite3"
)

// Register a custom SQLite driver with connection hooks for safety.
func init() {
	sql.Register("sqlite3_safe", &sqlite3.SQLiteDriver{
		ConnectHook: func(conn *sqlite3.SQLiteConn) error {
			var errs []error

			// Execute all PRAGMA commands, collecting any errors
			if _, err := conn.Exec("PRAGMA foreign_keys = ON", nil); err != nil {
				errs = append(errs, fmt.Errorf("failed to enable foreign keys: %w", err))
			}

			if _, err := conn.Exec("PRAGMA busy_timeout = 5000", nil); err != nil {
				errs = append(errs, fmt.Errorf("failed to set busy timeout: %w", err))
			}

			if _, err := conn.Exec("PRAGMA journal_mode = WAL", nil); err != nil {
				errs = append(errs, fmt.Errorf("failed to set WAL mode: %w", err))
			}

			// Return combined error if any occurred
			if len(errs) > 0 {
				var errStrings []string
				for _, err := range errs {
					errStrings = append(errStrings, err.Error())
				}
				return fmt.Errorf("PRAGMA errors: %s", strings.Join(errStrings, "; "))
			}

			return nil
		},
	})
}

// Wrapper around a SQLite database connection that provides
// functionality specific to Twins with enhanced recovery system.
type Store struct {
	db          *sqlx.DB
	backupMgr   *DailyBackupManager
	recoveryMgr *StartupRecoveryManager
	logger      *log.Logger
}

// NewStoreWithLogger creates a new SQLite-backed store with the provided logger.
func NewStoreWithLogger(ctx context.Context, dbPath string, logger *log.Logger) (*Store, error) {
	dir := filepath.Dir(dbPath)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
	}

	loggerAdapter := NewLoggerAdapter(logger)

	backupMgr := NewDailyBackupManager(dbPath, loggerAdapter)
	recoveryMgr := NewStartupRecoveryManager(dbPath, backupMgr, loggerAdapter)

	dbManager := NewDatabaseManagerWithStartupRecovery(dbPath, backupMgr, loggerAdapter)
	sqlDB, err := dbManager.OpenWithRecovery(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database with recovery: %w", err)
	}

	db := sqlx.NewDb(sqlDB, "sqlite3")

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := RunMigrations(db.DB); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	uuid := uuid.New().String()
	_, err = db.ExecContext(ctx, `
		INSERT OR IGNORE INTO config (key, value) 
		VALUES ('telegram_chat_uuid', ?)
	`, uuid)
	if err != nil {
		return nil, err
	}

	// Set database on backup manager before any other operations to prevent race conditions
	backupMgr.SetDatabase(db.DB)

	store := &Store{
		db:          db,
		backupMgr:   backupMgr,
		recoveryMgr: recoveryMgr,
		logger:      logger,
	}

	// Start backup manager after store is fully initialized
	backupMgr.Start(ctx)

	if err = store.InitOAuth(ctx); err != nil {
		return nil, err
	}

	logger.Info("Store initialized with enhanced recovery and daily backup system")
	return store, nil
}

// NewStore creates a new SQLite-backed store with a default logger for backward compatibility.
func NewStore(ctx context.Context, dbPath string) (*Store, error) {
	logger := log.New(os.Stdout)
	logger.SetLevel(log.InfoLevel)
	return NewStoreWithLogger(ctx, dbPath, logger)
}

// Close closes the database connection.
func (s *Store) Close() error {
	if s.backupMgr != nil {
		s.backupMgr.Stop()
	}
	return s.db.Close()
}

// GetBackupStatus returns the current status of the backup system.
func (s *Store) GetBackupStatus() string {
	if s.backupMgr == nil {
		return "Backup system not initialized"
	}
	return s.backupMgr.GetStatus()
}

// CreateManualBackup creates a manual backup of the database.
func (s *Store) CreateManualBackup(ctx context.Context) error {
	if s.backupMgr == nil {
		return fmt.Errorf("backup manager not initialized")
	}
	return s.backupMgr.CreateBackup(ctx)
}

// RestoreFromBackup restores the database from the latest backup.
func (s *Store) RestoreFromBackup(ctx context.Context) error {
	if s.recoveryMgr == nil {
		return fmt.Errorf("recovery manager not initialized")
	}
	return s.recoveryMgr.RestoreFromBackup(ctx)
}

// DB returns the underlying sqlx.DB instance.
func (s *Store) DB() *sqlx.DB {
	db := s.db
	return db
}

func (s *Store) GetValue(ctx context.Context, key string) (string, error) {
	var value sql.NullString

	err := s.db.GetContext(ctx, &value, "SELECT value FROM config WHERE key = ?", key)
	if err != nil {
		return "", err
	}
	if !value.Valid {
		return "", nil
	}

	return value.String, nil
}

func (s *Store) SetValue(ctx context.Context, key string, value string) error {
	_, err := s.db.ExecContext(
		ctx,
		"INSERT OR REPLACE INTO config (key, value) VALUES (?, ?)",
		key,
		value,
	)
	if err != nil {
		return err
	}
	return nil
}

type SourceUsername struct {
	ID          string  `db:"id" json:"id"`
	Source      string  `db:"source" json:"source"`
	Username    string  `db:"username" json:"username"`
	UserID      *string `db:"user_id" json:"user_id,omitempty"`
	FirstName   *string `db:"first_name" json:"first_name,omitempty"`
	LastName    *string `db:"last_name" json:"last_name,omitempty"`
	PhoneNumber *string `db:"phone_number" json:"phone_number,omitempty"`
	Bio         *string `db:"bio" json:"bio,omitempty"`
	CreatedAt   string  `db:"created_at" json:"created_at"`
	UpdatedAt   string  `db:"updated_at" json:"updated_at"`
}

func (s *Store) SetSourceUsername(ctx context.Context, sourceUsername SourceUsername) error {
	if sourceUsername.ID == "" {
		sourceUsername.ID = uuid.New().String()
	}

	_, err := s.db.ExecContext(
		ctx,
		`INSERT OR REPLACE INTO source_usernames 
		(id, source, username, user_id, first_name, last_name, phone_number, bio, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		sourceUsername.ID,
		sourceUsername.Source,
		sourceUsername.Username,
		sourceUsername.UserID,
		sourceUsername.FirstName,
		sourceUsername.LastName,
		sourceUsername.PhoneNumber,
		sourceUsername.Bio,
	)
	return err
}

func (s *Store) GetSourceUsername(ctx context.Context, source string) (*SourceUsername, error) {
	var sourceUsername SourceUsername
	err := s.db.GetContext(ctx, &sourceUsername, "SELECT * FROM source_usernames WHERE source = ?", source)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &sourceUsername, nil
}

func (s *Store) GetAllSourceUsernames(ctx context.Context) ([]SourceUsername, error) {
	var sourceUsernames []SourceUsername
	err := s.db.SelectContext(ctx, &sourceUsernames, "SELECT * FROM source_usernames ORDER BY source")
	if err != nil {
		return nil, err
	}
	return sourceUsernames, nil
}

func (s *Store) DeleteSourceUsername(ctx context.Context, source string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM source_usernames WHERE source = ?", source)
	return err
}
