// Owner: dmitry@eternis.ai
package memory

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	// MaxProcessableContentChars represents the maximum number of characters
	// for any single piece of content before chunking. Based on Qwen-2.5-70b's
	// 128k token context window, targeting ~0.4x window size with 4-char/token
	// conservative ratio.
	MaxProcessableContentChars = 20000

	// Boolean operators for tag filtering.
	AND = "AND"
	OR  = "OR"
)

// BooleanExpression represents a complex boolean expression for tag filtering.
type BooleanExpression struct {
	Operator string             `json:"operator"`
	Tags     []string           `json:"tags,omitempty"`  // For leaf nodes
	Left     *BooleanExpression `json:"left,omitempty"`  // For AND/OR nodes
	Right    *BooleanExpression `json:"right,omitempty"` // For AND/OR nodes
}

// TagsFilter provides flexible tag filtering options supporting AND, OR, and complex boolean expressions.
type TagsFilter struct {
	All []string `json:"all,omitempty"` // Must contain ALL specified tags (AND logic)
	Any []string `json:"any,omitempty"` // Must contain ANY of the specified tags (OR logic)

	// Complex cases
	Expression *BooleanExpression `json:"expression,omitempty"` // Complex boolean expressions
}

// SimilarityFilter provides advanced semantic similarity filtering options.
type SimilarityFilter struct {
	MaxDistance     float32 `json:"max_distance"`               // Maximum semantic distance (0 = disabled)
	MinSimilarity   float32 `json:"min_similarity,omitempty"`   // Minimum similarity score (0-1)
	SimilarityBoost float32 `json:"similarity_boost,omitempty"` // Boost factor for similarity scoring
	Threshold       float32 `json:"threshold,omitempty"`        // Custom similarity threshold
}

// TemporalFilter provides advanced temporal filtering with relative time support.
type TemporalFilter struct {
	After   *time.Time `json:"after,omitempty"`   // Filter for facts created after this time (inclusive)
	Before  *time.Time `json:"before,omitempty"`  // Filter for facts created before this time (inclusive)
	Within  *string    `json:"within,omitempty"`  // Relative time like "24h", "7d", "30d"
	Exactly *time.Time `json:"exactly,omitempty"` // Exact timestamp match (useful for specific events)
}

// ContentFilter provides content-based filtering options.
type ContentFilter struct {
	Contains    []string `json:"contains,omitempty"`     // Content must contain all these terms
	ContainsAny []string `json:"contains_any,omitempty"` // Content must contain any of these terms
	Excludes    []string `json:"excludes,omitempty"`     // Content must not contain these terms
	MinLength   *int     `json:"min_length,omitempty"`   // Minimum content length
	MaxLength   *int     `json:"max_length,omitempty"`   // Maximum content length
	Regex       *string  `json:"regex,omitempty"`        // Regular expression pattern match
}

// NumericRangeFilter provides generic numeric range filtering.
type NumericRangeFilter struct {
	Min       *float64 `json:"min,omitempty"`       // Minimum value (inclusive)
	Max       *float64 `json:"max,omitempty"`       // Maximum value (inclusive)
	Exact     *float64 `json:"exact,omitempty"`     // Exact value match
	NotEquals *float64 `json:"not_equals,omitempty"` // Value must not equal this
}

// ConversationFilter provides filtering specific to ConversationDocument types.
type ConversationFilter struct {
	Participants   []string `json:"participants,omitempty"`    // Filter by conversation participants
	Speakers       []string `json:"speakers,omitempty"`        // Filter by specific speakers in messages
	HasSpeaker     *string  `json:"has_speaker,omitempty"`     // Must contain messages from this speaker
	ExcludeSpeaker *string  `json:"exclude_speaker,omitempty"` // Must not contain messages from this speaker
}

// DocumentFilter provides filtering specific to document-level properties.
type DocumentFilter struct {
	DocumentTypes   []string            `json:"document_types,omitempty"`   // Filter by document type (text, conversation)
	IDPattern       *string             `json:"id_pattern,omitempty"`       // Regex pattern for document IDs
	IsChunk         *bool               `json:"is_chunk,omitempty"`         // Filter for chunked vs original documents
	OriginalDocID   *string             `json:"original_doc_id,omitempty"`  // Filter by original document ID (for chunks)
	ChunkNumber     *NumericRangeFilter `json:"chunk_number,omitempty"`     // Filter by chunk number
	HasMetadataKey  *string             `json:"has_metadata_key,omitempty"` // Must have this metadata key
	MetadataFilters map[string]string   `json:"metadata_filters,omitempty"` // Key-value pairs that must match in metadata
	ContentHash     *string             `json:"content_hash,omitempty"`     // Filter by specific content hash
	OriginalIDs     []string            `json:"original_ids,omitempty"`     // Filter by multiple original IDs
}

// Filter provides structured filtering options for memory queries.
// DESIGN NOTE: Only indexed fields are included in this struct for optimal query performance.
// Some fields (factValue, factTemporalContext, factSensitivity) exist in the schema for
// storage and display purposes but are not indexed/filterable due to their rich text nature.
type Filter struct {
	Source   *string     // Filter by document source
	Subject  *string     // Filter by fact subject (user, entity names) - renamed from ContactName
	Tags     *TagsFilter // Filter by tags with boolean logic support
	Limit    *int        // Maximum number of results to return

	// Enhanced filtering capabilities
	Similarity   *SimilarityFilter   `json:"similarity,omitempty"`   // Advanced semantic similarity filtering
	Temporal     *TemporalFilter     `json:"temporal,omitempty"`     // Advanced temporal filtering
	Content      *ContentFilter      `json:"content,omitempty"`      // Content-based filtering
	Conversation *ConversationFilter `json:"conversation,omitempty"` // Conversation-specific filtering
	Document     *DocumentFilter     `json:"document,omitempty"`     // Document-level filtering

	// Structured fact filtering fields - ONLY indexed fields
	FactCategory   *string // Filter by fact category (profile_stable, preference, goal_plan, etc.)
	FactAttribute  *string // Filter by fact attribute (specific property being described)
	FactImportance *int    // Filter by importance score (1, 2, 3)

	// Enhanced numeric range filtering
	FactImportanceRange *NumericRangeFilter `json:"fact_importance_range,omitempty"` // Advanced importance filtering

	// Legacy fields (deprecated but maintained for backward compatibility)
	Distance            float32    `json:"distance,omitempty"`             // DEPRECATED: Use Similarity.MaxDistance instead
	FactImportanceMin   *int       `json:"fact_importance_min,omitempty"`  // DEPRECATED: Use FactImportanceRange.Min instead
	FactImportanceMax   *int       `json:"fact_importance_max,omitempty"`  // DEPRECATED: Use FactImportanceRange.Max instead
	TimestampAfter      *time.Time `json:"timestamp_after,omitempty"`      // DEPRECATED: Use Temporal.After instead
	TimestampBefore     *time.Time `json:"timestamp_before,omitempty"`     // DEPRECATED: Use Temporal.Before instead

	// Document references filtering
	DocumentReferences []string `json:"document_references,omitempty"` // Filter by document reference IDs

	// Sensitivity filtering
	SensitivityLevels []string `json:"sensitivity_levels,omitempty"` // Filter by sensitivity levels (high, medium, low)
}

