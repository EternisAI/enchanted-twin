package plannedv2

import (
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
)

// MessageRole represents the role of a message
type MessageRole string

const (
	MessageRoleSystem    MessageRole = "system"
	MessageRoleUser      MessageRole = "user"
	MessageRoleAssistant MessageRole = "assistant"
	MessageRoleTool      MessageRole = "tool"
)

// Message represents a chat message in a format that serializes well
type Message struct { // TODO: replace with ChatCompletionMessageParamUnion https://github.com/openai/openai-go/issues/247
	Role      MessageRole                                 `json:"role"`
	System    *openai.ChatCompletionSystemMessageParam    `json:"system,omitempty"`
	User      *openai.ChatCompletionUserMessageParam      `json:"user,omitempty"`
	Assistant *openai.ChatCompletionAssistantMessageParam `json:"assistant,omitempty"`
	Tool      *openai.ChatCompletionToolMessageParam      `json:"tool,omitempty"`
}

// PlanState represents the unified state for planned agent execution
type PlanState struct {
	// The original plan text
	Plan string `json:"plan"`

	// Current execution progress
	CurrentStep int `json:"current_step"`

	// Flag indicating if execution is complete
	Complete bool `json:"complete"`

	// Final output when plan is complete
	Output string `json:"output"`

	// Error message if execution failed
	Error string `json:"error,omitempty"`

	// Message history in serializable format
	Messages []Message `json:"messages"`

	// Tool calls made (OpenAI format - only used for API compatibility)
	ToolCalls []openai.ChatCompletionMessageToolCall `json:"tool_calls"`

	// Tool results (results from executed tools)
	ToolResults []ToolResult `json:"tool_results"`

	// Typed history entries (for structured logging and UI)
	History []HistoryEntry `json:"history"`

	// Available tools for the agent
	Tools []ToolDefinition `json:"tools"`

	// Image URLs generated (if any)
	ImageURLs []string `json:"image_urls"`

	// Execution metadata
	StartTime time.Time `json:"start_time"`
}

// HistoryEntry represents a single entry in the agent's execution history
type HistoryEntry struct {
	// Type of entry: "thought", "action", "observation", "system", "error"
	Type string `json:"type"`

	// Content of the entry
	Content string `json:"content"`

	// Timestamp when the entry was created
	Timestamp time.Time `json:"timestamp"`
}

// ActionRequest represents a request to execute a tool
type ActionRequest struct {
	// Name of the tool to use
	Tool string `json:"tool"`

	// Parameters for the tool
	Params map[string]interface{} `json:"params"`
}

// ToolResult contains the result of tool execution
type ToolResult struct {
	// Name of the tool that was executed
	Tool string `json:"tool"`

	// Parameters used for the execution
	Params map[string]interface{} `json:"params"`

	// Content result from the tool
	Content string `json:"content"`

	// Structured result data (if any)
	Data interface{} `json:"data,omitempty"`

	// Image URLs produced (if any)
	ImageURLs []string `json:"image_urls,omitempty"`

	// Error message (if execution failed)
	Error string `json:"error,omitempty"`
}

// ToolDefinition represents a unified tool definition
type ToolDefinition struct {
	// Name of the tool
	Name string `json:"name"`

	// Description of what the tool does
	Description string `json:"description"`

	// Tool parameters schema (JSON Schema)
	Parameters map[string]interface{} `json:"parameters"`

	// Entrypoint details
	Entrypoint types.ToolDefEntrypoint `json:"entrypoint"`

	// Return schema (if applicable)
	Returns map[string]interface{} `json:"returns,omitempty"`
}

// PlanInput represents the input for the planned agent workflow
type PlanInput struct {
	// The plan text that the agent should follow
	Plan string `json:"plan"`

	// List of tool names that the agent can use
	ToolNames []string `json:"tool_names,omitempty"`

	// LLM model to use for completions
	Model string `json:"model"`

	// Maximum number of steps to execute
	MaxSteps int `json:"max_steps"`

	// System prompt to use (optional)
	SystemPrompt string `json:"system_prompt,omitempty"`
}

// ToOpenAITool converts a ToolDefinition to OpenAI tool format
func (t ToolDefinition) ToOpenAITool() openai.ChatCompletionToolParam {
	// Convert our tool definition to OpenAI format
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        t.Name,
			Description: param.NewOpt(t.Description),
			Parameters:  t.Parameters,
		},
	}
}

// SystemMessage creates a system message
func SystemMessage(content string) Message {
	sysMsg := openai.SystemMessage(content)
	systemParam := sysMsg.OfSystem
	return Message{
		Role:   MessageRoleSystem,
		System: systemParam,
	}
}

// UserMessage creates a user message
func UserMessage(content string) Message {
	userMsg := openai.UserMessage(content)
	userParam := userMsg.OfUser
	return Message{
		Role: MessageRoleUser,
		User: userParam,
	}
}

// AssistantMessageWithToolCalls creates an assistant message with tool calls
func AssistantMessage(content string, toolCalls []openai.ChatCompletionMessageToolCall) Message {
	// Create a base assistant message
	assistantMsg := openai.AssistantMessage(content)
	assistantParam := assistantMsg.OfAssistant

	// Convert toolCalls to format expected by OpenAI
	for _, tc := range toolCalls {
		assistantParam.ToolCalls = append(assistantParam.ToolCalls, tc.ToParam())
	}

	return Message{
		Role:      MessageRoleAssistant,
		Assistant: assistantParam,
	}
}

// ToolMessage creates a tool message
func ToolMessage(content, toolCallID string) Message {
	toolMsg := openai.ToolMessage(content, toolCallID)
	toolParam := toolMsg.OfTool
	return Message{
		Role: MessageRoleTool,
		Tool: toolParam,
	}
}

// ToOpenAIMessages converts our simplified message format to OpenAI's format
func ToOpenAIMessages(messages []Message) []openai.ChatCompletionMessageParamUnion {
	result := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))

	for _, msg := range messages {
		switch msg.Role {
		case MessageRoleSystem:
			if msg.System != nil {
				systemParam := *msg.System
				union := openai.ChatCompletionMessageParamUnion{}
				union.OfSystem = &systemParam
				result = append(result, union)
			}
		case MessageRoleUser:
			if msg.User != nil {
				userParam := *msg.User
				union := openai.ChatCompletionMessageParamUnion{}
				union.OfUser = &userParam
				result = append(result, union)
			}
		case MessageRoleAssistant:
			if msg.Assistant != nil {
				assistantParam := *msg.Assistant
				union := openai.ChatCompletionMessageParamUnion{}
				union.OfAssistant = &assistantParam
				result = append(result, union)
			}
		case MessageRoleTool:
			if msg.Tool != nil {
				toolParam := *msg.Tool
				union := openai.ChatCompletionMessageParamUnion{}
				union.OfTool = &toolParam
				result = append(result, union)
			}
		}
	}

	return result
}
