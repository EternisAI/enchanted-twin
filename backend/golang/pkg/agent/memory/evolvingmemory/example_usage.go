package evolvingmemory

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/log"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

// ExampleUsageStructuredConversations demonstrates how to use the new structured conversation format
func ExampleUsageStructuredConversations(storage *WeaviateStorage) error {
	ctx := context.Background()

	// Create an example structured conversation
	exampleDoc := CreateExampleConversationDocument()

	// Print the JSON representation
	jsonData, err := ConversationDocumentToJSON(exampleDoc)
	if err != nil {
		return fmt.Errorf("failed to convert to JSON: %w", err)
	}

	fmt.Println("Example Structured Conversation JSON:")
	fmt.Println(string(jsonData))
	fmt.Println()

	// Validate the document
	if err := ValidateConversationDocument(exampleDoc); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	fmt.Println("✓ Document validation passed")

	// Store the conversation using the new structured format
	docs := []memory.ConversationDocument{*exampleDoc}

	progressChan := make(chan memory.ProgressUpdate, 10)
	go func() {
		for update := range progressChan {
			fmt.Printf("Progress: %d/%d documents processed\n", update.Processed, update.Total)
		}
	}()

	// Use the structured storage method
	if err := storage.StoreConversations(ctx, docs, progressChan); err != nil {
		return fmt.Errorf("failed to store structured conversations: %w", err)
	}

	fmt.Println("✓ Structured conversation stored successfully")

	// Query the stored memories
	queryResult, err := storage.Query(ctx, "pizza preferences")
	if err != nil {
		return fmt.Errorf("failed to query memories: %w", err)
	}

	fmt.Printf("Query results: Found %d relevant memories\n", len(queryResult.Documents))
	for i, doc := range queryResult.Documents {
		fmt.Printf("Memory %d: %s\n", i+1, doc.Content)
	}

	return nil
}

// CreateComplexExampleConversation creates a more complex example with multiple people
func CreateComplexExampleConversation() *memory.ConversationDocument {
	now := time.Now()
	return &memory.ConversationDocument{
		ID: "complex_conversation_001",
		Conversation: memory.StructuredConversation{
			Source: "team_chat",
			People: []string{"Alice", "Bob", "Charlie"},
			User:   "Alice",
			Conversation: []memory.ConversationMessage{
				{
					Speaker: "Alice",
					Content: "Hey team, I'm planning to start a new fitness routine. Any recommendations?",
					Time:    now.Add(-30 * time.Minute),
				},
				{
					Speaker: "Bob",
					Content: "I've been doing CrossFit for 2 years now. It's intense but really effective!",
					Time:    now.Add(-28 * time.Minute),
				},
				{
					Speaker: "Charlie",
					Content: "I prefer yoga and running. Less intense but great for mental health too.",
					Time:    now.Add(-25 * time.Minute),
				},
				{
					Speaker: "Alice",
					Content: "I'm interested in both approaches. Bob, how often do you do CrossFit?",
					Time:    now.Add(-22 * time.Minute),
				},
				{
					Speaker: "Bob",
					Content: "I go 4 times a week - Monday, Tuesday, Thursday, Friday. Wednesdays I do light cardio.",
					Time:    now.Add(-20 * time.Minute),
				},
				{
					Speaker: "Alice",
					Content: "That sounds like a good schedule. Charlie, what's your yoga routine like?",
					Time:    now.Add(-18 * time.Minute),
				},
				{
					Speaker: "Charlie",
					Content: "I do yoga every morning for 30 minutes, and I run 3 times a week in the evenings. Usually Tuesday, Thursday, and Saturday.",
					Time:    now.Add(-15 * time.Minute),
				},
				{
					Speaker: "Alice",
					Content: "Thanks both! I think I'll start with yoga in the mornings and maybe try CrossFit once a week to begin with.",
					Time:    now.Add(-10 * time.Minute),
				},
			},
		},
		Tags: []string{"fitness", "health", "planning", "team_discussion"},
		Metadata: map[string]string{
			"session_type": "team_chat",
			"platform":     "slack",
			"channel":      "general",
		},
	}
}

// DemonstrateFormatComparison shows the difference between old and new formats
func DemonstrateFormatComparison() {
	fmt.Println("=== FORMAT COMPARISON ===")
	fmt.Println()

	// Create example using new format
	newFormatDoc := CreateExampleConversationDocument()

	// Convert to old format
	oldFormatDoc := newFormatDoc.ToTextDocument()

	fmt.Println("NEW STRUCTURED FORMAT:")
	newJSON, _ := ConversationDocumentToJSON(newFormatDoc)
	fmt.Println(string(newJSON))
	fmt.Println()

	fmt.Println("OLD FORMAT (converted):")
	fmt.Printf("ID: %s\n", oldFormatDoc.ID)
	fmt.Printf("Content: %s\n", oldFormatDoc.Content)
	fmt.Printf("Timestamp: %v\n", oldFormatDoc.Timestamp)
	fmt.Printf("Tags: %v\n", oldFormatDoc.Tags)
	fmt.Printf("Metadata: %v\n", oldFormatDoc.Metadata)
	fmt.Println()

	fmt.Println("BENEFITS OF NEW FORMAT:")
	fmt.Println("✓ Explicit structure - no parsing required")
	fmt.Println("✓ Proper timestamp per message")
	fmt.Println("✓ Clear speaker identification")
	fmt.Println("✓ Validation built-in")
	fmt.Println("✓ Source tracking")
	fmt.Println("✓ User identification")
	fmt.Println("✓ JSON schema-friendly")
}

// LoggerExample shows how to set up logging for the new format
func LoggerExample() *log.Logger {
	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		TimeFormat:      time.Kitchen,
		Prefix:          "StructuredMemory",
	})
	logger.SetLevel(log.DebugLevel)
	return logger
}