// Document interface that both TextDocument and ConversationDocument implement.
type Document interface {
	ID() string
	Content() string
	Timestamp() *time.Time
	Tags() []string
	Metadata() map[string]string
	Source() string
	Chunk() []Document // New method for document chunking
}

// ConversationMessage represents a single message in a conversation.
type ConversationMessage struct {
	Speaker string    `json:"speaker"`
	Content string    `json:"content"`
	Time    time.Time `json:"time"`
}

// ConversationDocument represents a document containing structured conversation data.
type ConversationDocument struct {
	FieldID       string                `json:"id"`
	FieldSource   string                `json:"source"`
	People        []string              `json:"people"`
	User          string                `json:"user"`
	Conversation  []ConversationMessage `json:"conversation"`
	FieldTags     []string              `json:"tags,omitempty"`
	FieldMetadata map[string]string     `json:"metadata,omitempty"`
}

// Document interface implementation for ConversationDocument.
func (cd *ConversationDocument) ID() string {
	return cd.FieldID
}

func (cd *ConversationDocument) Content() string {
	var builder strings.Builder
	primaryUser := cd.User

	// Normalize people list
	normalizedPeople := make([]string, len(cd.People))
	for i, person := range cd.People {
		if person == primaryUser {
			normalizedPeople[i] = "primaryUser"
		} else {
			normalizedPeople[i] = person
		}
	}

	// CONVO header: CONVO|{id}|{source}
	builder.WriteString(fmt.Sprintf("CONVO|%s|%s\n", cd.FieldID, cd.FieldSource))

	// PEOPLE: PEOPLE|{user1}|{user2}|...
	builder.WriteString("PEOPLE|")
	builder.WriteString(strings.Join(normalizedPeople, "|"))
	builder.WriteString("\n")

	// PRIMARY: PRIMARY|primaryUser (always normalized)
	builder.WriteString("PRIMARY|primaryUser\n")

	// Separator
	builder.WriteString("|||\n")

	// Messages: {speaker}|||{time}|||{content}
	for _, msg := range cd.Conversation {
		trimmed := strings.TrimSpace(msg.Content)
		if trimmed == "" {
			continue
		}

		// Normalize speaker name
		normalizedSpeaker := msg.Speaker
		if msg.Speaker == primaryUser {
			normalizedSpeaker = "primaryUser"
		}

		timeStr := msg.Time.Format(time.RFC3339)
		builder.WriteString(fmt.Sprintf("%s|||%s|||%s\n", normalizedSpeaker, timeStr, trimmed))
	}

	// Final separator
	builder.WriteString("|||\n")

	// TAGS: TAGS|{tag1}|{tag2}|...
	builder.WriteString("TAGS|")
	builder.WriteString(strings.Join(cd.FieldTags, "|"))
	builder.WriteString("\n")

	return builder.String()
}

func (cd *ConversationDocument) Timestamp() *time.Time {
	if len(cd.Conversation) > 0 {
		return &cd.Conversation[0].Time
	}
	return nil
}

func (cd *ConversationDocument) Tags() []string {
	return cd.FieldTags
}

func (cd *ConversationDocument) Metadata() map[string]string {
	metadata := make(map[string]string)
	if cd.FieldMetadata != nil {
		for k, v := range cd.FieldMetadata {
			metadata[k] = v
		}
	}
	metadata["user"] = cd.User
	// Add people to metadata if needed, for now, it's directly accessible
	// metadata["people"] = strings.Join(cd.People, ", ")
	return metadata
}

func (cd *ConversationDocument) Source() string {
	return cd.FieldSource
}

// Chunk implements intelligent conversation chunking.
func (cd *ConversationDocument) Chunk() []Document {
	if cd == nil || len(cd.Conversation) == 0 {
		return []Document{cd}
	}

	if len(cd.Content()) < MaxProcessableContentChars {
		return []Document{cd}
	}

	var chunks []Document
	var currentChunkMessages []ConversationMessage
	currentCharCount := 0

	for _, msg := range cd.Conversation {
		msgContent := fmt.Sprintf("%s: %s\n", msg.Speaker, msg.Content)
		msgLen := len(msgContent)

		// Handle oversized individual messages
		if msgLen > MaxProcessableContentChars {
			// Finalize current chunk if it exists
			if len(currentChunkMessages) > 0 {
				chunk := cd.createConversationChunk(currentChunkMessages, len(chunks)+1)
				chunks = append(chunks, chunk)
				currentChunkMessages = nil
				currentCharCount = 0
			}

			// Split the oversized message
			splitMessages := cd.SplitOversizedMessage(msg)
			for _, splitMsg := range splitMessages {
				chunk := cd.createConversationChunk([]ConversationMessage{splitMsg}, len(chunks)+1)
				chunks = append(chunks, chunk)
			}
			continue
		}

		// Start new chunk if adding this message would exceed limit
		if currentCharCount+msgLen > MaxProcessableContentChars && len(currentChunkMessages) > 0 {
			chunk := cd.createConversationChunk(currentChunkMessages, len(chunks)+1)
			chunks = append(chunks, chunk)
			currentChunkMessages = nil
			currentCharCount = 0
		}

		currentChunkMessages = append(currentChunkMessages, msg)
		currentCharCount += msgLen
	}

	// Add final chunk
	if len(currentChunkMessages) > 0 {
		chunk := cd.createConversationChunk(currentChunkMessages, len(chunks)+1)
		chunks = append(chunks, chunk)
	}

	return chunks
}

