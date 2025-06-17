package whatsapp

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	openai "github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/pkg/ai"
	whatsappdb "github.com/EternisAI/enchanted-twin/pkg/db/sqlc/whatsapp"
)

type ConversationAnalyzer struct {
	logger    *log.Logger
	aiService *ai.Service
}

type ConversationAssessment struct {
	IsNewConversation bool    `json:"is_new_conversation"`
	Confidence        float64 `json:"confidence"`
	Reasoning         string  `json:"reasoning"`
}

func NewConversationAnalyzer(logger *log.Logger, aiService *ai.Service) *ConversationAnalyzer {
	return &ConversationAnalyzer{
		logger:    logger,
		aiService: aiService,
	}
}

func (ca *ConversationAnalyzer) AssessConversationBoundary(ctx context.Context, recentMessages []whatsappdb.WhatsappMessage, newMessage string, newSender string) (*ConversationAssessment, error) {
	if len(recentMessages) == 0 {
		return &ConversationAssessment{
			IsNewConversation: true,
			Confidence:        1.0,
			Reasoning:         "No previous messages exist, this is clearly a new conversation",
		}, nil
	}

	// Build context from recent messages
	var messageContext strings.Builder
	messageContext.WriteString("Recent messages in this conversation:\n")

	for i, msg := range recentMessages {
		timeAgo := "recently"
		if i < len(recentMessages)-1 {
			timeAgo = "earlier"
		}
		messageContext.WriteString(fmt.Sprintf("%s: %s (%s)\n", msg.SenderName, msg.Content, timeAgo))
	}

	messageContext.WriteString(fmt.Sprintf("\nNew message:\n%s: %s\n", newSender, newMessage))

	systemPrompt := `You are an expert at analyzing conversation flow and determining conversation boundaries.

Your task is to determine if a new message is:
1. Starting a NEW conversation (topic change, greeting after long gap, unrelated to previous messages)
2. Continuing the EXISTING conversation (reply, follow-up, related response)

Consider these factors:
- Topic continuity vs topic changes
- Natural conversation flow vs abrupt shifts
- Greetings that indicate new conversations
- Questions that reference previous messages (indicating continuation)
- Time-sensitive responses vs standalone statements

Respond in this exact format:
DECISION: [NEW_CONVERSATION or CONTINUING_CONVERSATION]
CONFIDENCE: [number between 0.0 and 1.0]
REASONING: [brief explanation of your assessment]

Be conservative - only mark as NEW_CONVERSATION when you're quite confident there's a clear boundary.`

	userPrompt := fmt.Sprintf(`Analyze this conversation context and determine if the new message starts a new conversation:

%s

Is the new message starting a new conversation or continuing the existing one?`, messageContext.String())

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
		openai.UserMessage(userPrompt),
	}

	params := openai.ChatCompletionNewParams{
		Messages: messages,
		Model:    "gpt-4o-mini",
	}

	response, err := ca.aiService.ParamsCompletions(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get AI assessment: %w", err)
	}

	content := response.Content
	ca.logger.Debug("AI conversation assessment", "response", content)

	// Parse the structured response
	assessment := &ConversationAssessment{
		Confidence: 0.5, // default
		Reasoning:  "AI assessment completed",
	}

	// Extract decision
	if strings.Contains(content, "DECISION: NEW_CONVERSATION") {
		assessment.IsNewConversation = true
	} else if strings.Contains(content, "DECISION: CONTINUING_CONVERSATION") {
		assessment.IsNewConversation = false
	}

	// Extract confidence
	if confStart := strings.Index(content, "CONFIDENCE: "); confStart != -1 {
		confStr := content[confStart+12:]
		if idx := strings.Index(confStr, "\n"); idx != -1 {
			confStr = confStr[:idx]
		}
		confStr = strings.TrimSpace(confStr)

		var confidence float64
		if _, err := fmt.Sscanf(confStr, "%f", &confidence); err == nil {
			assessment.Confidence = confidence
		}
	}

	// Extract reasoning
	if reasonStart := strings.Index(content, "REASONING: "); reasonStart != -1 {
		reasonStr := content[reasonStart+11:]
		if idx := strings.Index(reasonStr, "\n"); idx != -1 {
			reasonStr = reasonStr[:idx]
		}
		assessment.Reasoning = strings.TrimSpace(reasonStr)
	}

	ca.logger.Debug("Parsed conversation assessment",
		"is_new_conversation", assessment.IsNewConversation,
		"confidence", assessment.Confidence,
		"reasoning", assessment.Reasoning)

	return assessment, nil
}
