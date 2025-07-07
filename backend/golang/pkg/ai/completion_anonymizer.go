package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"
)

type CompletionAnonymizer struct {
	completionService Completions
	logger            *log.Logger
	model             string
}

func NewCompletionAnonymizer(completionService Completions, logger *log.Logger, model string) *CompletionAnonymizer {
	return &CompletionAnonymizer{
		completionService: completionService,
		logger:            logger,
		model:             model,
	}
}

func (a *CompletionAnonymizer) Anonymize(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion) AnonymizeResult {
	a.logger.Info("Starting completion-based anonymization", "message_count", len(messages))

	if len(messages) == 0 {
		a.logger.Warn("No messages to anonymize")
		return AnonymizeResult{
			Messages: messages,
			Success:  true,
			Error:    nil,
		}
	}

	conversationText := a.extractConversationText(messages)
	if conversationText == "" {
		a.logger.Debug("No text content found to anonymize")
		return AnonymizeResult{
			Messages: messages,
			Success:  true,
			Error:    nil,
		}
	}

	anonymizedText, err := a.anonymizeText(ctx, conversationText)
	if err != nil {
		a.logger.Error("Failed to anonymize conversation text", "error", err)
		return AnonymizeResult{
			Messages: messages,
			Success:  false,
			Error:    fmt.Errorf("failed to anonymize conversation: %w", err),
		}
	}

	a.logger.Info("Successfully anonymized conversation text", "original_length", len(conversationText), "anonymized_length", len(anonymizedText))

	return AnonymizeResult{
		Messages: messages,
		Success:  true,
		Error:    nil,
	}
}

func (a *CompletionAnonymizer) extractConversationText(messages []openai.ChatCompletionMessageParamUnion) string {
	var textParts []string

	for _, message := range messages {
		if messageJSON, err := message.MarshalJSON(); err == nil {
			var msgMap map[string]interface{}
			if json.Unmarshal(messageJSON, &msgMap) == nil {
				if content, ok := msgMap["content"].(string); ok && content != "" {
					textParts = append(textParts, content)
				}
			}
		}
	}

	return strings.Join(textParts, "\n")
}

func (a *CompletionAnonymizer) anonymizeText(ctx context.Context, text string) (string, error) {
	if strings.TrimSpace(text) == "" {
		return text, nil
	}

	anonymizationPrompt := fmt.Sprintf(`Please anonymize the following text by replacing any personally identifiable information (PII) with generic placeholders. Replace names with [NAME], email addresses with [EMAIL], phone numbers with [PHONE], addresses with [ADDRESS], and other sensitive information with appropriate placeholders. Keep the meaning and context intact.

Text to anonymize:
%s

Anonymized text:`, text)

	anonymizationMessages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage("You are an expert at anonymizing text while preserving its meaning and context."),
		openai.UserMessage(anonymizationPrompt),
	}

	a.logger.Debug("Sending anonymization request to completion service", "text_length", len(text))

	completion, err := a.completionService.Completions(ctx, anonymizationMessages, nil, a.model)
	if err != nil {
		return "", fmt.Errorf("completion service anonymization failed: %w", err)
	}

	anonymizedText := completion.Content
	if anonymizedText == "" {
		return "", fmt.Errorf("completion service returned empty anonymized text")
	}

	a.logger.Debug("Successfully anonymized text", "original_length", len(text), "anonymized_length", len(anonymizedText))
	return anonymizedText, nil
}