// SplitOversizedMessage splits an oversized message into smaller chunks.
func (cd *ConversationDocument) SplitOversizedMessage(msg ConversationMessage) []ConversationMessage {
	speakerPrefix := fmt.Sprintf("%s: ", msg.Speaker)
	speakerPrefixLen := len(speakerPrefix)

	// Account for speaker prefix and newline in the content limit
	maxContentPerChunk := MaxProcessableContentChars - speakerPrefixLen - 1 // -1 for newline

	// Ensure we have at least some space for content
	if maxContentPerChunk < 100 {
		maxContentPerChunk = 100 // Minimum reasonable content size
	}

	content := msg.Content

	// Handle empty content
	if content == "" {
		return []ConversationMessage{msg}
	}

	var splitMessages []ConversationMessage
	partNumber := 1

	for len(content) > 0 {
		var chunkContent string

		if len(content) <= maxContentPerChunk {
			// Last chunk - take remaining content
			chunkContent = content
			content = ""
		} else {
			// Reserve space for potential markers
			continuationMarker := " [continued...]"
			partMarkerSpace := 20 // Space for "[Part X] " prefix
			availableSpace := maxContentPerChunk - len(continuationMarker) - partMarkerSpace

			// Ensure we have reasonable space
			if availableSpace < 50 {
				availableSpace = maxContentPerChunk - 10 // Minimal approach
			}

			// Find a good break point (prefer word boundaries)
			breakPoint := availableSpace
			if breakPoint > len(content) {
				breakPoint = len(content)
			}

			// Look backwards for a word boundary (space, newline, punctuation)
			for i := breakPoint - 1; i > availableSpace/2 && i < len(content); i-- {
				char := content[i]
				if char == ' ' || char == '\n' || char == '.' || char == '!' || char == '?' || char == ',' || char == ';' {
					breakPoint = i + 1 // Include the punctuation/space
					break
				}
			}

			chunkContent = content[:breakPoint]
			content = content[breakPoint:]

			// Add continuation indicator if this isn't the last part
			if len(content) > 0 {
				chunkContent += continuationMarker
			}
		}

		// Add part number for multi-part messages only if we have multiple parts
		willHaveMultipleParts := len(content) > 0 || partNumber > 1
		if willHaveMultipleParts {
			partPrefix := fmt.Sprintf("[Part %d] ", partNumber)

			// Ensure the total doesn't exceed limits
			totalLength := len(partPrefix) + len(chunkContent)
			if totalLength > maxContentPerChunk {
				// Trim content to fit with the part prefix
				trimAmount := totalLength - maxContentPerChunk
				if trimAmount < len(chunkContent) {
					chunkContent = chunkContent[:len(chunkContent)-trimAmount]
					// Remove partial words at the end
					lastSpace := strings.LastIndex(chunkContent, " ")
					if lastSpace > len(chunkContent)/2 {
						chunkContent = chunkContent[:lastSpace]
					}
				}
			}

			chunkContent = partPrefix + chunkContent
		}

		// Create a new message for this chunk
		splitMsg := ConversationMessage{
			Speaker: msg.Speaker,
			Content: chunkContent,
			Time:    msg.Time,
		}

		splitMessages = append(splitMessages, splitMsg)
		partNumber++

		// Safety check to prevent infinite loops
		if partNumber > 100 {
			break
		}
	}

	// If we only ended up with one message and it's not oversized, return original
	if len(splitMessages) == 1 {
		finalMsg := fmt.Sprintf("%s: %s\n", splitMessages[0].Speaker, splitMessages[0].Content)
		if len(finalMsg) <= MaxProcessableContentChars {
			return []ConversationMessage{msg} // Return original without markers
		}
	}

	return splitMessages
}

// createConversationChunk creates a new ConversationDocument chunk.
func (cd *ConversationDocument) createConversationChunk(messages []ConversationMessage, chunkNum int) *ConversationDocument {
	newID := fmt.Sprintf("%s-chunk-%d", cd.ID(), chunkNum)
	metadata := make(map[string]string)
	// Copy original metadata to the new chunk
	for k, v := range cd.Metadata() {
		metadata[k] = v
	}

	// Use namespaced keys to avoid collisions with existing metadata
	metadata["_enchanted_chunk_number"] = fmt.Sprintf("%d", chunkNum)
	metadata["_enchanted_original_document_id"] = cd.ID()
	metadata["_enchanted_chunk_type"] = "conversation"

	return &ConversationDocument{
		FieldID:       newID,
		FieldSource:   cd.Source(),
		People:        cd.People,
		User:          cd.User,
		Conversation:  messages,
		FieldTags:     cd.Tags(),
		FieldMetadata: metadata,
	}
}

// TextDocument represents a document format used internally by storage.
type TextDocument struct {
	FieldID        string            `json:"id"`
	FieldContent   string            `json:"content"`
	FieldTimestamp *time.Time        `json:"timestamp"`
	FieldSource    string            `json:"source,omitempty"`
	FieldTags      []string          `json:"tags,omitempty"`
	FieldMetadata  map[string]string `json:"metadata,omitempty"`
}

// Document interface implementation for TextDocument.
func (td *TextDocument) ID() string {
	return td.FieldID
}

func (td *TextDocument) Content() string {
	return td.FieldContent
}

func (td *TextDocument) Timestamp() *time.Time {
	return td.FieldTimestamp
}

func (td *TextDocument) Tags() []string {
	return td.FieldTags
}

func (td *TextDocument) Metadata() map[string]string {
	// Ensure metadata is not nil
	if td.FieldMetadata == nil {
		return make(map[string]string)
	}
	return td.FieldMetadata // Source is no longer guaranteed to be in metadata; use Source() method
}

func (td *TextDocument) Source() string {
	return td.FieldSource // Now returns the top-level field
}

// Chunk implements intelligent text document chunking (replaces truncation).
func (td *TextDocument) Chunk() []Document {
	if td == nil || td.Content() == "" {
		return []Document{td}
	}

	if len(td.Content()) <= MaxProcessableContentChars {
		return []Document{td}
	}

	// Chunk by paragraphs first, then by sentences if needed
	return td.chunkByParagraphs()
}

// chunkByParagraphs splits text content into chunks respecting paragraph boundaries.
func (td *TextDocument) chunkByParagraphs() []Document {
	content := td.Content()

	// Split by double newlines (paragraphs)
	paragraphs := strings.Split(content, "\n\n")

	var chunks []Document
	var currentChunk strings.Builder
	chunkNum := 1

	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}

		// Check if adding this paragraph would exceed the limit
		proposedLength := currentChunk.Len() + len(paragraph) + 2 // +2 for \n\n

		if proposedLength > MaxProcessableContentChars && currentChunk.Len() > 0 {
			// Create chunk from current content
			chunk := td.createTextChunk(currentChunk.String(), chunkNum)
			chunks = append(chunks, chunk)
			chunkNum++
			currentChunk.Reset()
		}

		// If single paragraph is too large, split it by sentences
		if len(paragraph) > MaxProcessableContentChars {
			// Finalize current chunk if it has content
			if currentChunk.Len() > 0 {
				chunk := td.createTextChunk(currentChunk.String(), chunkNum)
				chunks = append(chunks, chunk)
				chunkNum++
				currentChunk.Reset()
			}

			// Split oversized paragraph by sentences
			sentenceChunks := td.chunkBySentences(paragraph, &chunkNum)
			chunks = append(chunks, sentenceChunks...)
			continue
		}

		// Add paragraph to current chunk
		if currentChunk.Len() > 0 {
			currentChunk.WriteString("\n\n")
		}
		currentChunk.WriteString(paragraph)
	}

	// Add final chunk if there's remaining content
	if currentChunk.Len() > 0 {
		chunk := td.createTextChunk(currentChunk.String(), chunkNum)
		chunks = append(chunks, chunk)
	}

	// If no chunks were created, return original document
	if len(chunks) == 0 {
		return []Document{td}
	}

	return chunks
}

