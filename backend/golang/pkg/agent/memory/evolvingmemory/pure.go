package evolvingmemory

import (
	"fmt"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/weaviate/weaviate/entities/models"
)

// PrepareDocuments converts raw documents into prepared documents with extracted metadata
func PrepareDocuments(docs []memory.Document, currentTime time.Time) ([]PreparedDocument, []error) {
	prepared := make([]PreparedDocument, 0, len(docs))
	errors := make([]error, 0)

	for _, doc := range docs {
		p, err := prepareDocument(doc, currentTime)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to prepare document: %w", err))
			continue
		}
		prepared = append(prepared, p)
	}

	return prepared, errors
}

func prepareDocument(doc memory.Document, currentTime time.Time) (PreparedDocument, error) {
	prepared := PreparedDocument{
		Original:   doc,
		Timestamp:  currentTime,
		DateString: getCurrentDateForPrompt(),
	}

	// Determine document type and extract speaker info
	switch d := doc.(type) {
	case *memory.ConversationDocument:
		prepared.Type = DocumentTypeConversation
		// Use the User field as the speaker ID for conversation documents
		if d.User != "" {
			prepared.SpeakerID = d.User
		}
	case *memory.TextDocument:
		prepared.Type = DocumentTypeText
		// Text documents are document-level (no speaker)
		// In the current implementation, speakerID is hardcoded as "user" but
		// for the new pipeline we'll treat it as document-level
	default:
		return PreparedDocument{}, fmt.Errorf("unknown document type: %T", doc)
	}

	// Override timestamp if document provides one
	if ts := doc.Timestamp(); ts != nil && !ts.IsZero() {
		prepared.Timestamp = *ts
	}

	return prepared, nil
}

// DistributeWork splits documents evenly among workers
func DistributeWork(docs []PreparedDocument, workers int) [][]PreparedDocument {
	if workers <= 0 {
		workers = 1
	}

	chunks := make([][]PreparedDocument, workers)
	for i, doc := range docs {
		workerIdx := i % workers
		chunks[workerIdx] = append(chunks[workerIdx], doc)
	}

	return chunks
}

// ValidateMemoryOperation ensures speaker context rules are followed
func ValidateMemoryOperation(rule ValidationRule) error {
	switch rule.Action {
	case UPDATE, DELETE:
		// Document-level context cannot modify speaker-specific memories
		if rule.IsDocumentLevel && rule.TargetSpeakerID != "" {
			return fmt.Errorf("document-level context cannot %s speaker-specific memory", rule.Action)
		}

		// Speaker-specific context can only modify their own memories
		if !rule.IsDocumentLevel && rule.TargetSpeakerID != rule.CurrentSpeakerID {
			return fmt.Errorf("speaker %s cannot %s memory belonging to speaker %s",
				rule.CurrentSpeakerID, rule.Action, rule.TargetSpeakerID)
		}
	}
	return nil
}

// CreateMemoryObject builds the Weaviate object for ADD operations
func CreateMemoryObject(fact ExtractedFact, decision MemoryDecision) *models.Object {
	metadata := make(map[string]string)

	// Copy original metadata
	for k, v := range fact.Source.Original.Metadata() {
		metadata[k] = v
	}

	// Add speaker ID if present
	if fact.SpeakerID != "" {
		metadata["speakerID"] = fact.SpeakerID
	}

	return &models.Object{
		Class: ClassName,
		Properties: map[string]interface{}{
			"content":      fact.Content,
			"metadataJson": marshalMetadata(metadata),
			"timestamp":    fact.Source.Timestamp.Format(time.RFC3339),
		},
	}
}

// BatchObjects groups objects into batches of specified size
func BatchObjects(objects []*models.Object, size int) [][]*models.Object {
	if size <= 0 {
		size = 100 // default
	}

	var batches [][]*models.Object
	for i := 0; i < len(objects); i += size {
		end := i + size
		if end > len(objects) {
			end = len(objects)
		}
		batches = append(batches, objects[i:end])
	}

	return batches
}

func marshalMetadata(metadata map[string]string) string {
	// Simple implementation - in production might want proper JSON marshaling
	result := "{"
	first := true
	for k, v := range metadata {
		if !first {
			result += ","
		}
		result += fmt.Sprintf(`"%s":"%s"`, k, v)
		first = false
	}
	result += "}"
	return result
}
