package graphmemory

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"

	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/openai/openai-go"
)

type GraphMemory struct {
	db *sql.DB
	ai *ai.Service
}

func NewGraphMemory(pgString string, ai *ai.Service, recreate bool) (*GraphMemory, error) {
	db, err := sql.Open("postgres", pgString)
	if err != nil {
		return nil, err
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	mem := &GraphMemory{db: db, ai: ai}

	// Create schema with proper error handling
	if err := mem.ensureDbSchema(recreate); err != nil {
		// Close database connection on error
		if closeErr := db.Close(); closeErr != nil {
			return nil, fmt.Errorf("failed to create database schema: %w (close error: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("failed to create database schema: %w", err)
	}

	return mem, nil
}

// EntryInfo holds information about a text entry to process
type EntryInfo struct {
	index       int
	textEntryID int64
	text        string
	tags        []string
}

// Constants for parallel processing
const (
	MaxConcurrentWorkers = 5
)

// Fact represents a subject-predicate-object triple extracted from text
type Fact struct {
	ID          int64
	TextEntryID int64
	Sub         string // Subject
	Prd         string // Predicate
	Obj         string // Object
}

func (m *GraphMemory) Store(ctx context.Context, documents []memory.TextDocument) error {
	if len(documents) == 0 {
		return nil
	}

	// Prepare entries and store them in the database
	entriesToProcess, err := m.prepareTextEntries(ctx, documents)
	if err != nil {
		return err
	}

	// Process facts in parallel
	allFacts, errs := m.extractAndStoreFacts(ctx, entriesToProcess)

	// Check for errors during fact extraction
	for i, err := range errs {
		if err != nil {
			fmt.Printf("Error processing entry %d: %v\n", i, err)
		}
	}

	// Print summary of facts extracted
	totalFacts := 0
	for _, facts := range allFacts {
		totalFacts += len(facts)
	}
	fmt.Printf("Processed %d entries, extracted %d facts total\n", len(entriesToProcess), totalFacts)

	return nil
}

// prepareTextEntries inserts text documents into the database and prepares them for further processing
func (m *GraphMemory) prepareTextEntries(ctx context.Context, documents []memory.TextDocument) ([]EntryInfo, error) {
	// Begin a transaction
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	entriesToProcess := make([]EntryInfo, 0, len(documents))

	for i, doc := range documents {
		// Prepare metadata for non-tag fields
		var metaMap map[string]string

		// Store the document timestamp in metadata if available
		if doc.Timestamp != nil {
			if metaMap == nil {
				metaMap = make(map[string]string)
			}
			metaMap["timestamp"] = doc.Timestamp.Format(time.RFC3339)
		}

		// Store the document ID in metadata if available
		if doc.ID != "" {
			if metaMap == nil {
				metaMap = make(map[string]string)
			}
			metaMap["id"] = doc.ID
		}

		// Convert meta map to PostgreSQL hstore format string
		var metaStr string
		if len(metaMap) > 0 {
			pairs := make([]string, 0, len(metaMap))
			for k, v := range metaMap {
				pairs = append(pairs, fmt.Sprintf("\"%s\"=>\"%s\"", k, v))
			}
			metaStr = strings.Join(pairs, ", ")
		}

		// Prepare tags for ltree array
		var tagsArray []string
		if len(doc.Tags) > 0 {
			tagsArray = make([]string, 0, len(doc.Tags))
			for _, tag := range doc.Tags {
				// Convert tag to valid ltree format (replace spaces/special chars with underscores)
				formattedTag := strings.ReplaceAll(tag, " ", "_")
				formattedTag = strings.ReplaceAll(formattedTag, "-", "_")
				tagsArray = append(tagsArray, formattedTag)
			}
		}

		// Format tags as PostgreSQL array string
		var tagsStr string
		if len(tagsArray) > 0 {
			escapedTags := make([]string, 0, len(tagsArray))
			for _, tag := range tagsArray {
				escapedTags = append(escapedTags, "\""+tag+"\"")
			}
			tagsStr = "{" + strings.Join(escapedTags, ",") + "}"
		}

		// Insert the text entry with appropriate handling of NULL values
		var textEntryID int64
		var query string

		if metaStr == "" && tagsStr == "" {
			// No metadata or tags
			query = `
				INSERT INTO text_entries (text, meta, tags)
				VALUES ($1, NULL, NULL)
				ON CONFLICT (text, COALESCE(meta, ''))
				DO UPDATE SET id = text_entries.id
				RETURNING id`
			err = tx.QueryRowContext(ctx, query, doc.Content).Scan(&textEntryID)
		} else if metaStr == "" {
			// Only tags, no metadata
			query = `
				INSERT INTO text_entries (text, meta, tags)
				VALUES ($1, NULL, $2::ltree[])
				ON CONFLICT (text, COALESCE(meta, ''))
				DO UPDATE SET tags = $2::ltree[]
				RETURNING id`
			err = tx.QueryRowContext(ctx, query, doc.Content, tagsStr).Scan(&textEntryID)
		} else if tagsStr == "" {
			// Only metadata, no tags
			query = `
				INSERT INTO text_entries (text, meta, tags)
				VALUES ($1, $2::hstore, NULL)
				ON CONFLICT (text, COALESCE(meta, ''))
				DO UPDATE SET id = text_entries.id
				RETURNING id`
			err = tx.QueryRowContext(ctx, query, doc.Content, metaStr).Scan(&textEntryID)
		} else {
			// Both metadata and tags
			query = `
				INSERT INTO text_entries (text, meta, tags)
				VALUES ($1, $2::hstore, $3::ltree[])
				ON CONFLICT (text, COALESCE(meta, ''))
				DO UPDATE SET tags = $3::ltree[]
				RETURNING id`
			err = tx.QueryRowContext(ctx, query, doc.Content, metaStr, tagsStr).Scan(&textEntryID)
		}

		if err != nil {
			return nil, fmt.Errorf("error inserting text entry: %w", err)
		}

		// Add to entries to process
		entriesToProcess = append(entriesToProcess, EntryInfo{
			index:       i,
			textEntryID: textEntryID,
			text:        doc.Content,
			tags:        doc.Tags,
		})
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("error committing transaction: %w", err)
	}

	return entriesToProcess, nil
}

// extractAndStoreFacts processes text entries in parallel to extract facts using AI
func (m *GraphMemory) extractAndStoreFacts(ctx context.Context, entries []EntryInfo) ([][]Fact, []error) {
	allFacts := make([][]Fact, len(entries))
	allErrors := make([]error, len(entries))

	// Skip processing if no AI service is available
	if m.ai == nil {
		fmt.Println("No AI service available, skipping fact extraction")
		return allFacts, allErrors
	}

	// Create a semaphore to limit concurrent API calls
	sem := make(chan struct{}, MaxConcurrentWorkers)
	var wg sync.WaitGroup

	// Use a mutex to protect concurrent map access
	var mutex sync.Mutex

	for _, entry := range entries {
		wg.Add(1)
		go func(idx int, entryID int64, text string) {
			defer wg.Done()

			// Acquire semaphore slot
			sem <- struct{}{}
			defer func() { <-sem }()

			// Extract facts using AI
			fmt.Printf("Worker %d: Extracting facts from text (length: %d chars, ID: %d)\n",
				idx, len(text), entryID)

			facts, err := m.extractFacts(ctx, text, entryID)

			// Safely update the results
			mutex.Lock()
			defer mutex.Unlock()

			if err != nil {
				fmt.Printf("ERROR worker %d: extracting facts: %v\n", idx, err)
				allErrors[idx] = fmt.Errorf("error extracting facts: %w", err)
				return
			}

			fmt.Printf("Worker %d: Successfully extracted %d facts\n", idx, len(facts))
			allFacts[idx] = facts
		}(entry.index, entry.textEntryID, entry.text)
	}

	// Wait for all goroutines to complete
	fmt.Println("Waiting for all fact extraction workers to complete...")
	wg.Wait()
	fmt.Println("All fact extraction workers completed")

	return allFacts, allErrors
}

// extractFacts uses the AI service to extract subject-predicate-object triples from text
func (m *GraphMemory) extractFacts(ctx context.Context, text string, textEntryID int64) ([]Fact, error) {
	// Check if AI service is available
	if m.ai == nil {
		return nil, fmt.Errorf("AI service is not available for fact extraction")
	}

	// Get unique triple values to provide context to AI
	uniqueSubs, uniquePrds, uniqueObjs, err := m.getUniqueTripleValues(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting unique triple values: %w", err)
	}

	// Create AI prompt with instructions and examples
	promptText := createFactExtractionPrompt(text, uniqueSubs, uniquePrds, uniqueObjs)

	// Create the system and user messages for the OpenAI API
	systemPrompt := "You are a fact extraction assistant that identifies subject-predicate-object triples from text. Extract factual triples in the format 'Subject | Predicate | Object'. Each triple should be on a new line. Be precise and extract only clear factual statements."

	// Prepare messages for OpenAI API
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
		openai.UserMessage(promptText),
	}

	// Use default model if not specified
	model := "gpt-4o"

	// Call the OpenAI API
	response, err := m.ai.Completions(ctx, messages, nil, model)
	if err != nil {
		return nil, fmt.Errorf("error calling AI service: %w", err)
	}

	// Parse the response to extract facts
	facts := m.parseAIFactResponse(response.Content, textEntryID)
	fmt.Printf("AI extracted %d facts from text\n", len(facts))

	// Store the facts in the database
	storedFacts, err := m.storeFacts(ctx, facts, textEntryID)
	if err != nil {
		return nil, fmt.Errorf("error storing facts: %w", err)
	}

	return storedFacts, nil
}

// parseAIFactResponse parses the AI's response and extracts facts
func (m *GraphMemory) parseAIFactResponse(content string, textEntryID int64) []Fact {
	facts := []Fact{}

	// Split content into lines
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) < 3 {
			continue // Skip very short lines
		}

		// Skip lines that don't look like facts (should contain at least one pipe)
		if !strings.Contains(line, "|") {
			continue
		}

		// Parse lines in "Subject | Predicate | Object" format
		parts := strings.Split(line, "|")
		if len(parts) >= 3 {
			subject := strings.TrimSpace(parts[0])
			predicate := strings.TrimSpace(parts[1])
			object := strings.TrimSpace(parts[2])

			// If there are more than 3 parts, combine the rest into the object
			if len(parts) > 3 {
				for i := 3; i < len(parts); i++ {
					object += " | " + strings.TrimSpace(parts[i])
				}
			}

			// Only add if all parts are non-empty
			if len(subject) > 0 && len(predicate) > 0 && len(object) > 0 {
				facts = append(facts, Fact{
					TextEntryID: textEntryID,
					Sub:         subject,
					Prd:         predicate,
					Obj:         object,
				})
			}
		}
	}

	return facts
}