// chunkBySentences splits a large paragraph into sentence-based chunks.
func (td *TextDocument) chunkBySentences(paragraph string, chunkNum *int) []Document {
	// Split by sentence endings
	sentences := strings.FieldsFunc(paragraph, func(r rune) bool {
		return r == '.' || r == '!' || r == '?'
	})

	var chunks []Document
	var currentChunk strings.Builder

	for i, sentence := range sentences {
		sentence = strings.TrimSpace(sentence)
		if sentence == "" {
			continue
		}

		// Add back the punctuation (except for the last one if it was split)
		if i < len(sentences)-1 {
			if strings.Contains(paragraph, sentence+".") {
				sentence += "."
			} else if strings.Contains(paragraph, sentence+"!") {
				sentence += "!"
			} else if strings.Contains(paragraph, sentence+"?") {
				sentence += "?"
			}
		}

		// Check if adding this sentence would exceed the limit
		proposedLength := currentChunk.Len() + len(sentence) + 1 // +1 for space

		if proposedLength > MaxProcessableContentChars && currentChunk.Len() > 0 {
			chunk := td.createTextChunk(currentChunk.String(), *chunkNum)
			chunks = append(chunks, chunk)
			(*chunkNum)++
			currentChunk.Reset()
		}

		// If single sentence is still too large, split by character limit as last resort
		if len(sentence) > MaxProcessableContentChars {
			if currentChunk.Len() > 0 {
				chunk := td.createTextChunk(currentChunk.String(), *chunkNum)
				chunks = append(chunks, chunk)
				(*chunkNum)++
				currentChunk.Reset()
			}

			charChunks := td.chunkByCharacterLimit(sentence, chunkNum)
			chunks = append(chunks, charChunks...)
			continue
		}

		// Add sentence to current chunk
		if currentChunk.Len() > 0 {
			currentChunk.WriteString(" ")
		}
		currentChunk.WriteString(sentence)
	}

	// Add final chunk if there's remaining content
	if currentChunk.Len() > 0 {
		chunk := td.createTextChunk(currentChunk.String(), *chunkNum)
		chunks = append(chunks, chunk)
		(*chunkNum)++
	}

	return chunks
}

// chunkByCharacterLimit splits text by character limit with word boundary awareness.
func (td *TextDocument) chunkByCharacterLimit(text string, chunkNum *int) []Document {
	var chunks []Document

	for len(text) > 0 {
		chunkSize := MaxProcessableContentChars
		if len(text) <= chunkSize {
			// Last chunk
			chunk := td.createTextChunk(text, *chunkNum)
			chunks = append(chunks, chunk)
			(*chunkNum)++
			break
		}

		// Find a word boundary
		breakPoint := chunkSize
		for i := chunkSize - 1; i > chunkSize/2 && i < len(text); i-- {
			if text[i] == ' ' || text[i] == '\n' || text[i] == '\t' {
				breakPoint = i
				break
			}
		}

		chunkContent := strings.TrimSpace(text[:breakPoint])
		chunk := td.createTextChunk(chunkContent, *chunkNum)
		chunks = append(chunks, chunk)
		(*chunkNum)++

		text = strings.TrimSpace(text[breakPoint:])
	}

	return chunks
}

// createTextChunk creates a new TextDocument chunk.
func (td *TextDocument) createTextChunk(content string, chunkNum int) *TextDocument {
	newID := fmt.Sprintf("%s-chunk-%d", td.ID(), chunkNum)
	metadata := make(map[string]string)

	// Copy original metadata
	for k, v := range td.Metadata() {
		metadata[k] = v
	}

	// Use namespaced keys to avoid collisions with existing metadata
	metadata["_enchanted_chunk_number"] = fmt.Sprintf("%d", chunkNum)
	metadata["_enchanted_original_document_id"] = td.ID()
	metadata["_enchanted_chunk_type"] = "text"

	return &TextDocument{
		FieldID:        newID,
		FieldContent:   content,
		FieldTimestamp: td.FieldTimestamp,
		FieldSource:    td.FieldSource,
		FieldTags:      td.FieldTags,
		FieldMetadata:  metadata,
	}
}

// MemoryFact represents an extracted fact about a person with structured fields.
type MemoryFact struct {
	// Identity and display
	ID        string    `json:"id"`
	Content   string    `json:"content"` // Searchable content (generated from structured fields)
	Timestamp time.Time `json:"timestamp"`

	// Structured fact fields
	Category        string  `json:"category"`         // Semantic category (preference, health, goal_plan, etc.)
	Subject         string  `json:"subject"`          // Who/what the fact is about
	Attribute       string  `json:"attribute"`        // Property being described
	Value           string  `json:"value"`            // The actual fact content
	TemporalContext *string `json:"temporal_context"` // When this happened (optional)
	Sensitivity     string  `json:"sensitivity"`      // Privacy level: high/medium/low
	Importance      int     `json:"importance"`       // Priority score: 1-3

	// Source tracking
	Source             string   `json:"source"`              // Source of the memory document
	DocumentReferences []string `json:"document_references"` // IDs of source documents
	Tags               []string `json:"tags,omitempty"`      // Tags for categorization

	// Legacy support
	Metadata map[string]string `json:"metadata,omitempty"` // Additional metadata (being phased out)
}

type QueryResult struct {
	Facts []MemoryFact `json:"facts"`
}

// StoredDocument represents a document as stored in the storage system.
type StoredDocument struct {
	ID           string            `json:"id"`
	Content      string            `json:"content"`
	ContentHash  string            `json:"content_hash"`
	DocumentType string            `json:"document_type"` // "text" or "conversation"
	OriginalID   string            `json:"original_id"`
	Metadata     map[string]string `json:"metadata"`
	CreatedAt    *time.Time        `json:"created_at"`
	
	// Additional computed fields
	IsChunk       bool   `json:"is_chunk"`        // Whether this is a document chunk
	ChunkNumber   *int   `json:"chunk_number"`    // Chunk sequence number (if chunk)
	OriginalDocID *string `json:"original_doc_id"` // Original document ID (if chunk)
}

