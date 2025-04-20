package graphmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/log"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
	openai "github.com/openai/openai-go"
)

func (g *GraphMemory) Query(
	ctx context.Context,
	nlQuestion string,
) (memory.QueryResult, error) {

	var out memory.QueryResult
	qlist, err := g.GenerateSQLQueries(ctx, nlQuestion)
	if err != nil {
		return out, err
	}

	g.logger.Debug("Generated queries", "queries", qlist)

	seenDoc := make(map[string]struct{})

	for _, q := range qlist {
		log.Debug("Database query", "query", q.Query)
		rows, err := g.db.QueryContext(ctx, q.Query)
		if err != nil {
			return out, err
		}

		textIdx := indexOf(q.Columns, "text")

		for rows.Next() {
			vals := make([]any, len(q.Columns))
			ptrs := make([]any, len(vals))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				_ = rows.Close()
				return out, err
			}

			if textIdx >= 0 { // --- sentence documents -------------
				id := ""
				if idx := indexOf(q.Columns, "id"); idx >= 0 {
					id = fmt.Sprint(vals[idx])
				} else {
					id = strconv.Itoa(len(out.Documents) + 1)
				}
				if _, dup := seenDoc[id]; dup {
					continue
				}

				doc := memory.TextDocument{
					ID:      id,
					Content: fmt.Sprint(vals[textIdx]),
				}
				if ts := indexOf(q.Columns, "timestamp"); ts >= 0 {
					if t, ok := vals[ts].(time.Time); ok {
						doc.Timestamp = &t
					}
				}
				out.Documents = append(out.Documents, doc)
				seenDoc[id] = struct{}{}

			} else { // --- aggregate or misc rows ------------------
				var parts []string
				for i, a := range q.Columns {
					parts = append(parts, fmt.Sprintf("%s=%v", a, vals[i]))
				}
				out.Text = append(out.Text, strings.Join(parts, ", "))
			}
		}
		_ = rows.Close()
	}

	return out, nil
}

func indexOf(sl []string, key string) int {
	for i, s := range sl {
		if s == key {
			return i
		}
	}
	return -1
}

const schema = `
-- Drop existing tables if they exist
DROP TABLE IF EXISTS embeddings;
DROP TABLE IF EXISTS facts;
DROP TABLE IF EXISTS text_entries;

-- Table for storing raw text entries and metadata
CREATE TABLE text_entries (
                              id BIGSERIAL PRIMARY KEY,
                              text TEXT NOT NULL,
                              metadata JSONB,  -- Allows storing additional metadata as JSON
                              UNIQUE(text, metadata)
);

-- Table for storing extracted structured facts
CREATE TABLE facts (
                       id BIGSERIAL PRIMARY KEY,
                       text_entry_id BIGINT NOT NULL,
                       sub TEXT NOT NULL,  -- Subject
                       prd TEXT NOT NULL,  -- Predicate
                       obj TEXT NOT NULL,  -- Object
                       FOREIGN KEY (text_entry_id) REFERENCES text_entries(id) ON DELETE CASCADE
);

-- Create GIN indexes for fulltext search on subject and object
CREATE INDEX facts_sub_fulltext_idx ON facts USING GIN (to_tsvector('english', sub));
CREATE INDEX facts_obj_fulltext_idx ON facts USING GIN (to_tsvector('english', obj));

CREATE INDEX facts_sub_idx ON facts(sub);
CREATE INDEX facts_prd_idx ON facts(prd);
CREATE INDEX facts_obj_idx ON facts(obj);
`

type queryPayload struct {
	Query   string   `json:"query"`
	Columns []string `json:"columns"`
}

func (g *GraphMemory) GenerateSQLQueries(
	ctx context.Context,
	question string,
) ([]queryPayload, error) {

	subs, prds, objs, err := g.getUniqueTripleValues(ctx)
	if err != nil {
		return nil, err
	}

	prompt := createSQLGenerationPrompt(question, subs, prds, objs)
	g.logger.Debug("Generated prompt", "prompt", prompt)

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(prompt),
	}

	resp, err := g.ai.Completions(
		ctx,
		messages,
		[]openai.ChatCompletionToolParam{extractSQLQueriesToolDefinition()},
		"gpt-4o-mini",
	)
	if err != nil {
		return nil, err
	}

	var wrap struct {
		Queries []queryPayload `json:"queries"`
	}
	for _, tc := range resp.ToolCalls {
		if tc.Function.Name == "submit_sql_queries" {
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &wrap); err != nil {
				return nil, err
			}
		}
	}
	if len(wrap.Queries) == 0 {
		return nil, fmt.Errorf("GPT returned no queries")
	}
	return wrap.Queries, nil
}

// graphmemory/prompt.go
func createSQLGenerationPrompt(
	nlQuestion string,
	uniqueSubs, uniquePrds, uniqueObjs []string,
) string {

	subs := strings.Join(helpers.SafeSlice(uniqueSubs, 10), ", ")
	prds := strings.Join(uniquePrds, ", ")
	objs := strings.Join(helpers.SafeSlice(uniqueObjs, 10), ", ")

	examples := ""
	if len(uniqueSubs)+len(uniquePrds)+len(uniqueObjs) > 0 {
		examples = fmt.Sprintf(`
Example values in DB
  • Subjects  : %s
  • Predicates: %s
  • Objects   : %s
`, subs, prds, objs)
	}

	return fmt.Sprintf(`
You are an expert PostgreSQL assistant.

=================================================
SCHEMA (abridged)
-------------------------------------------------
%s
=================================================

INSTRUCTIONS
-------------------------------------------------
1. Produce **one or more** SQL statements that collectively answer the question.
2. When querying facts perform a JOIN with 'text_entries' on 'text_entry_id' to obtain 'text'.
3. For each statement list every column alias (left‑to‑right).
   · Use alias **text** when the column holds original sentences.
   · Use alias **value** for scalar answers like COUNT(*).

<START OF QUESTION>
%s
</START OF QUESTION>

<EXAMPLES>
%s
</EXAMPLES>
`, schema, nlQuestion, examples)
}
