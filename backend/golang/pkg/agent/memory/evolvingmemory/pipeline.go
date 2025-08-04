package evolvingmemory

import (
	"time"
)

// DefaultConfig returns reasonable defaults for the processing pipeline.
func DefaultConfig() Config {
	return Config{
		Workers:                4,
		FactsPerWorker:         50,
		BatchSize:              100,
		FlushInterval:          5 * time.Minute, // Reduced from 30s for energy efficiency
		FactExtractionTimeout:  5 * time.Minute, // Reduced from 20min to prevent long-running AI operations
		StorageTimeout:         5 * time.Minute, // Reduced from 20min for better resource management
		EnableRichContext:      true,
		ParallelFactExtraction: true,
		StreamingProgress:      true,
	}
}