// DocumentQueryResult represents the result of a document query.
type DocumentQueryResult struct {
	Documents []StoredDocument `json:"documents"`
	Total     int              `json:"total"`      // Total documents matching (before limit)
	HasMore   bool             `json:"has_more"`   // Whether there are more results
}

// ProgressUpdate represents progress information for memory storage operations.
type ProgressUpdate struct {
	Processed int    `json:"processed"`
	Total     int    `json:"total"`
	Stage     string `json:"stage,omitempty"`
}

// ProgressCallback is the standard callback function for tracking storage progress.
type ProgressCallback func(processed, total int)

// Storage defines the interface for memory storage operations.
type Storage interface {
	Store(ctx context.Context, documents []Document, progressCallback ProgressCallback) error
	Query(ctx context.Context, query string, filter *Filter) (QueryResult, error)
	QueryDocuments(ctx context.Context, filter *Filter) (DocumentQueryResult, error)
}

// IndexableFields defines which fields should be indexed for efficient filtering.
// This helps storage implementations optimize their schema and query performance.
type IndexableFields struct {
	// Document-level indexed fields
	DocumentID       bool `json:"document_id"`        // Document.ID()
	DocumentSource   bool `json:"document_source"`    // Document.Source()
	DocumentTags     bool `json:"document_tags"`      // Document.Tags() - array field
	DocumentType     bool `json:"document_type"`      // text/conversation
	IsChunk          bool `json:"is_chunk"`           // Whether document is a chunk
	OriginalDocID    bool `json:"original_doc_id"`    // For chunks, the original document ID
	ChunkNumber      bool `json:"chunk_number"`       // Chunk sequence number
	
	// Conversation-specific indexed fields  
	ConversationParticipants []string `json:"conversation_participants"` // All participants in conversation
	ConversationSpeakers     []string `json:"conversation_speakers"`     // All speakers with messages
	
	// Fact-level indexed fields (existing)
	FactCategory    bool `json:"fact_category"`     // MemoryFact.Category
	FactSubject     bool `json:"fact_subject"`      // MemoryFact.Subject  
	FactAttribute   bool `json:"fact_attribute"`    // MemoryFact.Attribute
	FactImportance  bool `json:"fact_importance"`   // MemoryFact.Importance
	FactSensitivity bool `json:"fact_sensitivity"`  // MemoryFact.Sensitivity
	FactTimestamp   bool `json:"fact_timestamp"`    // MemoryFact.Timestamp
	
	// Metadata fields (key-based indexing)
	IndexedMetadataKeys []string `json:"indexed_metadata_keys"` // Which metadata keys to index
}

// FilterOptimizationHint provides hints to storage implementations for query optimization.
type FilterOptimizationHint struct {
	// Suggest which filters to apply first for performance
	PrimaryFilters   []string `json:"primary_filters,omitempty"`   // Apply these first (most selective)
	SecondaryFilters []string `json:"secondary_filters,omitempty"` // Apply these after vector search
	
	// Vector search optimization
	PrefilterVector  bool `json:"prefilter_vector,omitempty"`  // Apply filters before vector search
	PostfilterVector bool `json:"postfilter_vector,omitempty"` // Apply filters after vector search
	
	// Expected result size for optimization
	ExpectedResultSize *int `json:"expected_result_size,omitempty"` // Hint for result set size
}

// IsEmpty returns true if the TagsFilter has no filtering criteria.
func (tf *TagsFilter) IsEmpty() bool {
	if tf == nil {
		return true
	}
	return len(tf.All) == 0 && len(tf.Any) == 0 && tf.Expression == nil
}

// IsEmpty returns true if the SimilarityFilter has no filtering criteria.
func (sf *SimilarityFilter) IsEmpty() bool {
	if sf == nil {
		return true
	}
	return sf.MaxDistance == 0 && sf.MinSimilarity == 0 && sf.SimilarityBoost == 0 && sf.Threshold == 0
}

// IsEmpty returns true if the TemporalFilter has no filtering criteria.
func (tf *TemporalFilter) IsEmpty() bool {
	if tf == nil {
		return true
	}
	return tf.After == nil && tf.Before == nil && tf.Within == nil && tf.Exactly == nil
}

// IsEmpty returns true if the ContentFilter has no filtering criteria.
func (cf *ContentFilter) IsEmpty() bool {
	if cf == nil {
		return true
	}
	return len(cf.Contains) == 0 && len(cf.ContainsAny) == 0 && len(cf.Excludes) == 0 &&
		cf.MinLength == nil && cf.MaxLength == nil && cf.Regex == nil
}

// IsEmpty returns true if the NumericRangeFilter has no filtering criteria.
func (nrf *NumericRangeFilter) IsEmpty() bool {
	if nrf == nil {
		return true
	}
	return nrf.Min == nil && nrf.Max == nil && nrf.Exact == nil && nrf.NotEquals == nil
}

// IsEmpty returns true if the ConversationFilter has no filtering criteria.
func (cf *ConversationFilter) IsEmpty() bool {
	if cf == nil {
		return true
	}
	return len(cf.Participants) == 0 && len(cf.Speakers) == 0 &&
		cf.HasSpeaker == nil && cf.ExcludeSpeaker == nil
}

// IsEmpty returns true if the DocumentFilter has no filtering criteria.
func (df *DocumentFilter) IsEmpty() bool {
	if df == nil {
		return true
	}
	return len(df.DocumentTypes) == 0 && df.IDPattern == nil && df.IsChunk == nil &&
		df.OriginalDocID == nil && df.ChunkNumber.IsEmpty() && df.HasMetadataKey == nil &&
		len(df.MetadataFilters) == 0 && df.ContentHash == nil && len(df.OriginalIDs) == 0
}

// Validate ensures the SimilarityFilter has valid values.
func (sf *SimilarityFilter) Validate() error {
	if sf == nil {
		return nil
	}
	if sf.MaxDistance < 0 {
		return fmt.Errorf("max_distance must be non-negative, got %f", sf.MaxDistance)
	}
	if sf.MinSimilarity < 0 || sf.MinSimilarity > 1 {
		return fmt.Errorf("min_similarity must be between 0 and 1, got %f", sf.MinSimilarity)
	}
	if sf.Threshold < 0 || sf.Threshold > 1 {
		return fmt.Errorf("threshold must be between 0 and 1, got %f", sf.Threshold)
	}
	return nil
}

