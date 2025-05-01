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
func (m *EmbeddingsMemory) Store(
	ctx context.Context,
	documents []memory.TextDocument,
	progressChan chan<- memory.ProgressUpdate,
) error {
	if progressChan != nil {
		defer close(progressChan)
	}

	// max length of a text input to the Embeddings API
	maxBatchTextLength := 8192
	// number of items to Embeddings API
	batchSize := 30
	// number of concurrent requests to the Embeddings API
	semaphoreSize := 3

	filteredDocuments := []memory.TextDocument{}
	for _, document := range documents {
		if document.Content != "" {
			filteredDocuments = append(filteredDocuments, document)
		}
	}

	if len(filteredDocuments) == 0 {
		if progressChan != nil {
			select {
			case progressChan <- memory.ProgressUpdate{Processed: 0, Total: 0}:
			default:
			}
		}
		return nil
	}

	batches := helpers.BatchWithMaxTextLength(filteredDocuments, maxBatchTextLength, batchSize, func(doc memory.TextDocument) int {
		return len(doc.Content)
	})
	totalBatches := len(batches)
	resultChan := make(chan embeddingResult, totalBatches)
	var wg sync.WaitGroup
	var processedBatches atomic.Int32

	sem := make(chan struct{}, semaphoreSize)

	for _, batch := range batches {
		wg.Add(1)
		go func(batchDocs []memory.TextDocument) {
			defer wg.Done()

			sem <- struct{}{}

			defer func() { <-sem }()

			textInputs := make([]string, len(batchDocs))
			for i := range batchDocs {
				textInputs[i] = batchDocs[i].Content
			}

			embeddings, err := m.ai.Embeddings(ctx, textInputs, m.embeddingsModelName)

			resultChan <- embeddingResult{
				documents:  batchDocs,
				embeddings: embeddings,
				err:        err,
			}
		}(batch)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for result := range resultChan {
		if result.err != nil {
			return fmt.Errorf("embedding failed: %w", result.err)
		}

		if err := m.storeDocuments(ctx, result.documents, result.embeddings); err != nil {
			return fmt.Errorf("storing documents failed: %w", err)
		}

		processed := processedBatches.Add(1)

		m.logger.Info("Stored document batch",
			"batchSize", len(result.documents),
			"progress", fmt.Sprintf("%d/%d", processed, totalBatches))

		if progressChan != nil {
			update := memory.ProgressUpdate{Processed: int(processed), Total: totalBatches}
			select {
			case progressChan <- update:

			case <-ctx.Done():

				m.logger.Warn("Context canceled during progress update send")
				return ctx.Err()
			default:

				m.logger.Warn(
					"Progress channel full or receiver not ready, skipping update",
					"processed",
					processed,
					"total",
					totalBatches,
				)
			}
		}
	}

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
