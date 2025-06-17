package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"

	"github.com/EternisAI/enchanted-twin/pkg/ai"
	whatsappdb "github.com/EternisAI/enchanted-twin/pkg/db/sqlc/whatsapp"
)

type ConversationAnalyzer struct {
	logger    *log.Logger
	aiService *ai.Service
	model     string
}

type ConversationAssessment struct {
	IsNewConversation bool    `json:"is_new_conversation"`
	Confidence        float64 `json:"confidence"`
	Reasoning         string  `json:"reasoning"`
}

// conversationAnalysisTool defines the proper OpenAI tool for conversation analysis.
var conversationAnalysisTool = openai.ChatCompletionToolParam{
	Type: "function",
	Function: openai.FunctionDefinitionParam{
		Name:        "analyze_conversation_boundary",
		Description: param.NewOpt("Analyze whether a new message starts a new conversation or continues an existing one"),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"is_new_conversation": map[string]any{
					"type":        "boolean",
					"description": "Whether the new message starts a new conversation (true) or continues the existing one (false)",
				},
				"confidence": map[string]any{
					"type":        "number",
					"minimum":     0.0,
					"maximum":     1.0,
					"description": "Confidence level in the assessment (0.0 to 1.0)",
				},
				"reasoning": map[string]any{
					"type":        "string",
					"description": "Brief explanation of the assessment decision",
				},
			},
			"required":             []string{"is_new_conversation", "confidence", "reasoning"},
			"additionalProperties": false,
		},
	},
}

func NewConversationAnalyzer(logger *log.Logger, aiService *ai.Service, model string) *ConversationAnalyzer {
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &ConversationAnalyzer{
		logger:    logger,
		aiService: aiService,
		model:     model,
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

Be conservative - only mark as NEW_CONVERSATION when you're quite confident there's a clear boundary.`

	userPrompt := fmt.Sprintf(`Analyze this conversation context and determine if the new message starts a new conversation:

%s

Use the analyze_conversation_boundary tool to provide your assessment.`, messageContext.String())

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
		openai.UserMessage(userPrompt),
	}

	response, err := ca.aiService.Completions(ctx, messages, []openai.ChatCompletionToolParam{conversationAnalysisTool}, ca.model)
	if err != nil {
		return nil, fmt.Errorf("failed to get AI assessment: %w", err)
	}

	ca.logger.Debug("AI conversation assessment", "response", response)

	if len(response.ToolCalls) == 0 {
		return nil, fmt.Errorf("no tool calls returned from AI")
	}

	toolCall := response.ToolCalls[0]
	if toolCall.Function.Name != "analyze_conversation_boundary" {
		return nil, fmt.Errorf("unexpected tool call: %s", toolCall.Function.Name)
	}

	var assessment ConversationAssessment
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &assessment); err != nil {
		return nil, fmt.Errorf("failed to parse tool response: %w", err)
	}

	ca.logger.Debug("Parsed conversation assessment",
		"is_new_conversation", assessment.IsNewConversation,
		"confidence", assessment.Confidence,
		"reasoning", assessment.Reasoning)

	return &assessment, nil
}