// Validate ensures the TemporalFilter has valid values.
func (tf *TemporalFilter) Validate() error {
	if tf == nil {
		return nil
	}
	if tf.After != nil && tf.Before != nil && tf.After.After(*tf.Before) {
		return fmt.Errorf("after timestamp must be before before timestamp")
	}
	if tf.Within != nil {
		if _, err := time.ParseDuration(*tf.Within); err != nil {
			return fmt.Errorf("invalid within duration format: %s", *tf.Within)
		}
	}
	return nil
}

// Validate ensures the ContentFilter has valid values.
func (cf *ContentFilter) Validate() error {
	if cf == nil {
		return nil
	}
	if cf.MinLength != nil && *cf.MinLength < 0 {
		return fmt.Errorf("min_length must be non-negative, got %d", *cf.MinLength)
	}
	if cf.MaxLength != nil && *cf.MaxLength < 0 {
		return fmt.Errorf("max_length must be non-negative, got %d", *cf.MaxLength)
	}
	if cf.MinLength != nil && cf.MaxLength != nil && *cf.MinLength > *cf.MaxLength {
		return fmt.Errorf("min_length cannot be greater than max_length")
	}
	return nil
}

// Validate ensures the NumericRangeFilter has valid values.
func (nrf *NumericRangeFilter) Validate() error {
	if nrf == nil {
		return nil
	}
	if nrf.Min != nil && nrf.Max != nil && *nrf.Min > *nrf.Max {
		return fmt.Errorf("min value cannot be greater than max value")
	}
	return nil
}

// Validate ensures the ConversationFilter has valid values.
func (cf *ConversationFilter) Validate() error {
	if cf == nil {
		return nil
	}
	return nil
}

// Validate ensures the DocumentFilter has valid values.
func (df *DocumentFilter) Validate() error {
	if df == nil {
		return nil
	}
	if err := df.ChunkNumber.Validate(); err != nil {
		return fmt.Errorf("chunk number validation failed: %w", err)
	}
	
	// Validate document types
	validDocTypes := map[string]bool{"text": true, "conversation": true}
	for _, docType := range df.DocumentTypes {
		if !validDocTypes[docType] {
			return fmt.Errorf("invalid document type: %s, must be one of: text, conversation", docType)
		}
	}
	
	// Validate chunk number constraints
	if df.ChunkNumber != nil {
		if df.ChunkNumber.Min != nil && *df.ChunkNumber.Min < 1 {
			return fmt.Errorf("chunk number min must be at least 1")
		}
		if df.ChunkNumber.Max != nil && *df.ChunkNumber.Max < 1 {
			return fmt.Errorf("chunk number max must be at least 1")
		}
	}
	
	return nil
}

// Validate ensures the Filter has valid values and handles backward compatibility.
func (f *Filter) Validate() error {
	if f == nil {
		return nil
	}

	// Validate sub-filters
	if err := f.Similarity.Validate(); err != nil {
		return fmt.Errorf("similarity filter validation failed: %w", err)
	}
	if err := f.Temporal.Validate(); err != nil {
		return fmt.Errorf("temporal filter validation failed: %w", err)
	}
	if err := f.Content.Validate(); err != nil {
		return fmt.Errorf("content filter validation failed: %w", err)
	}
	if err := f.Conversation.Validate(); err != nil {
		return fmt.Errorf("conversation filter validation failed: %w", err)
	}
	if err := f.Document.Validate(); err != nil {
		return fmt.Errorf("document filter validation failed: %w", err)
	}
	if err := f.FactImportanceRange.Validate(); err != nil {
		return fmt.Errorf("fact importance range filter validation failed: %w", err)
	}

	// Validate legacy fields
	if f.Distance < 0 {
		return fmt.Errorf("distance must be non-negative, got %f", f.Distance)
	}
	if f.FactImportanceMin != nil && *f.FactImportanceMin < 1 {
		return fmt.Errorf("fact_importance_min must be at least 1, got %d", *f.FactImportanceMin)
	}
	if f.FactImportanceMax != nil && *f.FactImportanceMax > 3 {
		return fmt.Errorf("fact_importance_max must be at most 3, got %d", *f.FactImportanceMax)
	}
	if f.FactImportanceMin != nil && f.FactImportanceMax != nil && *f.FactImportanceMin > *f.FactImportanceMax {
		return fmt.Errorf("fact_importance_min cannot be greater than fact_importance_max")
	}

	// Validate limit
	if f.Limit != nil && *f.Limit <= 0 {
		return fmt.Errorf("limit must be positive, got %d", *f.Limit)
	}

	// Validate sensitivity levels
	validSensitivityLevels := map[string]bool{"high": true, "medium": true, "low": true}
	for _, level := range f.SensitivityLevels {
		if !validSensitivityLevels[level] {
			return fmt.Errorf("invalid sensitivity level: %s, must be one of: high, medium, low", level)
		}
	}

	return nil
}

// MigrateFromLegacy converts legacy filter fields to new structured filters for backward compatibility.
func (f *Filter) MigrateFromLegacy() {
	if f == nil {
		return
	}

	// Migrate distance to similarity filter
	if f.Distance > 0 && f.Similarity == nil {
		f.Similarity = &SimilarityFilter{
			MaxDistance: f.Distance,
		}
	}

	// Migrate timestamp filtering to temporal filter
	if (f.TimestampAfter != nil || f.TimestampBefore != nil) && f.Temporal == nil {
		f.Temporal = &TemporalFilter{
			After:  f.TimestampAfter,
			Before: f.TimestampBefore,
		}
	}

	// Migrate importance range to numeric range filter
	if (f.FactImportanceMin != nil || f.FactImportanceMax != nil) && f.FactImportanceRange == nil {
		f.FactImportanceRange = &NumericRangeFilter{}
		if f.FactImportanceMin != nil {
			min := float64(*f.FactImportanceMin)
			f.FactImportanceRange.Min = &min
		}
		if f.FactImportanceMax != nil {
			max := float64(*f.FactImportanceMax)
			f.FactImportanceRange.Max = &max
		}
	}
}

