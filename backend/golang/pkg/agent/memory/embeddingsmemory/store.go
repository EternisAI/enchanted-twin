package embeddingsmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
)

type embeddingResult struct {
	documents  []memory.TextDocument
	embeddings [][]float64
	err        error
}

// Store processes documents, generates embeddings, and stores them.
// It optionally sends progress updates via the progressChan.
func (m *EmbeddingsMemory) Store(ctx context.Context, documents []memory.TextDocument, progressChan chan<- memory.ProgressUpdate) error {
	if progressChan != nil {
		// Ensure the channel is closed when the function returns
		defer close(progressChan)
	}

	batchSize := 30
	// filter out empty documents
	filteredDocuments := []memory.TextDocument{}
	for _, document := range documents {
		if document.Content != "" {
			filteredDocuments = append(filteredDocuments, document)
		}
	}

	// Return early if there are no documents to process
	if len(filteredDocuments) == 0 {
		// Send a zero progress update if channel exists
		if progressChan != nil {
			select {
			case progressChan <- memory.ProgressUpdate{Processed: 0, Total: 0}:
			default:
			}
		}
		return nil
	}

	batches := helpers.Batch(filteredDocuments, batchSize)
	totalBatches := len(batches) // Store total batches count
	resultChan := make(chan embeddingResult, totalBatches)
	var wg sync.WaitGroup
	var processedBatches atomic.Int32
	// Limit concurrent embedding calls
	sem := make(chan struct{}, 3) // Consider making concurrency configurable

	for _, batch := range batches {
		wg.Add(1)
		go func(batchDocs []memory.TextDocument) {
			defer wg.Done()
			// Acquire semaphore
			sem <- struct{}{}
			// Release semaphore
			defer func() { <-sem }()

			textInputs := make([]string, len(batchDocs))
			for i := range batchDocs {
				textInputs[i] = batchDocs[i].Content
			}

			// Generate embeddings
			embeddings, err := m.ai.Embeddings(ctx, textInputs, m.embeddingsModelName)
			// Send result (including potential error) to channel
			resultChan <- embeddingResult{
				documents:  batchDocs,
				embeddings: embeddings,
				err:        err,
			}
		}(batch)
	}

	// Goroutine to close resultChan once all embedding workers are done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Process results as they come in
	for result := range resultChan {
		// Handle embedding errors
		if result.err != nil {
			// Don't send progress update on error, just return
			return fmt.Errorf("embedding failed: %w", result.err)
		}

		// Store the documents and embeddings for the current batch
		if err := m.storeDocuments(ctx, result.documents, result.embeddings); err != nil {
			// Don't send progress update on error, just return
			return fmt.Errorf("storing documents failed: %w", err)
		}

		// Increment processed count *after* successful storage
		processed := processedBatches.Add(1)

		// Log progress internally
		m.logger.Info("Stored document batch",
			"batchSize", len(result.documents), // Log actual batch size
			"progress", fmt.Sprintf("%d/%d", processed, totalBatches))

		// Send progress update to the external channel if provided
		if progressChan != nil {
			update := memory.ProgressUpdate{Processed: int(processed), Total: totalBatches}
			// Use non-blocking send to avoid deadlocks if the receiver stops listening
			select {
			case progressChan <- update:
			// Update sent successfully
			case <-ctx.Done():
				// If the context is cancelled, stop trying to send progress
				m.logger.Warn("Context cancelled during progress update send")
				return ctx.Err()
			default:
				// Channel buffer is full or channel is closed/nil receiver
				// Log this occurrence as it might indicate an issue with the receiver
				m.logger.Warn("Progress channel full or receiver not ready, skipping update", "processed", processed, "total", totalBatches)
			}
		}
	}

	// All batches processed successfully
	return nil
}

func (m *EmbeddingsMemory) storeDocuments(
	ctx context.Context,
	documents []memory.TextDocument,
	embeddings [][]float64,
) error {
	if len(documents) != len(embeddings) {
		return fmt.Errorf("len(documents) != len(embeddings)")
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Plain INSERTs – one for text, one for embedding
	textStmt, err := tx.PrepareContext(ctx,
		`INSERT INTO text_entries (id, text, meta, created_at)
		 VALUES ($1, $2, $3, $4)`)
	if err != nil {
		return err
	}
	defer func() { _ = textStmt.Close() }()

	embedStmt, err := tx.PrepareContext(ctx,
		`INSERT INTO embeddings (text_entry_id, embedding)
		 VALUES ($1, $2::vector)`)
	if err != nil {
		return err
	}
	defer func() { _ = embedStmt.Close() }()

	for i := range documents {
		metaBytes := []byte("{}")
		if documents[i].Metadata != nil {
			if metaBytes, err = json.Marshal(documents[i].Metadata); err != nil {
				return err
			}
		}

		createdAt := time.Now()
		if documents[i].Timestamp != nil {
			createdAt = *documents[i].Timestamp
		}

		id := uuid.New().String()

		if _, err := textStmt.ExecContext(ctx, id, documents[i].Content, metaBytes, createdAt); err != nil {
			return err
		}

		vec := padVector(embeddings[i], EmbeddingLength)
		vals := make([]string, len(vec))
		for j, v := range vec {
			vals[j] = fmt.Sprintf("%f", v)
		}
		vecLiteral := "[" + strings.Join(vals, ",") + "]"

		if _, err := embedStmt.ExecContext(ctx, id, vecLiteral); err != nil {
			return err
		}
	}

	return tx.Commit()
}
