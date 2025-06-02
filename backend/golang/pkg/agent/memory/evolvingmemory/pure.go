package evolvingmemory

import (
	"fmt"
	"strings"
	"time"

	"github.com/weaviate/weaviate/entities/models"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

// PrepareDocuments converts raw documents into prepared documents with extracted metadata.
func PrepareDocuments(docs []memory.Document, currentTime time.Time) ([]PreparedDocument, error) {
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

	if len(errors) > 0 {
		return nil, aggregateErrors(errors)
	}

	return prepared, nil
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

// DistributeWork splits documents evenly among workers.
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

// ValidateMemoryOperation ensures speaker context rules are followed.
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

// CreateMemoryObject builds the Weaviate object for ADD operations.
func CreateMemoryObject(fact ExtractedFact, decision MemoryDecision) *models.Object {
	metadata := make(map[string]string)

	for k, v := range fact.Source.Original.Metadata() {
		metadata[k] = v
	}

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

// marshalMetadata converts a metadata map to JSON string for storage.
func marshalMetadata(metadata map[string]string) string {
	if len(metadata) == 0 {
		return "{}"
	}

	var pairs []string
	for k, v := range metadata {
		pairs = append(pairs, fmt.Sprintf(`"%s":"%s"`, k, v))
	}

	return "{" + strings.Join(pairs, ",") + "}"
}

// aggregateErrors combines multiple errors into a single error with context about all failures.
func aggregateErrors(errors []error) error {
	if len(errors) == 0 {
		return nil
	}

	if len(errors) == 1 {
		return errors[0]
	}

	var messages []string
	for i, err := range errors {
		messages = append(messages, fmt.Sprintf("error %d: %v", i+1, err))
	}

	return fmt.Errorf("multiple errors occurred (%d total): %s", len(errors), strings.Join(messages, "; "))
}
