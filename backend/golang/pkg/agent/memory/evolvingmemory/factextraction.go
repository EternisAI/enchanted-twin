package evolvingmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

// extractFactsFromConversation extracts facts for a given speaker from a structured conversation.
func (s *WeaviateStorage) extractFactsFromConversation(ctx context.Context, convDoc memory.ConversationDocument, speakerID string, currentSystemDate string, docEventDateStr string) ([]string, error) {
	s.logger.Infof("== Starting Fact Extraction for Speaker: %s == (Conversation ID: '%s')", speakerID, convDoc.ID)

	factExtractionToolsList := []openai.ChatCompletionToolParam{
		extractFactsTool,
	}

	// Use the new FactExtractionPrompt for structured conversations
	sysPrompt := strings.ReplaceAll(FactExtractionPrompt, "{speaker_name}", speakerID)
	sysPrompt = strings.ReplaceAll(sysPrompt, "{current_date}", currentSystemDate)
	sysPrompt = strings.ReplaceAll(sysPrompt, "{conversation_date}", docEventDateStr)

	llmMsgs := []openai.ChatCompletionMessageParamUnion{
		{
			OfSystem: &openai.ChatCompletionSystemMessageParam{
				Content: openai.ChatCompletionSystemMessageParamContentUnion{
					OfString: openai.String(sysPrompt),
				},
			},
		},
	}

	// Build conversation context from structured messages
	parsedTurnsCount := 0
	for _, msg := range convDoc.Conversation.Conversation {
		if strings.TrimSpace(msg.Content) == "" {
			continue
		}

		parsedTurnsCount++
		fullTurnContent := fmt.Sprintf("%s: %s", msg.Speaker, msg.Content)

		if msg.Speaker == speakerID { // Target speaker's turn for the LLM context
			llmMsgs = append(llmMsgs, openai.ChatCompletionMessageParamUnion{
				OfUser: &openai.ChatCompletionUserMessageParam{
					Content: openai.ChatCompletionUserMessageParamContentUnion{
						OfString: openai.String(fullTurnContent),
					},
				},
			})
		} else { // Other speaker's turn, context for LLM
			llmMsgs = append(llmMsgs, openai.ChatCompletionMessageParamUnion{
				OfAssistant: &openai.ChatCompletionAssistantMessageParam{
					Content: openai.ChatCompletionAssistantMessageParamContentUnion{
						OfString: openai.String(fullTurnContent),
					},
				},
			})
		}
	}

	if parsedTurnsCount == 0 {
		s.logger.Warnf("No valid turns found in conversation for speaker %s in conversation %s.", speakerID, convDoc.ID)
	}
	if len(llmMsgs) <= 1 { // Only system prompt
		s.logger.Warnf("llmMsgs only contains system prompt for speaker %s in conversation %s. No conversational turns added. Skipping LLM call for fact extraction.", speakerID, convDoc.ID)
		return []string{}, nil // No error, but no facts
	}

	s.logger.Debugf("Calling LLM for Fact Extraction (%s). Model: %s, Tools: %d tools", speakerID, openAIChatModel, len(factExtractionToolsList))

	llmResponse, err := s.completionsService.Completions(ctx, llmMsgs, factExtractionToolsList, openAIChatModel)
	if err != nil {
		s.logger.Errorf("LLM completion error during fact extraction for speaker %s in conversation %s: %v", speakerID, convDoc.ID, err)
		return nil, fmt.Errorf("LLM completion error for speaker %s, conversation %s: %w", speakerID, convDoc.ID, err)
	}

	var extractedFacts []string
	if len(llmResponse.ToolCalls) > 0 {
		for _, toolCall := range llmResponse.ToolCalls {
			if toolCall.Function.Name == ExtractFactsToolName {
				var args ExtractFactsToolArguments
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err == nil {
					extractedFacts = append(extractedFacts, args.Facts...)
					s.logger.Infof("Successfully parsed EXTRACT_FACTS tool call. Extracted %d facts for speaker %s from conversation %s.", len(args.Facts), speakerID, convDoc.ID)
				} else {
					s.logger.Warnf("Failed to unmarshal EXTRACT_FACTS arguments for speaker %s from conversation %s: %v. Arguments: %s", speakerID, convDoc.ID, err, toolCall.Function.Arguments)
				}
			} else {
				s.logger.Warnf("LLM called an unexpected tool '%s' during fact extraction for speaker %s from conversation %s.", toolCall.Function.Name, speakerID, convDoc.ID)
			}
		}
	} else {
		s.logger.Info("LLM response for fact extraction for speaker %s from conversation %s did not contain tool calls. No facts extracted by tool.", speakerID, convDoc.ID)
	}

	return extractedFacts, nil
}
