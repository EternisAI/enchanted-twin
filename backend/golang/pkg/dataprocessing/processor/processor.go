package processor

import (
	"context"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
)

// Processor interface defines methods that each data source must implement.
type Processor interface {
	// Name returns the source identifier
	Name() string
	// ProcessFile processes exported files
	ProcessFile(ctx context.Context, filepath string) ([]types.Record, error)
	// ProcessDirectory processes a directory of files
	ProcessDirectory(ctx context.Context, filepath string) ([]types.Record, error)
	// Sync returns latest data from the source
	Sync(ctx context.Context, accessToken string) ([]types.Record, bool, error)
	// ToDocuments converts records to documents
	ToDocuments(ctx context.Context, records []types.Record) ([]memory.Document, error)
}
