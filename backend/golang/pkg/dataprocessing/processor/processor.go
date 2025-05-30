package processor

import (
	"context"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

// Processor interface defines methods that each data source must implement.
type Processor interface {
	// Name returns the source identifier
	Name() string
	// ProcessFile processes exported files
	ProcessFile(ctx context.Context, filepath string, store *db.Store) ([]types.Record, error)
	// Sync returns latest data from the source
	Sync(ctx context.Context) ([]types.Record, error)
	// ToDocuments converts records to documents
	ToDocuments(records []types.Record) ([]memory.Document, error)
}
