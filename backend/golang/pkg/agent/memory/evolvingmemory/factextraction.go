package evolvingmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/openai/openai-go"
)

// extractFactsFromTextDocument extracts facts for a given speaker from a text document.
func (s *WeaviateStorage) extractFactsFromTextDocument(ctx context.Context, sessionDoc memory.TextDocument, speakerID string, currentSystemDate string, docEventDateStr string) ([]string, error) {
	s.logger.Infof("== Starting Fact Extraction for Speaker: %s == (Session Doc ID: '%s')", speakerID, sessionDoc.ID)

	factExtractionToolsList := []openai.ChatCompletionToolParam{
		extractFactsTool, // This will now correctly refer to the one in tools.go
	}

	sysPrompt := strings.ReplaceAll(SpeakerFocusedFactExtractionPrompt, "{primary_speaker_name}", speakerID)
	sysPrompt = strings.ReplaceAll(sysPrompt, "{current_system_date}", currentSystemDate)
	sysPrompt = strings.ReplaceAll(sysPrompt, "{document_event_date}", docEventDateStr)

	llmMsgs := []openai.ChatCompletionMessageParamUnion{
		{
			OfSystem: &openai.ChatCompletionSystemMessageParam{
				Content: openai.ChatCompletionSystemMessageParamContentUnion{
					OfString: openai.String(sysPrompt),
				},
			},
		},
	}

	conversationLines := strings.Split(strings.TrimSpace(sessionDoc.Content), "\\n")
	parsedTurnsCount := 0
	for _, line := range conversationLines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}
		parts := strings.SplitN(trimmedLine, ":", 2)
		if len(parts) < 2 {
			s.logger.Warnf("Skipping malformed line (no speaker colon): '%s' for speaker %s in doc %s", trimmedLine, speakerID, sessionDoc.ID)
			continue
		}
		turnSpeaker := strings.TrimSpace(parts[0])
		turnText := strings.TrimSpace(parts[1])

		if turnText == "" {
			continue
		}

		parsedTurnsCount++
		fullTurnContent := fmt.Sprintf("%s: %s", turnSpeaker, turnText)

		if turnSpeaker == speakerID { // User's turn for the LLM context
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

	if parsedTurnsCount == 0 && len(conversationLines) > 0 {
		s.logger.Warnf("No valid turns were parsed from %d conversation lines for speaker %s in doc %s. LLM might not have sufficient context.", len(conversationLines), speakerID, sessionDoc.ID)
	}
	if len(llmMsgs) <= 1 { // Only system prompt
		s.logger.Warnf("llmMsgs only contains system prompt for speaker %s in doc %s. No conversational turns added. Skipping LLM call for fact extraction.", speakerID, sessionDoc.ID)
		return []string{}, nil // No error, but no facts
	}

	s.logger.Debugf("Calling LLM for Speaker-Focused Fact Extraction (%s). Model: %s, Tools: %d tools", speakerID, openAIChatModel, len(factExtractionToolsList))

	llmResponse, err := s.aiService.Completions(ctx, llmMsgs, factExtractionToolsList, openAIChatModel)
	if err != nil {
		s.logger.Errorf("LLM completion error during fact extraction for speaker %s in doc %s: %v", speakerID, sessionDoc.ID, err)
		return nil, fmt.Errorf("LLM completion error for speaker %s, doc %s: %w", speakerID, sessionDoc.ID, err)
	}

	var extractedFacts []string
	if len(llmResponse.ToolCalls) > 0 {
		for _, toolCall := range llmResponse.ToolCalls {
			if toolCall.Function.Name == ExtractFactsToolName {
				var args ExtractFactsToolArguments // Assumes this struct is defined in evolvingmemory.go
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err == nil {
					extractedFacts = append(extractedFacts, args.Facts...)
					s.logger.Infof("Successfully parsed EXTRACT_FACTS tool call. Extracted %d facts for speaker %s from doc %s.", len(args.Facts), speakerID, sessionDoc.ID)
				} else {
					s.logger.Warnf("Failed to unmarshal EXTRACT_FACTS arguments for speaker %s from doc %s: %v. Arguments: %s", speakerID, sessionDoc.ID, err, toolCall.Function.Arguments)
				}
			} else {
				s.logger.Warnf("LLM called an unexpected tool '%s' during fact extraction for speaker %s from doc %s.", toolCall.Function.Name, speakerID, sessionDoc.ID)
			}
		}
	} else {
		s.logger.Info("LLM response for fact extraction for speaker %s from doc %s did not contain tool calls. No facts extracted by tool.", speakerID, sessionDoc.ID)
	}

	return extractedFacts, nil
}
