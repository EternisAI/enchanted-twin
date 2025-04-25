package embeddingsmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
)

func (m *EmbeddingsMemory) Store(ctx context.Context, documents []memory.TextDocument) error {
	batches := helpers.Batch(documents, 10)
	for _, batch := range batches {
		textInputs := make([]string, len(batch))
		for i, doc := range batch {
			textInputs[i] = doc.Content
		}
		embeddings, err := m.ai.Embeddings(ctx, textInputs, "text-embedding-3-small")
		if err != nil {
			return err
		}
		if err := m.storeDocuments(ctx, batch, embeddings); err != nil {
			return err
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
	defer tx.Rollback() // safe even after Commit

	// Plain INSERTs – one for text, one for embedding
	textStmt, err := tx.PrepareContext(ctx,
		`INSERT INTO text_entries (id, text, meta, created_at)
		 VALUES ($1, $2, $3, $4)`)
	if err != nil {
		return err
	}
	defer textStmt.Close()

	embedStmt, err := tx.PrepareContext(ctx,
		`INSERT INTO embeddings (text_entry_id, embedding)
		 VALUES ($1, $2::vector)`)
	if err != nil {
		return err
	}
	defer embedStmt.Close()

	for i, doc := range documents {
		metaBytes := []byte("{}")
		if doc.Metadata != nil {
			if metaBytes, err = json.Marshal(doc.Metadata); err != nil {
				return err
			}
		}

		createdAt := time.Now()
		if doc.Timestamp != nil {
			createdAt = *doc.Timestamp
		}

		id := uuid.New().String()

		if _, err := textStmt.ExecContext(ctx, id, doc.Content, metaBytes, createdAt); err != nil {
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