// GetOptimizationHint analyzes the filter and provides optimization hints for storage implementations.
func (f *Filter) GetOptimizationHint() *FilterOptimizationHint {
	if f == nil {
		return &FilterOptimizationHint{}
	}

	hint := &FilterOptimizationHint{
		PrimaryFilters:   make([]string, 0),
		SecondaryFilters: make([]string, 0),
	}

	// Highly selective filters (apply first)
	if f.Source != nil {
		hint.PrimaryFilters = append(hint.PrimaryFilters, "source")
	}
	if f.Subject != nil {
		hint.PrimaryFilters = append(hint.PrimaryFilters, "subject")
	}
	if f.FactCategory != nil {
		hint.PrimaryFilters = append(hint.PrimaryFilters, "fact_category")
	}
	if f.Document != nil && !f.Document.IsEmpty() {
		if f.Document.IsChunk != nil {
			hint.PrimaryFilters = append(hint.PrimaryFilters, "is_chunk")
		}
		if f.Document.OriginalDocID != nil {
			hint.PrimaryFilters = append(hint.PrimaryFilters, "original_doc_id")
		}
		if len(f.Document.DocumentTypes) > 0 {
			hint.PrimaryFilters = append(hint.PrimaryFilters, "document_type")
		}
	}

	// Medium selectivity filters
	if !f.Tags.IsEmpty() {
		hint.SecondaryFilters = append(hint.SecondaryFilters, "tags")
	}
	if f.Conversation != nil && !f.Conversation.IsEmpty() {
		hint.SecondaryFilters = append(hint.SecondaryFilters, "conversation")
	}
	if f.Temporal != nil && !f.Temporal.IsEmpty() {
		hint.SecondaryFilters = append(hint.SecondaryFilters, "temporal")
	}

	// Content-based filters (apply after vector search)
	if f.Content != nil && !f.Content.IsEmpty() {
		hint.SecondaryFilters = append(hint.SecondaryFilters, "content")
		hint.PostfilterVector = true
	}

	// Similarity filters suggest vector search
	if f.Similarity != nil && !f.Similarity.IsEmpty() {
		hint.PrefilterVector = len(hint.PrimaryFilters) > 0
	}

	// Estimate result size based on filter selectivity
	selectivity := len(hint.PrimaryFilters)*3 + len(hint.SecondaryFilters)
	if selectivity > 6 {
		resultSize := 100 // Highly selective
		hint.ExpectedResultSize = &resultSize
	} else if selectivity > 3 {
		resultSize := 500 // Medium selective
		hint.ExpectedResultSize = &resultSize
	} else if selectivity > 0 {
		resultSize := 1000 // Low selective
		hint.ExpectedResultSize = &resultSize
	}

	return hint
}

// GetRequiredIndexes returns the fields that need to be indexed for this filter to work efficiently.
func (f *Filter) GetRequiredIndexes() *IndexableFields {
	if f == nil {
		return &IndexableFields{}
	}

	indexes := &IndexableFields{
		IndexedMetadataKeys: make([]string, 0),
	}

	// Always index basic document fields
	indexes.DocumentID = true
	indexes.DocumentSource = f.Source != nil
	indexes.DocumentTags = !f.Tags.IsEmpty()

	// Fact-level indexes
	indexes.FactCategory = f.FactCategory != nil
	indexes.FactSubject = f.Subject != nil
	indexes.FactAttribute = f.FactAttribute != nil
	indexes.FactImportance = f.FactImportance != nil || f.FactImportanceRange != nil && !f.FactImportanceRange.IsEmpty()
	indexes.FactTimestamp = f.Temporal != nil && !f.Temporal.IsEmpty()
	indexes.FactSensitivity = len(f.SensitivityLevels) > 0

	// Document-level indexes
	if f.Document != nil && !f.Document.IsEmpty() {
		indexes.DocumentType = len(f.Document.DocumentTypes) > 0
		indexes.IsChunk = f.Document.IsChunk != nil
		indexes.OriginalDocID = f.Document.OriginalDocID != nil
		indexes.ChunkNumber = f.Document.ChunkNumber != nil && !f.Document.ChunkNumber.IsEmpty()
		
		// Metadata indexes
		if f.Document.HasMetadataKey != nil {
			indexes.IndexedMetadataKeys = append(indexes.IndexedMetadataKeys, *f.Document.HasMetadataKey)
		}
		for key := range f.Document.MetadataFilters {
			indexes.IndexedMetadataKeys = append(indexes.IndexedMetadataKeys, key)
		}
	}

	// Conversation indexes
	if f.Conversation != nil && !f.Conversation.IsEmpty() {
		if len(f.Conversation.Participants) > 0 {
			indexes.ConversationParticipants = f.Conversation.Participants
		}
		if len(f.Conversation.Speakers) > 0 {
			indexes.ConversationSpeakers = f.Conversation.Speakers
		}
	}

	return indexes
}

// AnalyzeFilterComplexity returns a complexity score and suggestions for filter optimization.
func (f *Filter) AnalyzeFilterComplexity() (complexity int, suggestions []string) {
	if f == nil {
		return 0, nil
	}

	suggestions = make([]string, 0)

	// Count active filters
	if f.Source != nil {
		complexity++
	}
	if f.Subject != nil {
		complexity++
	}
	if !f.Tags.IsEmpty() {
		complexity += 2 // Tag filtering can be complex
		if f.Tags.Expression != nil {
			complexity += 2 // Boolean expressions add complexity
			suggestions = append(suggestions, "Consider simplifying tag boolean expressions for better performance")
		}
	}
	if f.Similarity != nil && !f.Similarity.IsEmpty() {
		complexity += 3 // Vector search is expensive
	}
	if f.Temporal != nil && !f.Temporal.IsEmpty() {
		complexity++
		if f.Temporal.Within != nil {
			suggestions = append(suggestions, "Relative time filters require runtime calculation")
		}
	}
	if f.Content != nil && !f.Content.IsEmpty() {
		complexity += 4 // Text processing is expensive
		if f.Content.Regex != nil {
			complexity += 2
			suggestions = append(suggestions, "Regex patterns can be slow; consider simpler text matching")
		}
	}
	if f.Conversation != nil && !f.Conversation.IsEmpty() {
		complexity += 2
	}
	if f.Document != nil && !f.Document.IsEmpty() {
		complexity++
		if f.Document.IDPattern != nil {
			complexity++
			suggestions = append(suggestions, "ID pattern matching should use indexed prefix patterns when possible")
		}
	}

	// Add suggestions based on complexity
	if complexity > 8 {
		suggestions = append(suggestions, "High filter complexity detected; consider breaking into multiple queries")
	}
	if complexity > 5 && (f.Content != nil && !f.Content.IsEmpty()) {
		suggestions = append(suggestions, "Consider applying content filters after other filters to reduce processing")
	}

	return complexity, suggestions
}

// ToDocument converts a StoredDocument back to a Document interface implementation.
func (sd *StoredDocument) ToDocument() Document {
	if sd.DocumentType == "conversation" {
		return sd.toConversationDocument()
	}
	return sd.toTextDocument()
}

// toTextDocument converts StoredDocument to TextDocument.
func (sd *StoredDocument) toTextDocument() *TextDocument {
	return &TextDocument{
		FieldID:        sd.OriginalID,
		FieldContent:   sd.Content,
		FieldTimestamp: sd.CreatedAt,
		FieldSource:    sd.Metadata["source"],
		FieldTags:      parseTagsFromMetadata(sd.Metadata),
		FieldMetadata:  sd.Metadata,
	}
}

