package evolvingmemory

import (
	"time"
)

// DefaultConfig provides default configuration values.
func DefaultConfig() Config {
	return Config{
		Workers:                4,
		FactsPerWorker:         50,
		BatchSize:              100,
		FlushInterval:          30 * time.Second,
		FactExtractionTimeout:  5 * time.Minute,
		MemoryDecisionTimeout:  5 * time.Minute,
		StorageTimeout:         5 * time.Minute,
		EnableRichContext:      true,
		ParallelFactExtraction: true,
		StreamingProgress:      true,
	}
}
