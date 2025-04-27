package embeddingsmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

func toVectorLiteral(vec []float64) string {
	vals := make([]string, len(vec))
	for i, v := range vec {
		vals[i] = fmt.Sprintf("%f", v)
	}
	return "[" + strings.Join(vals, ",") + "]"
}

type entryRow struct {
	ID        string          `db:"id"`
	Text      string          `db:"text"`
	Meta      json.RawMessage `db:"meta"`
	CreatedAt time.Time       `db:"created_at"`
	Score     float64         `db:"score"`
}

func (m *EmbeddingsMemory) Query(ctx context.Context, query string) (memory.QueryResult, error) {
	// 1) embed
	vec, err := m.ai.Embedding(ctx, query, m.embeddingsModelName)
	if err != nil {
		return memory.QueryResult{}, err
	}
	if len(vec) == 0 {
		return memory.QueryResult{}, fmt.Errorf("empty embedding")
	}
	vecLiteral := toVectorLiteral(vec)

	// 2) ANN search
	const topK = 10
	const sqlQ = `
	    SELECT te.id,
	           te.text,
	           te.meta,
	           te.created_at,
	           1 - (e.embedding <=> $1::vector) AS score
	      FROM embeddings  e
	      JOIN text_entries te ON te.id = e.text_entry_id
	     ORDER BY e.embedding <=> $1::vector
	     LIMIT $2`

	rows, err := m.db.QueryxContext(ctx, sqlQ, vecLiteral, topK)
	if err != nil {
		return memory.QueryResult{}, err
	}
	defer func() { _ = rows.Close() }()

	// 3) stream rows
	var docs []memory.TextDocument
	for rows.Next() {
		var r entryRow
		if err := rows.StructScan(&r); err != nil {
			return memory.QueryResult{}, err
		}

		meta := map[string]string{}
		_ = json.Unmarshal(r.Meta, &meta)

		docs = append(docs, memory.TextDocument{
			ID:        r.ID,
			Content:   r.Text,
			Timestamp: &r.CreatedAt,
			Metadata:  meta,
		})
	}
	if err := rows.Err(); err != nil {
		return memory.QueryResult{}, err
	}

	return memory.QueryResult{
		Text:      []string{query},
		Documents: docs,
	}, nil
}
