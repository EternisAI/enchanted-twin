package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/whatsapp"
)

func main() {
	// Define command-line flags
	inputFile := flag.String("input", "", "Path to WhatsApp SQLite database file (required)")
	outputFile := flag.String("output", "", "Path to output JSONL file (required)")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	dryRun := flag.Bool("dry-run", false, "Preview the output without writing the file")
	saveDocsJSON := flag.Bool("save-docs-json", false, "Save ConversationDocument structures to a JSON file")
	help := flag.Bool("help", false, "Show help message")

	// Custom usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "WhatsApp to JSONL Converter\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "This tool converts WhatsApp SQLite database files to JSONL format.\n")
		fmt.Fprintf(os.Stderr, "Each line in the output JSONL file represents a complete conversation.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s -input ChatStorage.sqlite -output conversations.jsonl\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -input ChatStorage.sqlite -output conversations.jsonl -dry-run -verbose\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -input ChatStorage.sqlite -output conversations.jsonl -save-docs-json\n", os.Args[0])
	}

	flag.Parse()

	// Show help if requested
	if *help {
		flag.Usage()
		os.Exit(0)
	}

	// Validate required flags
	if *inputFile == "" || *outputFile == "" {
		fmt.Fprintf(os.Stderr, "Error: Both -input and -output flags are required\n\n")
		flag.Usage()
		os.Exit(1)
	}

	// Set up logger
	logger := log.New(os.Stdout)
	if *verbose {
		logger.SetLevel(log.DebugLevel)
	} else {
		logger.SetLevel(log.InfoLevel)
	}

	// Validate input file exists
	if _, err := os.Stat(*inputFile); os.IsNotExist(err) {
		logger.Error("Input file does not exist", "file", *inputFile)
		os.Exit(1)
	}

	// Ensure output file has .jsonl extension
	if !strings.HasSuffix(*outputFile, ".jsonl") {
		logger.Warn("Output file should have .jsonl extension, adding it", "file", *outputFile)
		*outputFile = *outputFile + ".jsonl"
	}

	// Create output directory if it doesn't exist (unless dry-run)
	if !*dryRun {
		outputDir := filepath.Dir(*outputFile)
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			logger.Error("Failed to create output directory", "dir", outputDir, "error", err)
			os.Exit(1)
		}
	}

	logger.Info("Starting WhatsApp to JSONL conversion",
		"input", *inputFile,
		"output", *outputFile,
		"dry-run", *dryRun)

	// Create WhatsApp processor
	processor, err := whatsapp.NewWhatsappProcessor(nil, logger)
	if err != nil {
		logger.Error("Failed to create WhatsApp processor", "error", err)
		os.Exit(1)
	}

	// Process the SQLite file
	ctx := context.Background()
	logger.Info("Reading WhatsApp database...")

	records, err := processor.ProcessFile(ctx, *inputFile)
	if err != nil {
		logger.Error("Failed to process WhatsApp database", "error", err)
		os.Exit(1)
	}

	logger.Info("Successfully read database", "conversations", len(records))

	// Call ToDocuments to convert records to memory documents
	logger.Info("Converting records to memory documents...")
	documents, err := processor.ToDocuments(ctx, records)
	if err != nil {
		logger.Error("Failed to convert to documents", "error", err)
		os.Exit(1)
	}

	logger.Info("Successfully converted to documents", "documents", len(documents))

	// Save ConversationDocument structures to JSON if requested
	if *saveDocsJSON {
		jsonFilename := strings.TrimSuffix(*outputFile, ".jsonl") + "-docs.json"
		logger.Info("Saving ConversationDocument structures to JSON", "file", jsonFilename)

		// Create a slice to hold serializable representations
		type SerializableConversation struct {
			ID           string                       `json:"id"`
			Source       string                       `json:"source"`
			Tags         []string                     `json:"tags"`
			User         string                       `json:"user"`
			People       []string                     `json:"people"`
			Metadata     map[string]string            `json:"metadata"`
			Timestamp    *time.Time                   `json:"timestamp,omitempty"`
			MessageCount int                          `json:"message_count"`
			Conversation []memory.ConversationMessage `json:"conversation"`
			Content      string                       `json:"content_preview"`
		}

		var docsData []SerializableConversation

		for _, doc := range documents {
			if convDoc, ok := doc.(*memory.ConversationDocument); ok {
				serializable := SerializableConversation{
					ID:           convDoc.ID(),
					Source:       convDoc.Source(),
					Tags:         convDoc.Tags(),
					User:         convDoc.User,
					People:       convDoc.People,
					Metadata:     convDoc.Metadata(),
					Timestamp:    convDoc.Timestamp(),
					MessageCount: len(convDoc.Conversation),
					Conversation: convDoc.Conversation,
					Content:      convDoc.Content(),
				}
				docsData = append(docsData, serializable)
			}
		}

		// Marshal to JSON with indentation
		jsonData, err := json.MarshalIndent(docsData, "", "  ")
		if err != nil {
			logger.Error("Failed to marshal ConversationDocuments to JSON", "error", err)
		} else {
			// Write to file
			if err := os.WriteFile(jsonFilename, jsonData, 0o644); err != nil {
				logger.Error("Failed to write JSON file", "file", jsonFilename, "error", err)
			} else {
				logger.Info("Successfully saved ConversationDocument structures to JSON",
					"file", jsonFilename,
					"conversations", len(docsData),
					"size", fmt.Sprintf("%.2f MB", float64(len(jsonData))/1024/1024))
			}
		}
	}

	// Print out ConversationDocument structures for inspection
	if *verbose || *dryRun {
		logger.Info("Inspecting ConversationDocument structures:")

		for i, doc := range documents {
			if i >= 5 { // Show first 5 documents
				if len(documents) > 5 {
					logger.Info("... and more documents (showing first 5 only)")
				}
				break
			}

			// Type assert to ConversationDocument
			if convDoc, ok := doc.(*memory.ConversationDocument); ok {
				logger.Info("ConversationDocument",
					"index", i+1,
					"ID", convDoc.ID(),
					"Source", convDoc.Source(),
					"Tags", convDoc.Tags(),
					"User", convDoc.User,
					"People", convDoc.People,
					"MessageCount", len(convDoc.Conversation),
					"Metadata", convDoc.Metadata())
			} else {
				logger.Warn("Document is not a ConversationDocument", "index", i+1, "type", fmt.Sprintf("%T", doc))
			}
		}
	}

	// Display sample conversation info if verbose or dry-run
	if (*verbose || *dryRun) && len(records) > 0 {
		logger.Debug("Sample record data (raw):")
		samplesToShow := 3
		if *dryRun {
			samplesToShow = 5 // Show more samples in dry-run mode
		}

		for i, record := range records {
			if i >= samplesToShow {
				break
			}

			id, _ := record.Data["id"].(string)
			people, _ := record.Data["people"].([]string)
			conversation, _ := record.Data["conversation"].([]map[string]interface{})

			logger.Debug("Conversation",
				"index", i+1,
				"id", id,
				"participants", people,
				"messages", len(conversation))

			// In dry-run mode, show first few messages of each conversation
			if *dryRun && len(conversation) > 0 {
				logger.Debug("First few messages:")
				for j, msg := range conversation {
					if j >= 3 { // Show first 3 messages
						break
					}
					speaker, _ := msg["speaker"].(string)
					content, _ := msg["content"].(string)
					if len(content) > 50 {
						content = content[:50] + "..."
					}
					logger.Debug(fmt.Sprintf("  [%d] %s: %s", j+1, speaker, content))
				}

				// Show sample JSONL output for the first conversation
				if i == 0 {
					logger.Info("Sample JSONL output (first conversation):")
					jsonRecord := struct {
						Data      map[string]interface{} `json:"data"`
						Timestamp string                 `json:"timestamp"`
						Source    string                 `json:"source"`
					}{
						Data:      record.Data,
						Timestamp: record.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
						Source:    record.Source,
					}

					jsonData, err := json.MarshalIndent(jsonRecord, "", "  ")
					if err == nil {
						fmt.Println(string(jsonData))
					}
				}
			}
		}
	}

	// Skip file writing in dry-run mode
	if *dryRun {
		logger.Info("Dry-run mode: Skipping file write")
	} else {
		// Save records to JSONL file
		logger.Info("Writing conversations to JSONL file...")
		if err := dataprocessing.SaveRecords(records, *outputFile); err != nil {
			logger.Error("Failed to save records", "error", err)
			os.Exit(1)
		}

		logger.Info("Conversion completed successfully!",
			"output", *outputFile,
			"conversations", len(records))
	}

	// Print summary statistics
	totalMessages := 0
	participants := make(map[string]bool)

	for _, record := range records {
		if conversation, ok := record.Data["conversation"].([]map[string]interface{}); ok {
			totalMessages += len(conversation)
		}
		if people, ok := record.Data["people"].([]string); ok {
			for _, person := range people {
				participants[person] = true
			}
		}
	}

	logger.Info("Summary statistics:",
		"total_conversations", len(records),
		"total_messages", totalMessages,
		"unique_participants", len(participants))

	if totalMessages > 0 && len(records) > 0 {
		avgMessagesPerConv := float64(totalMessages) / float64(len(records))
		logger.Info("Average messages per conversation:", "average", fmt.Sprintf("%.2f", avgMessagesPerConv))
	}
}
