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
		FactExtractionTimeout:  20 * time.Minute,
		MemoryDecisionTimeout:  20 * time.Minute,
		StorageTimeout:         20 * time.Minute,
		EnableRichContext:      true,
		ParallelFactExtraction: true,
		StreamingProgress:      true,
	}
}

func findMemoryByID(memories []ExistingMemory, id string) *ExistingMemory {
	for i := range memories {
		if memories[i].ID == id {
			return &memories[i]
		}
	}
	return nil
}
