package graphmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
	"github.com/charmbracelet/log"
	openai "github.com/openai/openai-go"
)

// graphmemory/query.go.
func (g *GraphMemory) Query(
	ctx context.Context,
	nlQuestion string,
) (memory.QueryResult, error) {
	g.logger.Debug("Query called",
		"question", nlQuestion,
	)

	var out memory.QueryResult
	seenDoc := make(map[string]struct{})

	// 1. Ask the LLM for the SQL we should run
	qlist, err := g.GenerateSQLQueries(ctx, nlQuestion)
	if err != nil {
		return out, fmt.Errorf("GenerateSQLQueries: %w", err)
	}
	g.logger.Debug("LLM produced SQL queries", "count", len(qlist), "queries", qlist)

	// 2. Execute each query and transform rows → QueryResult
	for qi, q := range qlist {
		g.logger.Debug("Running query",
			"idx", qi,
			"sql", q.Query,
			"llmColumns", q.Columns,
		)

		rows, err := g.db.QueryContext(ctx, q.Query)
		if err != nil {
			return out, fmt.Errorf("db.QueryContext: %w", err)
		}

		// Always close rows, even if we return early
		func() {
			defer func() {
				if err := rows.Close(); err != nil {
					g.logger.Error("Failed to close rows", "error", err)
				}
			}()

			// --- authoritative column list from the driver -------------
			dbCols, err := rows.Columns()
			if err != nil {
				log.Error("Rows.Columns failed", "err", err)
				return
			}

			cols := dbCols // default
			if len(q.Columns) == 0 {
				g.logger.Warn("LLM omitted column list – falling back to DB columns",
					"dbCols", dbCols)
			} else if len(q.Columns) != len(dbCols) {
				g.logger.Warn("Column count mismatch – using DB columns",
					"llmCols", len(q.Columns),
					"dbCols", len(dbCols))
			} else {
				cols = q.Columns
			}

			textIdx := indexOf(cols, "text")

			// --- stream rows ------------------------------------------
			for rows.Next() {
				vals := make([]any, len(cols))
				ptrs := make([]any, len(vals))
				for i := range vals {
					ptrs[i] = &vals[i]
				}
				if err := rows.Scan(ptrs...); err != nil {
					log.Error("rows.Scan failed", "err", err)
					return
				}

				if textIdx >= 0 { // document‑like row
					// ---------- dedup & enrich ----------
					idIdx := indexOf(cols, "id")
					id := strconv.Itoa(len(out.Documents) + 1)
					if idIdx >= 0 {
						id = fmt.Sprint(vals[idIdx])
					}
					if _, dup := seenDoc[id]; dup {
						continue
					}

					doc := memory.TextDocument{
						ID:      id,
						Content: fmt.Sprint(vals[textIdx]),
					}
					if tsIdx := indexOf(cols, "timestamp"); tsIdx >= 0 {
						if t, ok := vals[tsIdx].(time.Time); ok {
							doc.Timestamp = &t
						}
					}
					out.Documents = append(out.Documents, doc)
					seenDoc[id] = struct{}{}
				} else { // aggregate / misc row
					var parts []string
					for i := range vals {
						name := dbCols[i] // always in‑range
						if i < len(q.Columns) && q.Columns[i] != "" {
							name = q.Columns[i]
						}
						parts = append(parts, fmt.Sprintf("%s=%v", name, vals[i]))
					}
					out.Text = append(out.Text, strings.Join(parts, ", "))
				}
			}

			if err := rows.Err(); err != nil {
				log.Error("Row iteration error", "err", err)
			}
		}()
	}

	g.logger.Debug("Query complete",
		"documents", len(out.Documents),
		"aggregateLines", len(out.Text),
	)
	return out, nil
}

// indexOf returns the position of key in sl or ‑1 if absent.
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
		g.completionsModelName,
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

// graphmemory/prompt.go.
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
  • Subjects: %s
  • Predicates: %s
  • Objects: %s
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
1. Produce **one or more** SQL statements that collectively answer the question. Do not return more than 100 documents together. Count queries do not need a limit.
2. When querying facts perform a JOIN with 'text_entries' on 'text_entry_id' to obtain 'text'.
3. For each statement list every column alias (left‑to‑right).
   · Use alias **text** when the column holds original sentences.
   · Use alias **value** for scalar answers like COUNT(*).
<EXAMPLES>
%s
</EXAMPLES>

<START OF QUESTION>
%s
</START OF QUESTION>

`, schema, examples, nlQuestion)
}
