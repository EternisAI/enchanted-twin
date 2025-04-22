package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/log"

	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/nats-io/nats.go"
	"github.com/openai/openai-go"
)

const MAX_STEPS = 10

type Agent struct {
	logger           *log.Logger
	nc               *nats.Conn
	aiService        *ai.Service
	CompletionsModel string
	PreToolCallback  func(toolCall openai.ChatCompletionMessageToolCall)
	PostToolCallback func(toolCall openai.ChatCompletionMessageToolCall, toolResult tools.ToolResult)
}

func NewAgent(logger *log.Logger, nc *nats.Conn, aiService *ai.Service, completionsModel string, preToolCallback func(toolCall openai.ChatCompletionMessageToolCall), postToolCallback func(toolCall openai.ChatCompletionMessageToolCall, toolResult tools.ToolResult)) *Agent {
	return &Agent{
		logger:           logger,
		nc:               nc,
		aiService:        aiService,
		CompletionsModel: completionsModel,
		PreToolCallback:  preToolCallback,
		PostToolCallback: postToolCallback,
	}
}

type AgentResponse struct {
	Content     string
	ToolCalls   []openai.ChatCompletionMessageToolCall
	ToolResults []tools.ToolResult
	ImageURLs   []string
}

func (a *Agent) Execute(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, currentTools []tools.Tool) (AgentResponse, error) {
	currentStep := 0
	responseContent := ""
	toolCalls := make([]openai.ChatCompletionMessageToolCall, 0)
	toolResults := make([]tools.ToolResult, 0)
	imageURLs := make([]string, 0)

	apiToolDefinitions := make([]openai.ChatCompletionToolParam, 0)

	toolsMap := make(map[string]tools.Tool, 0)
	for _, tool := range currentTools {
		toolsMap[tool.Definition().Function.Name] = tool
		apiToolDefinitions = append(apiToolDefinitions, tool.Definition())
	}

	for currentStep < MAX_STEPS {
		completion, err := a.aiService.Completions(ctx, messages, apiToolDefinitions, a.CompletionsModel)
		if err != nil {
			return AgentResponse{}, err
		}

		messages = append(messages, completion.ToParam())

		if len(completion.ToolCalls) == 0 {
			return AgentResponse{
				Content:     completion.Content,
				ToolCalls:   toolCalls,
				ToolResults: toolResults,
				ImageURLs:   imageURLs,
			}, nil
		}

		for _, toolCall := range completion.ToolCalls {
			if a.PreToolCallback != nil {
				a.logger.Debug("Pre tool callback", "tool_call", toolCall)
				a.PreToolCallback(toolCall)
			}
		}
		// we send message with tool call
		for _, toolCall := range completion.ToolCalls {
			// we send message with tool call
			tool, ok := toolsMap[toolCall.Function.Name]
			if !ok {
				return AgentResponse{}, fmt.Errorf("tool not found: %s", toolCall.Function.Name)
			}

			var args map[string]any
			err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
			if err != nil {
				return AgentResponse{}, err
			}

			a.logger.Debug("Executing tool", "tool", toolCall.Function.Name, "args", args)
			toolResult, err := tool.Execute(ctx, args)

			if err != nil {
				return AgentResponse{}, err
			}

			if len(toolResult.ImageURLs) > 0 {
				imageURLs = append(imageURLs, toolResult.ImageURLs...)
			}

			// send message with isCompleted true
			if a.PostToolCallback != nil {
				a.logger.Debug("Post tool callback", "tool_call", toolCall, "tool_result", toolResult)
				a.PostToolCallback(toolCall, toolResult)
			}
			messages = append(messages, openai.ToolMessage(toolResult.Content, toolCall.ID))
			toolCalls = append(toolCalls, toolCall)
			toolResults = append(toolResults, toolResult)
		}

		responseContent = completion.Content
		currentStep++
	}

	return AgentResponse{
		Content:     responseContent,
		ToolCalls:   toolCalls,
		ToolResults: toolResults,
		ImageURLs:   imageURLs,
	}, nil
}
