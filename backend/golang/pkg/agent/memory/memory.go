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
	MaxProcessableContentChars = 50000

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
	// Simple cases (backward compatible)
	All []string `json:"all,omitempty"` // Must contain ALL specified tags (AND logic)
	Any []string `json:"any,omitempty"` // Must contain ANY of the specified tags (OR logic)

	// Complex cases
	Expression *BooleanExpression `json:"expression,omitempty"` // Complex boolean expressions
}

// Filter provides structured filtering options for memory queries.
// DESIGN NOTE: Only indexed fields are included in this struct for optimal query performance.
// Some fields (factValue, factTemporalContext, factSensitivity) exist in the schema for
// storage and display purposes but are not indexed/filterable due to their rich text nature.
type Filter struct {
	Source   *string     // Filter by document source
	Subject  *string     // Filter by fact subject (user, entity names) - renamed from ContactName
	Tags     *TagsFilter // Filter by tags with boolean logic support
	Distance float32     // Maximum semantic distance (0 = disabled)
	Limit    *int        // Maximum number of results to return

	// Structured fact filtering fields - ONLY indexed fields
	FactCategory   *string // Filter by fact category (profile_stable, preference, goal_plan, etc.)
	FactAttribute  *string // Filter by fact attribute (specific property being described)
	FactImportance *int    // Filter by importance score (1, 2, 3)

	// Ranges for numeric/date fields
	FactImportanceMin *int // Minimum importance score (inclusive)
	FactImportanceMax *int // Maximum importance score (inclusive)

	// Timestamp filtering
	TimestampAfter  *time.Time // Filter for facts created after this time (inclusive)
	TimestampBefore *time.Time // Filter for facts created before this time (inclusive)

	// Document references filtering
	DocumentReferences []string // Filter by document reference IDs
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
	FieldSource   string                `json:"source"`       // Merged from StructuredConversation
	People        []string              `json:"people"`       // Merged from StructuredConversation
	User          string                `json:"user"`         // Merged from StructuredConversation
	Conversation  []ConversationMessage `json:"conversation"` // Merged from StructuredConversation
	FieldTags     []string              `json:"tags,omitempty"`
	FieldMetadata map[string]string     `json:"metadata,omitempty"`
}

// Document interface implementation for ConversationDocument.
func (cd *ConversationDocument) ID() string {
	return cd.FieldID
}

func (cd *ConversationDocument) Content() string {
	var content strings.Builder
	content.Grow(len(cd.Conversation) * 50) // rough estimate
	hasContent := false                     // Track if any substantive content is added
	for _, msg := range cd.Conversation {
		trimmedMsgContent := strings.TrimSpace(msg.Content)
		if trimmedMsgContent == "" {
			continue // Skip messages with only whitespace content
		}
		content.WriteString(msg.Speaker)
		content.WriteString(": ")
		content.WriteString(trimmedMsgContent)
		content.WriteString("\n")
		hasContent = true
	}
	if !hasContent {
		return "" // If no messages had real content, return empty string
	}
	return strings.TrimSpace(content.String()) // Final trim for the whole block
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

// MemoryFact represents an extracted fact about a person.
type MemoryFact struct {
	ID        string            `json:"id"`
	Content   string            `json:"content"`
	Timestamp time.Time         `json:"timestamp"`
	Source    string            `json:"source"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type QueryResult struct {
	Facts []MemoryFact `json:"facts"`
}

type ProgressUpdate struct {
	Processed int    `json:"processed"`
	Total     int    `json:"total"`
	Stage     string `json:"stage,omitempty"` // "extracting", "storing", etc.
}

type Storage interface {
	Store(ctx context.Context, documents []Document) (<-chan ProgressUpdate, <-chan error)
	Query(ctx context.Context, query string, filter *Filter) (QueryResult, error)
}

// Helper functions to convert slices to Document interface

// TextDocumentsToDocuments converts a slice of TextDocument to a slice of Document.
func TextDocumentsToDocuments(textDocs []TextDocument) []Document {
	docs := make([]Document, len(textDocs))
	for i := range textDocs {
		// Create a new variable for the address operation to avoid capturing loop variable
		doc := textDocs[i]
		docs[i] = &doc
	}
	return docs
}

// IsEmpty returns true if the TagsFilter has no filtering criteria.
func (tf *TagsFilter) IsEmpty() bool {
	if tf == nil {
		return true
	}
	return len(tf.All) == 0 && len(tf.Any) == 0 && tf.Expression == nil
}

// IsLeaf returns true if this is a leaf node (has tags).
func (be *BooleanExpression) IsLeaf() bool {
	return be != nil && len(be.Tags) > 0
}

// IsBranch returns true if this is a branch node (has left/right operands).
func (be *BooleanExpression) IsBranch() bool {
	return be != nil && be.Left != nil && be.Right != nil
}