// storeFacts inserts facts into the database
func (m *GraphMemory) storeFacts(ctx context.Context, facts []Fact, textEntryID int64) ([]Fact, error) {
	if len(facts) == 0 {
		return []Fact{}, nil
	}

	// Begin a transaction
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	storedFacts := make([]Fact, 0, len(facts))

	for _, fact := range facts {
		var factID int64
		err := tx.QueryRowContext(ctx,
			"INSERT INTO facts (text_entry_id, sub, prd, obj) VALUES ($1, $2, $3, $4) RETURNING id",
			textEntryID, fact.Sub, fact.Prd, fact.Obj).Scan(&factID)

		if err != nil {
			return nil, fmt.Errorf("error inserting fact: %w", err)
		}

		fact.ID = factID
		storedFacts = append(storedFacts, fact)
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("error committing transaction: %w", err)
	}

	return storedFacts, nil
}

// getUniqueTripleValues retrieves unique subjects, predicates, and objects from the database
func (m *GraphMemory) getUniqueTripleValues(ctx context.Context) ([]string, []string, []string, error) {
	// Helper function to query and process unique values
	getUniqueValues := func(column string) ([]string, error) {
		var values []string
		rows, err := m.db.QueryContext(ctx, fmt.Sprintf("SELECT DISTINCT %s FROM facts LIMIT 100", column))
		if err != nil {
			return nil, err
		}
		defer func() {
			if closeErr := rows.Close(); closeErr != nil {
				err = fmt.Errorf("error closing rows: %v (original error: %w)", closeErr, err)
			}
		}()

		for rows.Next() {
			var value string
			if err := rows.Scan(&value); err == nil && value != "" {
				// Trim long values
				if len(value) > 30 {
					value = value[:27] + "..."
				}
				values = append(values, value)
			}
		}

		if err := rows.Err(); err != nil {
			return nil, err
		}

		return values, nil
	}

	subs, err := getUniqueValues("sub")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error getting unique subjects: %w", err)
	}

	preds, err := getUniqueValues("prd")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error getting unique predicates: %w", err)
	}

	objs, err := getUniqueValues("obj")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error getting unique objects: %w", err)
	}

	return subs, preds, objs, nil
}

