package evolvingmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

func extractJSONFromContent(content string) (string, error) {
	content = strings.TrimSpace(content)

	content = strings.ReplaceAll(content, "│", "")
	content = strings.TrimSpace(content)

	jsonRegex := regexp.MustCompile(`(?s)<json>\s*(.*?)\s*</json>`)
	matches := jsonRegex.FindStringSubmatch(content)

	if len(matches) > 1 {
		jsonContent := strings.TrimSpace(matches[1])
		jsonContent = strings.ReplaceAll(jsonContent, "│", "")
		return strings.TrimSpace(jsonContent), nil
	}

	if strings.HasPrefix(content, "{") && strings.HasSuffix(content, "}") {
		return content, nil
	}

	return "", fmt.Errorf("no valid JSON found in content")
}

func parseFactsFromContentField(llmResponse openai.ChatCompletionMessage, sourceDoc memory.Document, docType string, logger *log.Logger) []*memory.MemoryFact {
	if llmResponse.Content == "" {
		return nil
	}

	logger.Debug("Raw content before parsing", "content", llmResponse.Content)
	jsonContent, err := extractJSONFromContent(llmResponse.Content)
	if err != nil {
		logger.Debug("Failed to extract JSON from content field", "error", err, "content_preview", llmResponse.Content[:min(200, len(llmResponse.Content))])
		return nil
	}

	logger.Debug("Extracted JSON from content field", "json", jsonContent)

	var args ExtractMemoryFactsToolArguments
	if err := json.Unmarshal([]byte(jsonContent), &args); err != nil {
		logger.Error("FAILED to unmarshal JSON from content field", "error", err, "json", jsonContent)
		return nil
	}

	logger.Debug("Successfully parsed structured facts from content field", "count", len(args.Facts))

	if len(args.Facts) == 0 {
		logger.Warn("Content field returned zero facts for "+docType, "id", sourceDoc.ID())
		return nil
	}

	var extractedFacts []*memory.MemoryFact
	for factIdx := range args.Facts {
		memoryFact := &args.Facts[factIdx]

		memoryFact.ID = uuid.New().String()
		memoryFact.Source = sourceDoc.Source()
		memoryFact.Content = memoryFact.GenerateContent()
		if timestamp := sourceDoc.Timestamp(); timestamp != nil {
			memoryFact.Timestamp = *timestamp
		} else {
			memoryFact.Timestamp = time.Now()
		}

		if memoryFact.DocumentReferences == nil {
			memoryFact.DocumentReferences = []string{sourceDoc.ID()}
		}

		if filePath := sourceDoc.FilePath(); filePath != "" {
			memoryFact.FilePath = filePath
		}

		logger.Debug(docType+" Fact (from content)",
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

	return extractedFacts
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

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

	// Use anti-duplication prompt for twin chat conversations
	var systemPrompt string
	if convDoc.Source() == "chat" {
		systemPrompt = TwinChatFactExtractionPrompt
		logger.Debug("Using twin chat anti-duplication prompt", "source", convDoc.Source())
	} else {
		systemPrompt = FactExtractionPrompt
		logger.Debug("Using standard fact extraction prompt", "source", convDoc.Source())
	}

	llmMsgs := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
		openai.UserMessage(content),
	}

	logger.Debug("Sending conversation to LLM", "system_prompt_length", len(systemPrompt), "json_length", len(content))

	llmResponse, err := completionsService.Completions(ctx, llmMsgs, factExtractionToolsList, completionsModel)
	if err != nil {
		logger.Error("LLM completion FAILED for conversation", "id", convDoc.ID(), "error", err)
		return nil, fmt.Errorf("LLM completion error for conversation %s: %w", convDoc.ID(), err)
	}

	logger.Debug("LLM Response for conversation", "id", convDoc.ID())
	logger.Debug("Response Content", "content", llmResponse.Content)
	logger.Debug("Tool Calls Count", "count", len(llmResponse.ToolCalls))

	var extractedFacts []*memory.MemoryFact

	// First, try to extract facts from tool calls
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

			if len(convDoc.Conversation) > 0 {
				// Use the timestamp from the conversation messages for temporal context
				firstMessageTime := convDoc.Conversation[0].Time
				temporalContext := firstMessageTime.Format("2006-01-02") // YYYY-MM-DD format
				memoryFact.TemporalContext = &temporalContext
			}

			// Timestamp is always when the fact was processed, not when the conversation happened
			memoryFact.Timestamp = time.Now()

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

	if len(extractedFacts) == 0 && len(llmResponse.ToolCalls) == 0 {
		logger.Debug("No tool calls found, attempting to parse facts from content field")

		if llmResponse.Content != "" {
			extractedFacts = parseFactsFromContentField(llmResponse, sourceDoc, "conversation", logger)
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

	llmResponse, err := completionsService.Completions(ctx, llmMsgs, factExtractionToolsList, completionsModel)
	if err != nil {
		logger.Error("LLM completion FAILED for document", "id", textDoc.ID(), "error", err)
		return nil, fmt.Errorf("LLM completion error for document %s: %w", textDoc.ID(), err)
	}

	logger.Debug("LLM Response for document", "id", textDoc.ID(), "content", llmResponse.Content, "tool_calls_count", len(llmResponse.ToolCalls))

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

			memoryFact.ID = uuid.New().String()
			memoryFact.Source = sourceDoc.Source()
			memoryFact.Content = memoryFact.GenerateContent()

			if timestamp := sourceDoc.Timestamp(); timestamp != nil {
				temporalContext := timestamp.Format("2006-01-02") // YYYY-MM-DD format
				memoryFact.TemporalContext = &temporalContext
			}

			// Timestamp is always when the fact was processed, not when the document was created
			memoryFact.Timestamp = time.Now()

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

	if len(extractedFacts) == 0 && len(llmResponse.ToolCalls) == 0 {
		logger.Debug("No tool calls found, attempting to parse facts from content field")

		if llmResponse.Content != "" {
			extractedFacts = parseFactsFromContentField(llmResponse, sourceDoc, "document", logger)
		}
	}

	if len(extractedFacts) == 0 {
		logger.Warn("NO FACTS EXTRACTED from document", "id", textDoc.ID())
	} else {
		logger.Info("Document fact extraction completed", "id", textDoc.ID(), "facts_extracted", len(extractedFacts))
	}

	return extractedFacts, nil
}
