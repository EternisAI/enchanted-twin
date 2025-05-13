package podman

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	charmlog "github.com/charmbracelet/log"
	_ "github.com/lib/pq"
)

// TestDbConnection tests connectivity to the given PostgreSQL connection string.
// It lists tables in the connected database to verify read access and provides
// diagnostic logging. It retries a few times with exponential back-off before
// returning an error.
func TestDbConnection(ctx context.Context, connString string, logger *charmlog.Logger) error {
	logger.Info("Attempting direct database connection", "connectionString", connString)

	const maxAttempts = 5
	for i := 0; i < maxAttempts; i++ {
		db, err := sql.Open("postgres", connString)
		if err == nil {
			defer db.Close() //nolint:errcheck

			pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err = db.PingContext(pingCtx)
			cancel()
			if err == nil {
				logger.Info("Direct database connection successful")

				var result int
				if err := db.QueryRow("SELECT 1").Scan(&result); err == nil && result == 1 {
					logger.Info("Database query successful", "result", result)

					listQuery := `SELECT table_name FROM information_schema.tables WHERE table_schema='public' ORDER BY table_name`
					rows, qErr := db.Query(listQuery)
					if qErr != nil {
						logger.Warn("Failed to list tables", "error", qErr)
					} else {
						defer rows.Close() //nolint:errcheck
						var tables []string
						for rows.Next() {
							var tbl string
							if rows.Scan(&tbl) == nil {
								tables = append(tables, tbl)
							}
						}
						if len(tables) == 0 {
							logger.Info("No tables found in database (empty database)")
						} else {
							logger.Info("Tables in database", "count", len(tables), "tables", tables)
						}
					}
					return nil
				}
			} else {
				logger.Warn("Database ping failed", "error", err)
				if strings.Contains(err.Error(), "does not exist") {
					logger.Error("Database in connection string does not exist")
				}
			}
		} else {
			logger.Warn("Failed to open database connection", "error", err)
		}

		wait := time.Duration(1<<uint(i)) * time.Second
		logger.Warn("Database connection attempt failed, retrying", "attempt", i+1, "max", maxAttempts, "wait", wait)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}

	return fmt.Errorf("unable to connect to database after %d attempts", maxAttempts)
}
