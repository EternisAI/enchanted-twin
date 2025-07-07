package evolvingmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

// ExtractFactsFromDocument routes fact extraction based on document type.
// This is pure business logic extracted from the adapter.
// Returns the extracted facts. The source document is already known by the caller.
func ExtractFactsFromDocument(ctx context.Context, doc memory.Document, completionsService *ai.Service, completionsModel string, logger *log.Logger) ([]*memory.MemoryFact, error) {
	switch typedDoc := doc.(type) {
	case *memory.ConversationDocument:
		// Extract for the document-level context (no specific speaker)
		return extractFactsFromConversation(ctx, *typedDoc, completionsService, completionsModel, doc, logger)

	case *memory.TextDocument:
		return extractFactsFromTextDocument(ctx, *typedDoc, completionsService, completionsModel, doc, logger)

	default:
		return nil, fmt.Errorf("unsupported document type: %T", doc)
	}
}

// extractFactsFromConversation extracts facts for a given speaker from a structured conversation.
func extractFactsFromConversation(ctx context.Context, convDoc memory.ConversationDocument, completionsService *ai.Service, completionsModel string, sourceDoc memory.Document, logger *log.Logger) ([]*memory.MemoryFact, error) {
	factExtractionToolsList := []openai.ChatCompletionToolParam{
		extractFactsTool,
	}

	content := convDoc.Content()

	if len(convDoc.Conversation) == 0 {
		logger.Info("Skipping empty conversation", "id", convDoc.ID())
		return []*memory.MemoryFact{}, nil
	}

	llmMsgs := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(FactExtractionPrompt),
		openai.UserMessage(content),
	}

	logger.Debug("Sending conversation to LLM", "system_prompt_length", len(FactExtractionPrompt), "json_length", len(content))

	result, err := completionsService.Completions(ctx, llmMsgs, factExtractionToolsList, completionsModel, ai.Background)
	if err != nil {
		logger.Error("LLM completion FAILED for conversation", "id", convDoc.ID(), "error", err)
		return nil, fmt.Errorf("LLM completion error for conversation %s: %w", convDoc.ID(), err)
	}
	llmResponse := result.Message

	logger.Debug("LLM Response for conversation", "id", convDoc.ID())
	logger.Debug("Response Content", "content", llmResponse.Content)
	logger.Debug("Tool Calls Count", "count", len(llmResponse.ToolCalls))

	if len(llmResponse.ToolCalls) == 0 {
		logger.Warn("No tool calls returned for conversation - fact extraction may have failed", "id", convDoc.ID())
	}

	var extractedFacts []*memory.MemoryFact
	for _, toolCall := range llmResponse.ToolCalls {
		logger.Debug("Tool Call", "name", toolCall.Function.Name)
		logger.Debug("Arguments", "args", toolCall.Function.Arguments)

		if toolCall.Function.Name != ExtractFactsToolName {
			logger.Debug("SKIPPING: Wrong tool name", "expected", ExtractFactsToolName)
			continue
		}

		var args ExtractMemoryFactsToolArguments
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			logger.Error("FAILED to unmarshal tool arguments", "error", err)
			continue
		}

		logger.Debug("Successfully parsed structured facts from conversation", "count", len(args.Facts))

		if len(args.Facts) == 0 {
			logger.Warn("Tool call returned zero facts for conversation", "id", convDoc.ID())
		}

		for factIdx := range args.Facts {
			memoryFact := &args.Facts[factIdx]

			// Set required fields that LLM doesn't provide
			memoryFact.ID = uuid.New().String()
			memoryFact.Source = sourceDoc.Source()

			if filePath := sourceDoc.FilePath(); filePath != "" {
				memoryFact.FilePath = filePath
			}

			memoryFact.Content = memoryFact.GenerateContent()
			if timestamp := sourceDoc.Timestamp(); timestamp != nil {
				memoryFact.Timestamp = *timestamp
			} else {
				memoryFact.Timestamp = time.Now() // fallback if no timestamp available
			}

			// Set document reference
			if memoryFact.DocumentReferences == nil {
				memoryFact.DocumentReferences = []string{sourceDoc.ID()}
			}

			logger.Debug("Conversation Fact",
				"index", factIdx+1,
				"id", memoryFact.ID,
				"category", memoryFact.Category,
				"subject", memoryFact.Subject,
				"attribute", memoryFact.Attribute,
				"value", memoryFact.Value,
				"importance", memoryFact.Importance,
				"sensitivity", memoryFact.Sensitivity,
				"source", memoryFact.Source)

			extractedFacts = append(extractedFacts, memoryFact)
		}
	}

	logger.Debug("=== CONVERSATION FACT EXTRACTION SUMMARY ===")
	logger.Info("Conversation fact extraction completed", "id", convDoc.ID(), "facts_extracted", len(extractedFacts))
	if len(extractedFacts) == 0 {
		logger.Warn("NO FACTS EXTRACTED from conversation", "id", convDoc.ID())
	}
	logger.Debug("=== CONVERSATION FACT EXTRACTION END ===")

	return extractedFacts, nil
}

