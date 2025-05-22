// Owner: august@eternis.ai
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/nats-io/nats.go"
	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

const MAX_STEPS = 10

type Agent struct {
	logger           *log.Logger
	nc               *nats.Conn
	aiService        *ai.Service
	CompletionsModel string
	ReasoningModel   string
	PreToolCallback  func(toolCall openai.ChatCompletionMessageToolCall)
	PostToolCallback func(toolCall openai.ChatCompletionMessageToolCall, toolResult types.ToolResult)
}

func NewAgent(
	logger *log.Logger,
	nc *nats.Conn,
	aiService *ai.Service,
	completionsModel string,
	reasoningModel string,
	preToolCallback func(toolCall openai.ChatCompletionMessageToolCall),
	postToolCallback func(toolCall openai.ChatCompletionMessageToolCall, toolResult types.ToolResult),
) *Agent {
	return &Agent{
		logger:           logger,
		nc:               nc,
		aiService:        aiService,
		CompletionsModel: completionsModel,
		ReasoningModel:   reasoningModel,
		PreToolCallback:  preToolCallback,
		PostToolCallback: postToolCallback,
	}
}

type AgentResponse struct {
	Content     string
	ToolCalls   []openai.ChatCompletionMessageToolCall
	ToolResults []types.ToolResult
	ImageURLs   []string
}

func (r AgentResponse) String() string {
	imageURLS := strings.Join(r.ImageURLs, ",")
	if len(imageURLS) > 0 {
		return fmt.Sprintf("%s\nImages:%s", r.Content, imageURLS)
	}
	return r.Content
}

func (a *Agent) Execute(
	ctx context.Context,
	origin map[string]any,
	messages []openai.ChatCompletionMessageParamUnion,
	currentTools []tools.Tool,
) (AgentResponse, error) {
	a.logger.Info("-----------------------------------")
	a.logger.Info("-----------------------------------")
	a.logger.Info("Executing agent")
	a.logger.Info("Messages", "messages", messages)
	currentStep := 0
	responseContent := ""
	toolCalls := make([]openai.ChatCompletionMessageToolCall, 0)
	toolResults := make([]types.ToolResult, 0)
	imageURLs := make([]string, 0)

	apiToolDefinitions := make([]openai.ChatCompletionToolParam, 0)

	toolsMap := make(map[string]tools.Tool, 0)
	for _, tool := range currentTools {
		toolsMap[tool.Definition().Function.Name] = tool
		apiToolDefinitions = append(apiToolDefinitions, tool.Definition())
	}

	for currentStep < MAX_STEPS {
		completion, err := a.aiService.Completions(
			ctx,
			messages,
			apiToolDefinitions,
			a.CompletionsModel,
		)
		if err != nil {
			a.logger.Error("Error completing", "error", err)
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
			a.logger.Debug("Pre tool callback", "tool_call", toolCall)
			if a.PreToolCallback != nil {
				a.PreToolCallback(toolCall)
			}
		}

		for _, toolCall := range completion.ToolCalls {
			tool, ok := toolsMap[toolCall.Function.Name]
			if !ok {
				return AgentResponse{}, fmt.Errorf("tool not found: %s", toolCall.Function.Name)
			}

			var args map[string]any
			err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
			if err != nil {
				a.logger.Error("Error unmarshalling tool call arguments", "name", toolCall.Function.Name, "args", toolCall.Function.Arguments, "error", err)
				return AgentResponse{}, err
			}

			// Add extra logging for thread_id in send_to_twin_network tool
			if toolCall.Function.Name == "send_to_twin_network" {
				if threadID, ok := args["thread_id"].(string); ok {
					a.logger.Info("Thread ID being used for twin network", "thread_id", threadID, "has_hash_prefix", strings.HasPrefix(threadID, "#"))
				} else {
					a.logger.Warn("No thread_id found in send_to_twin_network call")
				}
			}

			toolResult, err := tool.Execute(ctx, args)
			if err != nil {
				a.logger.Error("Error executing tool", "name", toolCall.Function.Name, "args", args, "error", err)
				return AgentResponse{}, err
			}

			a.logger.Debug("Post tool callback", "tool_call", toolCall, "result", toolResult)
			if a.PostToolCallback != nil {
				a.PostToolCallback(toolCall, toolResult)
			}

			resultImageURLs := toolResult.ImageURLs()
			if len(resultImageURLs) > 0 {
				imageURLs = append(imageURLs, resultImageURLs...)
			}

			messages = append(messages, openai.ToolMessage(toolResult.Content(), toolCall.ID))

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