// createFactExtractionPrompt creates a prompt for the AI to extract facts from text
func createFactExtractionPrompt(text string, uniqueSubs, uniquePreds, uniqueObjs []string) string {
	prompt := "Extract subject-predicate-object facts from the following text.\n\n"

	// Add some examples of existing facts to guide the AI
	if len(uniqueSubs) > 0 && len(uniquePreds) > 0 && len(uniqueObjs) > 0 {
		prompt += "Here are some examples of the kind of facts already in the database:\n"

		// Add up to 5 example triples
		max := min(5, min(len(uniqueSubs), min(len(uniquePreds), len(uniqueObjs))))
		for i := 0; i < max; i++ {
			prompt += fmt.Sprintf("- %s | %s | %s\n", uniqueSubs[i], uniquePreds[i], uniqueObjs[i])
		}
		prompt += "\n"
	}

	// Provide format instructions
	prompt += "Please extract facts in the format: Subject | Predicate | Object\n\n"
	prompt += "Text to process:\n" + text

	return prompt
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (m *GraphMemory) Query(ctx context.Context, query string) ([]memory.TextDocument, error) {
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	// First, try to find exact text matches
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, text, meta, tags, created_at 
		FROM text_entries 
		WHERE text ILIKE $1
		ORDER BY created_at DESC
		LIMIT 10
	`, "%"+query+"%")

	if err != nil {
		return nil, fmt.Errorf("error querying text entries: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = fmt.Errorf("error closing rows: %v (original error: %w)", closeErr, err)
		}
	}()

	documents := []memory.TextDocument{}

	for rows.Next() {
		var id int64
		var text string
		var meta sql.NullString
		var tags sql.NullString // ltree[] will be read as text in this simple implementation
		var createdAt time.Time

		if err := rows.Scan(&id, &text, &meta, &tags, &createdAt); err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}

		// Parse meta data to extract document ID
		docID := fmt.Sprintf("doc-%d", id)
		metaMap := map[string]string{}

		if meta.Valid {
			// Parse hstore format "key"=>"value" - this is a simple implementation
			metaStr := meta.String
			if metaStr != "" {
				entries := strings.Split(metaStr, ",")
				for _, entry := range entries {
					keyValue := strings.Split(entry, "=>")
					if len(keyValue) == 2 {
						key := strings.Trim(strings.TrimSpace(keyValue[0]), "\"")
						value := strings.Trim(strings.TrimSpace(keyValue[1]), "\"")
						metaMap[key] = value

						// Extract ID if present
						if key == "id" {
							docID = value
						}
					}
				}
			}
		}

		// Parse tags from string to string array
		// This is a simple implementation that assumes tags format: {tag1,tag2,tag3}
		tagList := []string{}
		if tags.Valid && tags.String != "" {
			tagsStr := tags.String
			// Remove curly braces
			tagsStr = strings.Trim(tagsStr, "{}")
			if tagsStr != "" {
				// Split by comma
				tagItems := strings.Split(tagsStr, ",")
				for _, tag := range tagItems {
					tag = strings.Trim(strings.TrimSpace(tag), "\"")
					if tag != "" {
						tagList = append(tagList, tag)
					}
				}
			}
		}

		// Create timestamp pointer
		timestamp := &createdAt

		documents = append(documents, memory.TextDocument{
			ID:        docID,
			Content:   text,
			Tags:      tagList,
			Timestamp: timestamp,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	// If we haven't found any direct text matches, try to find matches via facts
	if len(documents) == 0 {
		// Find facts that match the query
		factRows, err := m.db.QueryContext(ctx, `
			SELECT DISTINCT text_entry_id
			FROM facts
			WHERE sub ILIKE $1 OR prd ILIKE $1 OR obj ILIKE $1
			LIMIT 10
		`, "%"+query+"%")

		if err != nil {
			return nil, fmt.Errorf("error querying facts: %w", err)
		}
		defer func() {
			if closeErr := factRows.Close(); closeErr != nil {
				err = fmt.Errorf("error closing fact rows: %v (original error: %w)", closeErr, err)
			}
		}()

		// Collect text entry IDs
		var textEntryIDs []int64
		for factRows.Next() {
			var textEntryID int64
			if err := factRows.Scan(&textEntryID); err != nil {
				return nil, fmt.Errorf("error scanning fact row: %w", err)
			}
			textEntryIDs = append(textEntryIDs, textEntryID)
		}

		if err := factRows.Err(); err != nil {
			return nil, fmt.Errorf("error iterating fact rows: %w", err)
		}

		// If we found matching facts, look up the corresponding text entries
		if len(textEntryIDs) > 0 {
			// Create a parameterized IN clause like ($1, $2, $3, ...)
			params := make([]string, len(textEntryIDs))
			args := make([]interface{}, len(textEntryIDs))
			for i, id := range textEntryIDs {
				params[i] = fmt.Sprintf("$%d", i+1)
				args[i] = id
			}

			query := fmt.Sprintf(`
				SELECT id, text, meta, tags, created_at 
				FROM text_entries 
				WHERE id IN (%s)
				ORDER BY created_at DESC
			`, strings.Join(params, ", "))

			rows, err := m.db.QueryContext(ctx, query, args...)
			if err != nil {
				return nil, fmt.Errorf("error querying text entries by IDs: %w", err)
			}
			defer func() {
				if closeErr := rows.Close(); closeErr != nil {
					err = fmt.Errorf("error closing rows: %v (original error: %w)", closeErr, err)
				}
			}()

			// Process rows the same way as above
			for rows.Next() {
				var id int64
				var text string
				var meta sql.NullString
				var tags sql.NullString
				var createdAt time.Time

				if err := rows.Scan(&id, &text, &meta, &tags, &createdAt); err != nil {
					return nil, fmt.Errorf("error scanning row: %w", err)
				}

				// Same processing as above
				docID := fmt.Sprintf("doc-%d", id)
				metaMap := map[string]string{}

				if meta.Valid {
					metaStr := meta.String
					if metaStr != "" {
						entries := strings.Split(metaStr, ",")
						for _, entry := range entries {
							keyValue := strings.Split(entry, "=>")
							if len(keyValue) == 2 {
								key := strings.Trim(strings.TrimSpace(keyValue[0]), "\"")
								value := strings.Trim(strings.TrimSpace(keyValue[1]), "\"")
								metaMap[key] = value

								if key == "id" {
									docID = value
								}
							}
						}
					}
				}

				tagList := []string{}
				if tags.Valid && tags.String != "" {
					tagsStr := tags.String
					tagsStr = strings.Trim(tagsStr, "{}")
					if tagsStr != "" {
						tagItems := strings.Split(tagsStr, ",")
						for _, tag := range tagItems {
							tag = strings.Trim(strings.TrimSpace(tag), "\"")
							if tag != "" {
								tagList = append(tagList, tag)
							}
						}
					}
				}

				timestamp := &createdAt

				documents = append(documents, memory.TextDocument{
					ID:        docID,
					Content:   text,
					Tags:      tagList,
					Timestamp: timestamp,
				})
			}

			if err := rows.Err(); err != nil {
				return nil, fmt.Errorf("error iterating rows: %w", err)
			}
		}
	}

	return documents, nil
}

func (m *GraphMemory) ensureDbSchema(recreate bool) error {
	if _, err := m.db.Exec(sqlExtensions); err != nil {
		return err
	}

	if recreate {
		if _, err := m.db.Exec(sqlDropSchema); err != nil {
			return err
		}
	}

	if _, err := m.db.Exec(sqlSchema); err != nil {
		return err
	}

	return nil
}