// extractFactsFromTextDocument extracts facts from text documents.
func extractFactsFromTextDocument(ctx context.Context, textDoc memory.TextDocument, completionsService *ai.Service, completionsModel string, sourceDoc memory.Document, logger *log.Logger) ([]*memory.MemoryFact, error) {
	factExtractionToolsList := []openai.ChatCompletionToolParam{
		extractFactsTool,
	}

	content := textDoc.Content()
	if content == "" {
		logger.Info("Skipping empty text document", "id", textDoc.ID())
		return []*memory.MemoryFact{}, nil
	}

	logger.Debug("=== FACT EXTRACTION START ===")
	logger.Debug("Document details",
		"id", textDoc.ID(),
		"source", textDoc.Source(),
		"tags", textDoc.Tags(),
		"metadata", textDoc.Metadata(),
		"content_length", len(content))
	logger.Debug("Full Content", "content", content)

	llmMsgs := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(FactExtractionPrompt),
		openai.UserMessage(content),
	}

	logger.Debug("Sending to LLM", "system_prompt_length", len(FactExtractionPrompt), "user_message_length", len(content))

	result, err := completionsService.Completions(ctx, llmMsgs, factExtractionToolsList, completionsModel, ai.Background)
	if err != nil {
		logger.Error("LLM completion FAILED for document", "id", textDoc.ID(), "error", err)
		return nil, fmt.Errorf("LLM completion error for document %s: %w", textDoc.ID(), err)
	}
	llmResponse := result.Message

	logger.Debug("LLM Response for document", "id", textDoc.ID(), "content", llmResponse.Content, "tool_calls_count", len(llmResponse.ToolCalls))

	if len(llmResponse.ToolCalls) == 0 {
		logger.Warn("No tool calls returned for document - fact extraction may have failed", "id", textDoc.ID())
	}

	var extractedFacts []*memory.MemoryFact
	for i, toolCall := range llmResponse.ToolCalls {
		logger.Debug("Tool Call", "index", i+1, "name", toolCall.Function.Name)
		logger.Debug("Arguments", "args", toolCall.Function.Arguments)

		if toolCall.Function.Name != ExtractFactsToolName {
			logger.Debug("SKIPPING: Wrong tool name", "expected", ExtractFactsToolName)
			continue
		}

		var args ExtractMemoryFactsToolArguments
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			logger.Error("FAILED to unmarshal tool arguments", "error", err)
			continue
		}

		logger.Debug("Successfully parsed structured facts", "count", len(args.Facts))

		if len(args.Facts) == 0 {
			logger.Warn("Tool call returned zero facts for document", "id", textDoc.ID())
		}

		for factIdx := range args.Facts {
			memoryFact := &args.Facts[factIdx]

			// Set required fields that LLM doesn't provide
			memoryFact.ID = uuid.New().String()
			memoryFact.Source = sourceDoc.Source()
			memoryFact.Content = memoryFact.GenerateContent()
			if timestamp := sourceDoc.Timestamp(); timestamp != nil {
				memoryFact.Timestamp = *timestamp
			} else {
				memoryFact.Timestamp = time.Now() // fallback if no timestamp available
			}

			// Set document reference
			if memoryFact.DocumentReferences == nil {
				memoryFact.DocumentReferences = []string{sourceDoc.ID()}
			}

			if filePath := sourceDoc.FilePath(); filePath != "" {
				memoryFact.FilePath = filePath
			}

			logger.Debug("Text Document Fact",
				"index", factIdx+1,
				"id", memoryFact.ID,
				"category", memoryFact.Category,
				"subject", memoryFact.Subject,
				"attribute", memoryFact.Attribute,
				"value", memoryFact.Value,
				"importance", memoryFact.Importance,
				"sensitivity", memoryFact.Sensitivity,
				"source", memoryFact.Source)

			extractedFacts = append(extractedFacts, memoryFact)
		}
	}

	if len(extractedFacts) == 0 {
		logger.Warn("NO FACTS EXTRACTED from document", "id", textDoc.ID())
	} else {
		logger.Info("Document fact extraction completed", "id", textDoc.ID(), "facts_extracted", len(extractedFacts))
	}

	return extractedFacts, nil
}
