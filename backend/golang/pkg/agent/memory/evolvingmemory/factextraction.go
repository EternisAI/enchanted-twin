package evolvingmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

// normalizeAndFormatConversation replaces primary user name with "primaryUser" and returns JSON.
func normalizeAndFormatConversation(convDoc memory.ConversationDocument) (string, error) {
	normalized := convDoc

	// Replace primary user name in conversation messages
	for i, msg := range normalized.Conversation {
		if msg.Speaker == convDoc.User {
			normalized.Conversation[i].Speaker = "primaryUser"
		}
	}

	// Replace primary user name in people list
	for i, person := range normalized.People {
		if person == convDoc.User {
			normalized.People[i] = "primaryUser"
		}
	}

	// Update the user field
	normalized.User = "primaryUser"

	// Just JSON marshal the whole thing
	jsonBytes, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal conversation: %w", err)
	}

	return string(jsonBytes), nil
}

// extractFactsFromConversation extracts facts for a given speaker from a structured conversation.
func (s *WeaviateStorage) extractFactsFromConversation(ctx context.Context, convDoc memory.ConversationDocument, speakerID string, currentSystemDate string, docEventDateStr string) ([]string, error) {
	s.logger.Infof("== Starting Rich Fact Extraction for Speaker: %s == (Conversation ID: '%s')", speakerID, convDoc.ID())

	factExtractionToolsList := []openai.ChatCompletionToolParam{
		extractFactsTool,
	}

	// Normalize and format as JSON
	conversationJSON, err := normalizeAndFormatConversation(convDoc)
	if err != nil {
		s.logger.Errorf("Failed to normalize conversation for speaker %s in conversation %s: %v", speakerID, convDoc.ID(), err)
		return nil, fmt.Errorf("conversation normalization error: %w", err)
	}

	if len(convDoc.Conversation) == 0 {
		s.logger.Warnf("No conversation messages found for speaker %s in conversation %s.", speakerID, convDoc.ID())
		return []string{}, nil
	}

	// Dead simple: static prompt + JSON conversation
	llmMsgs := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(ConversationFactExtractionPrompt),
		openai.UserMessage(conversationJSON),
	}

	s.logger.Debugf("Calling LLM for Rich Fact Extraction (%s). Model: %s, Tools: %d tools", speakerID, openAIChatModel, len(factExtractionToolsList))

	llmResponse, err := s.completionsService.Completions(ctx, llmMsgs, factExtractionToolsList, openAIChatModel)
	if err != nil {
		s.logger.Errorf("LLM completion error during rich fact extraction for speaker %s in conversation %s: %v", speakerID, convDoc.ID(), err)
		return nil, fmt.Errorf("LLM completion error for speaker %s, conversation %s: %w", speakerID, convDoc.ID(), err)
	}

	var extractedFacts []string
	if len(llmResponse.ToolCalls) > 0 {
		for _, toolCall := range llmResponse.ToolCalls {
			if toolCall.Function.Name == ExtractFactsToolName {
				var args ExtractFactsToolArguments
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err == nil {
					extractedFacts = append(extractedFacts, args.Facts...)
					s.logger.Infof("Successfully parsed EXTRACT_FACTS tool call. Extracted %d facts for speaker %s from conversation %s using rich context.", len(args.Facts), speakerID, convDoc.ID())
				} else {
					s.logger.Warnf("Failed to unmarshal EXTRACT_FACTS arguments for speaker %s from conversation %s: %v. Arguments: %s", speakerID, convDoc.ID(), err, toolCall.Function.Arguments)
				}
			} else {
				s.logger.Warnf("LLM called an unexpected tool '%s' during rich fact extraction for speaker %s from conversation %s.", toolCall.Function.Name, speakerID, convDoc.ID())
			}
		}
	} else {
		s.logger.Info("LLM response for rich fact extraction for speaker %s from conversation %s did not contain tool calls. No facts extracted by tool.", speakerID, convDoc.ID())
	}

	return extractedFacts, nil
}

// extractFactsFromTextDocument extracts facts from legacy text documents.
func (s *WeaviateStorage) extractFactsFromTextDocument(ctx context.Context, textDoc memory.TextDocument, speakerID string, currentSystemDate string, docEventDateStr string) ([]string, error) {
	s.logger.Infof("== Starting Legacy Text Fact Extraction for Speaker: %s == (Document ID: '%s')", speakerID, textDoc.ID())

	factExtractionToolsList := []openai.ChatCompletionToolParam{
		extractFactsTool,
	}

	// Use the legacy TextDocument prompt (still needs templating for legacy support)
	sysPrompt := strings.ReplaceAll(TextDocumentFactExtractionPrompt, "{speaker_name}", speakerID)
	sysPrompt = strings.ReplaceAll(sysPrompt, "{current_date}", currentSystemDate)
	sysPrompt = strings.ReplaceAll(sysPrompt, "{content_date}", docEventDateStr)

	content := textDoc.Content()
	if content == "" {
		s.logger.Warnf("No content found in text document for speaker %s in document %s.", speakerID, textDoc.ID())
		return []string{}, nil
	}

	llmMsgs := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(sysPrompt),
		openai.UserMessage(content),
	}

	s.logger.Debugf("Calling LLM for Legacy Text Fact Extraction (%s). Model: %s, Tools: %d tools", speakerID, openAIChatModel, len(factExtractionToolsList))

	llmResponse, err := s.completionsService.Completions(ctx, llmMsgs, factExtractionToolsList, openAIChatModel)
	if err != nil {
		s.logger.Errorf("LLM completion error during legacy text fact extraction for speaker %s in document %s: %v", speakerID, textDoc.ID(), err)
		return nil, fmt.Errorf("LLM completion error for speaker %s, document %s: %w", speakerID, textDoc.ID(), err)
	}

	var extractedFacts []string
	if len(llmResponse.ToolCalls) > 0 {
		for _, toolCall := range llmResponse.ToolCalls {
			if toolCall.Function.Name == ExtractFactsToolName {
				var args ExtractFactsToolArguments
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err == nil {
					extractedFacts = append(extractedFacts, args.Facts...)
					s.logger.Infof("Successfully parsed EXTRACT_FACTS tool call. Extracted %d facts for speaker %s from text document %s.", len(args.Facts), speakerID, textDoc.ID())
				} else {
					s.logger.Warnf("Failed to unmarshal EXTRACT_FACTS arguments for speaker %s from text document %s: %v. Arguments: %s", speakerID, textDoc.ID(), err, toolCall.Function.Arguments)
				}
			} else {
				s.logger.Warnf("LLM called an unexpected tool '%s' during legacy text fact extraction for speaker %s from text document %s.", toolCall.Function.Name, speakerID, textDoc.ID())
			}
		}
	} else {
		s.logger.Info("LLM response for legacy text fact extraction for speaker %s from text document %s did not contain tool calls. No facts extracted by tool.", speakerID, textDoc.ID())
	}

	return extractedFacts, nil
}
