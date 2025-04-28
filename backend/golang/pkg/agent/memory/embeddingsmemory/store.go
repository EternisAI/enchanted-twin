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

func (m *EmbeddingsMemory) Store(ctx context.Context, documents []memory.TextDocument) error {
	batchSize := 30
	// filter out empty documents
	filteredDocuments := []memory.TextDocument{}
	for _, document := range documents {
		if document.Content != "" {
			filteredDocuments = append(filteredDocuments, document)
		}
	}

	batches := helpers.Batch(filteredDocuments, batchSize)
	resultChan := make(chan embeddingResult, len(batches))
	var wg sync.WaitGroup
	var processedBatches atomic.Int32
	sem := make(chan struct{}, 3)

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
			return result.err
		}
		if err := m.storeDocuments(ctx, result.documents, result.embeddings); err != nil {
			return err
		}

		// Increment and log progress after each batch is processed
		processed := processedBatches.Add(1)
		m.logger.Info("Storing documents batch",
			"batchSize", batchSize,
			"progress", fmt.Sprintf("%d/%d", processed, len(batches)))
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

		vals := make([]string, len(embeddings[i]))
		for j, v := range embeddings[i] {
			vals[j] = fmt.Sprintf("%f", v)
		}
		vecLiteral := "[" + strings.Join(vals, ",") + "]"

		if _, err := embedStmt.ExecContext(ctx, id, vecLiteral); err != nil {
			return err
		}
	}

	return tx.Commit()
}
