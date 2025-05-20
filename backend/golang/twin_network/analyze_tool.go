package twin_network

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"

	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

// AnalyzeNetworkMessageTool implements a tool that analyses a network message
// and optionally suggests a reply.

type ReadNetworkTool struct {
	AI               *ai.Service
	Logger           *log.Logger
	CompletionsModel string
}

// NewReadNetworkTool constructs a new tool instance.
func NewReadNetworkTool(logger *log.Logger, aiService *ai.Service, completionsModel string) *ReadNetworkTool {
	return &ReadNetworkTool{
		AI:               aiService,
		Logger:           logger,
		CompletionsModel: completionsModel,
	}
}

// Definition exposes the JSON schema for the function-calling interface.
func (t *ReadNetworkTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        "Read_Network",
			Description: param.NewOpt("Analyzes a messages received from the twin network and provides reasoning. Optionally suggest a response."),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"reasoning": map[string]string{
						"type":        "string",
						"description": "The reasoning for the analysis of the network message.",
					},
					"response": map[string]string{
						"type":        "string",
						"description": "Response message.",
					},
				},
				"required": []string{"reasoning"},
			},
		},
	}
}

// Execute performs the actual analysis using the configured ai.Service.
func (t *ReadNetworkTool) Execute(ctx context.Context, inputs map[string]any) (types.ToolResult, error) {
	rawMsg, ok := inputs["network_message"]
	if !ok {
		return nil, errors.New("network_message is required")
	}

	type Message struct {
		Role    string
		Content string
	}

	var messages []Message
	switch v := rawMsg.(type) {
	case string:
		messages = []Message{{Role: "user", Content: v}}
	case []string:
		for _, msg := range v {
			messages = append(messages, Message{Role: "user", Content: msg})
		}
	case []map[string]string:
		for _, msg := range v {
			role, ok1 := msg["role"]
			content, ok2 := msg["content"]
			if ok1 && ok2 {
				messages = append(messages, Message{Role: role, Content: content})
			}
		}
	case []interface{}:
		for _, msg := range v {
			if m, ok := msg.(map[string]interface{}); ok {
				role, _ := m["role"].(string)
				content, _ := m["content"].(string)
				if role != "" && content != "" {
					messages = append(messages, Message{Role: role, Content: content})
				}
			}
		}
	default:
		return nil, errors.New("network_message must be a string, array of strings, or array of message objects")
	}

	if len(messages) == 0 {
		return nil, errors.New("no messages provided")
	}

	if t.Logger != nil {
		t.Logger.Debug("Analyzing network messages", "count", len(messages))
	}

	reasoning := ""

	if t.AI != nil {
		// Format messages for the AI
		formattedMessages := "Messages in the conversation:\n"
		for i, msg := range messages {
			formattedMessages += fmt.Sprintf("%d. [%s] %s\n", i+1, msg.Role, msg.Content)
		}

		messages := []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("You are an expert AI analysing messages exchanged between users and assistants in the twin network. Analyze the conversation and provide your analysis in two parts:\n1. Reasoning: Your analysis of the conversation flow, message patterns, and the roles of each participant\n2. Response: A suggested next response that would be appropriate in this context\nFormat your response exactly like this:\nReasoning: [your reasoning here]\nResponse: [your response here]"),
			openai.UserMessage(formattedMessages),
		}

		completion, err := t.AI.Completions(ctx, messages, nil, t.CompletionsModel)
		if err != nil {
			return nil, err
		}

		// Parse the completion into reasoning and response
		content := completion.Content
		reasoningPart := "Reasoning: "
		responsePart := "Response: "

		reasoningStart := strings.Index(content, reasoningPart)
		responseStart := strings.Index(content, responsePart)

		if reasoningStart != -1 && responseStart != -1 {
			reasoningEnd := responseStart
			reasoning = strings.TrimSpace(content[reasoningStart+len(reasoningPart) : reasoningEnd])
			response := strings.TrimSpace(content[responseStart+len(responsePart):])

			output := map[string]any{
				"reasoning": reasoning,
				"response":  response,
			}

			return &types.StructuredToolResult{
				ToolName:   "Read_Network",
				ToolParams: inputs,
				Output:     output,
			}, nil
		}

		reasoning = content
	} else {
		reasoning = "AI service unavailable to analyse message."
	}

	output := map[string]any{
		"reasoning": reasoning,
		"response":  "Unable to generate response.",
	}

	return &types.StructuredToolResult{
		ToolName:   "Read_Network",
		ToolParams: inputs,
		Output:     output,
	}, nil
}