// toConversationDocument converts StoredDocument to ConversationDocument.
func (sd *StoredDocument) toConversationDocument() *ConversationDocument {
	// Parse conversation content format back to structured data
	// This would need to reverse the Content() formatting from ConversationDocument
	// For now, return a basic structure - full implementation would parse the content
	return &ConversationDocument{
		FieldID:       sd.OriginalID,
		FieldSource:   sd.Metadata["source"],
		People:        parseParticipantsFromMetadata(sd.Metadata),
		User:          sd.Metadata["user"],
		Conversation:  parseMessagesFromContent(sd.Content),
		FieldTags:     parseTagsFromMetadata(sd.Metadata),
		FieldMetadata: sd.Metadata,
	}
}

// parseTagsFromMetadata extracts tags from metadata.
func parseTagsFromMetadata(metadata map[string]string) []string {
	if tagsStr, exists := metadata["tags"]; exists {
		if tagsStr != "" {
			return strings.Split(tagsStr, "|")
		}
	}
	return []string{}
}

// parseParticipantsFromMetadata extracts participants from metadata.
func parseParticipantsFromMetadata(metadata map[string]string) []string {
	if participantsStr, exists := metadata["participants"]; exists {
		if participantsStr != "" {
			return strings.Split(participantsStr, "|")
		}
	}
	return []string{}
}

// parseMessagesFromContent parses conversation content back to messages.
// This is a simplified version - full implementation would properly parse the content format.
func parseMessagesFromContent(_ string) []ConversationMessage {
	// TODO: Implement proper parsing of conversation content format
	// For now, return empty slice - full implementation would parse:
	// {speaker}|||{time}|||{content} format
	return []ConversationMessage{}
}

// IsDocumentChunk checks if the stored document represents a chunk.
func (sd *StoredDocument) IsDocumentChunk() bool {
	return sd.IsChunk || strings.Contains(sd.ID, "-chunk-")
}

// GetChunkInfo extracts chunk information from the stored document.
func (sd *StoredDocument) GetChunkInfo() (isChunk bool, chunkNumber int, originalID string) {
	if sd.ChunkNumber != nil {
		return true, *sd.ChunkNumber, *sd.OriginalDocID
	}
	
	// Fallback: parse from ID if chunk metadata isn't available
	if strings.Contains(sd.ID, "-chunk-") {
		// Parse chunk info from ID pattern: {originalID}-chunk-{number}
		parts := strings.Split(sd.ID, "-chunk-")
		if len(parts) == 2 {
			if num, err := fmt.Sscanf(parts[1], "%d", &chunkNumber); err == nil && num == 1 {
				return true, chunkNumber, parts[0]
			}
		}
	}
	
	return false, 0, sd.OriginalID
}

// DocumentQueryBuilder provides a fluent interface for building document queries.
type DocumentQueryBuilder struct {
	filter *Filter
}

// NewDocumentQuery creates a new document query builder.
func NewDocumentQuery() *DocumentQueryBuilder {
	return &DocumentQueryBuilder{
		filter: &Filter{
			Document: &DocumentFilter{},
		},
	}
}

// WithDocumentTypes filters by document types.
func (dqb *DocumentQueryBuilder) WithDocumentTypes(types ...string) *DocumentQueryBuilder {
	dqb.filter.Document.DocumentTypes = types
	return dqb
}

// WithOriginalID filters by original document ID.
func (dqb *DocumentQueryBuilder) WithOriginalID(id string) *DocumentQueryBuilder {
	dqb.filter.Document.OriginalDocID = &id
	return dqb
}

// WithOriginalIDs filters by multiple original document IDs.
func (dqb *DocumentQueryBuilder) WithOriginalIDs(ids ...string) *DocumentQueryBuilder {
	dqb.filter.Document.OriginalIDs = ids
	return dqb
}

// ChunksOnly filters for document chunks only.
func (dqb *DocumentQueryBuilder) ChunksOnly() *DocumentQueryBuilder {
	isChunk := true
	dqb.filter.Document.IsChunk = &isChunk
	return dqb
}

// OriginalsOnly filters for original documents only (no chunks).
func (dqb *DocumentQueryBuilder) OriginalsOnly() *DocumentQueryBuilder {
	isChunk := false
	dqb.filter.Document.IsChunk = &isChunk
	return dqb
}

// WithContentHash filters by content hash.
func (dqb *DocumentQueryBuilder) WithContentHash(hash string) *DocumentQueryBuilder {
	dqb.filter.Document.ContentHash = &hash
	return dqb
}

// WithMetadata adds metadata key-value filters.
func (dqb *DocumentQueryBuilder) WithMetadata(key, value string) *DocumentQueryBuilder {
	if dqb.filter.Document.MetadataFilters == nil {
		dqb.filter.Document.MetadataFilters = make(map[string]string)
	}
	dqb.filter.Document.MetadataFilters[key] = value
	return dqb
}

// WithSource filters by document source.
func (dqb *DocumentQueryBuilder) WithSource(source string) *DocumentQueryBuilder {
	dqb.filter.Source = &source
	return dqb
}

// WithTags filters by tags.
func (dqb *DocumentQueryBuilder) WithTags(tags ...string) *DocumentQueryBuilder {
	if dqb.filter.Tags == nil {
		dqb.filter.Tags = &TagsFilter{}
	}
	dqb.filter.Tags.All = tags
	return dqb
}

// WithLimit sets the maximum number of results.
func (dqb *DocumentQueryBuilder) WithLimit(limit int) *DocumentQueryBuilder {
	dqb.filter.Limit = &limit
	return dqb
}

// WithTimeRange filters by creation time range.
func (dqb *DocumentQueryBuilder) WithTimeRange(after, before *time.Time) *DocumentQueryBuilder {
	if dqb.filter.Temporal == nil {
		dqb.filter.Temporal = &TemporalFilter{}
	}
	dqb.filter.Temporal.After = after
	dqb.filter.Temporal.Before = before
	return dqb
}

// Build returns the constructed filter.
func (dqb *DocumentQueryBuilder) Build() *Filter {
	return dqb.filter
}

// GenerateContent creates the searchable content string from structured fields.
func (mf *MemoryFact) GenerateContent() string {
	// Simple combination for embeddings and search
	return fmt.Sprintf("%s - %s", mf.Subject, mf.Value)
}

// IsLeaf returns true if this is a leaf node (has tags).
func (be *BooleanExpression) IsLeaf() bool {
	return be != nil && len(be.Tags) > 0
}

// IsBranch returns true if this is a branch node (has left/right operands).
func (be *BooleanExpression) IsBranch() bool {
	return be != nil && be.Left != nil && be.Right != nil
}
