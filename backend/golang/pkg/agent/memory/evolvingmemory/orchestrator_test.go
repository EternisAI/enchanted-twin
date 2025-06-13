package evolvingmemory

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/charmbracelet/log"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

// BenchmarkDynamicWorkers demonstrates the dynamic worker pattern with varying document sizes.
func BenchmarkDynamicWorkers(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	// Create documents with varying content sizes to simulate different processing times
	createDocuments := func(count int) []memory.Document {
		docs := make([]memory.Document, count)
		for i := 0; i < count; i++ {
			var content string
			switch i % 4 {
			case 0:
				// Small document (fast processing)
				content = fmt.Sprintf("Small doc %d: Quick fact", i)
			case 1:
				// Medium document
				content = fmt.Sprintf("Medium doc %d: %s", i, string(make([]byte, 1000)))
			case 2:
				// Large document (slow processing)
				content = fmt.Sprintf("Large doc %d: %s", i, string(make([]byte, 5000)))
			case 3:
				// Very large document (very slow processing)
				content = fmt.Sprintf("Very large doc %d: %s", i, string(make([]byte, 10000)))
			}

			docs[i] = &memory.TextDocument{
				FieldID:      fmt.Sprintf("doc-%d", i),
				FieldContent: content,
				FieldSource:  "benchmark",
			}
		}
		return docs
	}

	// Test with different worker counts
	workerCounts := []int{1, 2, 4, 8}

	for _, workers := range workerCounts {
		b.Run(fmt.Sprintf("Workers-%d", workers), func(b *testing.B) {
			// Create storage with real services (you'd use your actual config)
			logger := log.Default()
			logger.SetLevel(log.InfoLevel) // Set to Debug to see worker distribution

			// Note: This requires actual AI services to be configured
			// You can replace with mock services for pure benchmarking
			storage, err := createMockStorage(logger)
			if err != nil {
				b.Skip("Skipping benchmark: storage creation failed")
			}

			docs := createDocuments(20) // 20 documents of varying sizes

			config := Config{
				Workers:               workers,
				BatchSize:             100,
				FlushInterval:         5 * time.Second,
				FactExtractionTimeout: 30 * time.Second,
				MemoryDecisionTimeout: 30 * time.Second,
				StorageTimeout:        30 * time.Second,
			}

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				ctx := context.Background()

				start := time.Now()
				progressCh, errorCh := storage.orchestrator.ProcessDocuments(ctx, docs, config)

				// Collect all progress updates
				var lastProgress Progress
				for progress := range progressCh {
					lastProgress = progress
				}

				// Drain errors
				for range errorCh {
					// Just drain
				}

				elapsed := time.Since(start)
				b.Logf("Workers: %d, Documents: %d, Time: %v, Throughput: %.2f docs/sec",
					workers, len(docs), elapsed, float64(len(docs))/elapsed.Seconds())

				if lastProgress.Processed > 0 {
					b.Logf("Processed: %d documents", lastProgress.Processed)
				}
			}
		})
	}
}

// TestWorkerDistribution shows how work is distributed among workers.
func TestWorkerDistribution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping distribution test in short mode")
	}

	logger := log.Default()
	logger.SetLevel(log.DebugLevel) // Enable debug logs to see worker activity

	storage, err := createMockStorage(logger)
	if err != nil {
		t.Skip("Skipping test: storage creation failed")
	}

	// Create documents with predictable processing patterns
	docs := []memory.Document{
		// Fast documents
		&memory.TextDocument{FieldID: "fast-1", FieldContent: "Quick fact 1", FieldSource: "test"},
		&memory.TextDocument{FieldID: "fast-2", FieldContent: "Quick fact 2", FieldSource: "test"},
		&memory.TextDocument{FieldID: "fast-3", FieldContent: "Quick fact 3", FieldSource: "test"},

		// Slow documents (larger content = more processing time)
		&memory.TextDocument{FieldID: "slow-1", FieldContent: string(make([]byte, 10000)), FieldSource: "test"},
		&memory.TextDocument{FieldID: "slow-2", FieldContent: string(make([]byte, 10000)), FieldSource: "test"},

		// Mixed
		&memory.TextDocument{FieldID: "med-1", FieldContent: string(make([]byte, 5000)), FieldSource: "test"},
		&memory.TextDocument{FieldID: "med-2", FieldContent: string(make([]byte, 5000)), FieldSource: "test"},
	}

	config := Config{
		Workers:               3, // 3 workers for 7 documents
		BatchSize:             100,
		FlushInterval:         5 * time.Second,
		FactExtractionTimeout: 30 * time.Second,
		MemoryDecisionTimeout: 30 * time.Second,
		StorageTimeout:        30 * time.Second,
	}

	ctx := context.Background()
	start := time.Now()

	progressCh, errorCh := storage.orchestrator.ProcessDocuments(ctx, docs, config)

	// Monitor progress
	for progress := range progressCh {
		t.Logf("Progress: %d/%d at %v", progress.Processed, progress.Total, time.Since(start))
	}

	// Check for errors
	for err := range errorCh {
		t.Logf("Error: %v", err)
	}

	t.Logf("Total processing time: %v", time.Since(start))
	t.Logf("Check the logs above to see how workers dynamically picked up documents")
	t.Logf("With static distribution, each worker would get 2-3 documents")
	t.Logf("With dynamic distribution, fast workers will process more documents")
}
