package evolvingmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

// extractFactsFromTextDocument extracts facts for a given speaker from a text document.
func (s *WeaviateStorage) extractFactsFromTextDocument(ctx context.Context, sessionDoc memory.TextDocument, speakerID string, currentSystemDate string, docEventDateStr string) ([]string, error) {
	// s.logger.Infof("== Starting Fact Extraction for Speaker: %s == (Session Doc ID: '%s')", speakerID, sessionDoc.ID)
	s.logger.Infof("Document content for fact extraction: %s", sessionDoc.Content)

	factExtractionToolsList := []openai.ChatCompletionToolParam{
		extractFactsTool,
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

	s.logger.Infof("Parsing conversation lines from document content...")
	conversationLines := strings.Split(strings.TrimSpace(sessionDoc.Content), "\n")
	s.logger.Infof("Split document into %d conversation lines", len(conversationLines))

	parsedTurnsCount := 0
	for i, line := range conversationLines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			s.logger.Debugf("Skipping empty line %d", i)
			continue
		}

		parts := strings.SplitN(trimmedLine, ":", 2)
		var turnSpeaker, turnText string

		if len(parts) >= 2 {

			turnSpeaker = strings.TrimSpace(parts[0])
			turnText = strings.TrimSpace(parts[1])
		} else {
			if fromSpeaker, exists := sessionDoc.Metadata["from"]; exists && fromSpeaker != "" {
				turnSpeaker = fromSpeaker
				turnText = trimmedLine
				s.logger.Debugf("Using metadata speaker '%s' for line without colon: '%s'", turnSpeaker, trimmedLine)
			} else {
				if speakerID != "" {
					turnSpeaker = speakerID
					turnText = trimmedLine

				} else {
					s.logger.Warnf("Skipping malformed line (no speaker colon and no metadata): '%s' for speaker %s in doc %s", trimmedLine, speakerID, sessionDoc.ID)
					continue
				}
			}
		}

		s.logger.Debugf("Parsed speaker: '%s', text: '%s'", turnSpeaker, turnText)

		if turnText == "" {
			s.logger.Debugf("Skipping line with empty text")
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
		} else {
			s.logger.Debugf("Adding assistant message for speaker %s", turnSpeaker)
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
	if len(llmMsgs) <= 1 {
		s.logger.Warnf("llmMsgs only contains system prompt for speaker %s in doc %s. No conversational turns added. Skipping LLM call for fact extraction.", speakerID, sessionDoc.ID)
		return []string{}, nil
	}

	s.logger.Debugf("Calling LLM for Speaker-Focused Fact Extraction (%s). Model: %s, Tools: %d tools", speakerID, openAIChatModel, len(factExtractionToolsList))

	llmResponse, err := s.completionsService.Completions(ctx, llmMsgs, factExtractionToolsList, openAIChatModel)
	if err != nil {
		s.logger.Errorf("LLM completion error during fact extraction for speaker %s in doc %s: %v", speakerID, sessionDoc.ID, err)
		return nil, fmt.Errorf("LLM completion error for speaker %s, doc %s: %w", speakerID, sessionDoc.ID, err)
	}

	s.logger.Infof("LLM response received for speaker %s. Tool calls: %d", speakerID, len(llmResponse.ToolCalls))
	if len(llmResponse.ToolCalls) == 0 {
		s.logger.Infof("LLM response content (no tool calls): %s", llmResponse.Content)
	}

	var extractedFacts []string
	if len(llmResponse.ToolCalls) > 0 {
		for i, toolCall := range llmResponse.ToolCalls {
			s.logger.Infof("Processing tool call %d: %s", i+1, toolCall.Function.Name)
			if toolCall.Function.Name == ExtractFactsToolName {
				s.logger.Infof("EXTRACT_FACTS tool call arguments: %s", toolCall.Function.Arguments)
				var args ExtractFactsToolArguments // Assumes this struct is defined in evolvingmemory.go
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err == nil {
					extractedFacts = append(extractedFacts, args.Facts...)
					s.logger.Infof("Successfully parsed EXTRACT_FACTS tool call. Extracted %d facts for speaker %s from doc %s.", len(args.Facts), speakerID, sessionDoc.ID)
					for j, fact := range args.Facts {
						s.logger.Infof("Extracted fact %d: %s", j+1, fact)
					}
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

	s.logger.Infof("Fact extraction completed for speaker %s. Total facts extracted: %d", speakerID, len(extractedFacts))
	return extractedFacts, nil
}
