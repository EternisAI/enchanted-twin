package plannedv2

import (
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
)

// MessageRole represents the role of a message.
type MessageRole string

const (
	MessageRoleSystem    MessageRole = "system"
	MessageRoleUser      MessageRole = "user"
	MessageRoleAssistant MessageRole = "assistant"
	MessageRoleTool      MessageRole = "tool"
)

// ToolCall represents a tool call in a serializable format.
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"` // Usually "function"
	Function ToolCallFunction `json:"function"`
	// Result of the tool execution (nil if not yet executed)
	Result *ToolResult `json:"result,omitempty"`
}

// ToolCallFunction represents a function call in a serializable format.
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// SystemMessageParams represents system message parameters.
type SystemMessageParams struct {
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

// UserMessageParams represents user message parameters.
type UserMessageParams struct {
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

// AssistantMessageParams represents assistant message parameters.
type AssistantMessageParams struct {
	Content   string     `json:"content,omitempty"`
	Name      string     `json:"name,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// ToolMessageParams represents tool message parameters.
type ToolMessageParams struct {
	Content    string `json:"content"`
	ToolCallID string `json:"tool_call_id"`
}

// Message represents a chat message in a format that serializes well.
type Message struct { // TODO: replace with ChatCompletionMessageParamUnion https://github.com/openai/openai-go/issues/247
	Role      MessageRole             `json:"role"`
	System    *SystemMessageParams    `json:"system,omitempty"`
	User      *UserMessageParams      `json:"user,omitempty"`
	Assistant *AssistantMessageParams `json:"assistant,omitempty"`
	Tool      *ToolMessageParams      `json:"tool,omitempty"`
}

// PlanState represents the unified state for planned agent execution.
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

	// Tool calls made in a serializable format
	ToolCalls []ToolCall `json:"tool_calls"`

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

// HistoryEntry represents a single entry in the agent's execution history.
type HistoryEntry struct {
	// Type of entry: "thought", "action", "observation", "system", "error"
	Type string `json:"type"`

	// Content of the entry
	Content string `json:"content"`

	// Timestamp when the entry was created
	Timestamp time.Time `json:"timestamp"`
}

// ActionRequest represents a request to execute a tool.
type ActionRequest struct {
	// Name of the tool to use
	Tool string `json:"tool"`

	// Parameters for the tool
	Params map[string]interface{} `json:"params"`
}

// ToolResult contains the result of tool execution.
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

// ToolDefinition represents a unified tool definition.
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

// PlanInput represents the input for the planned agent workflow.
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

// ToOpenAITool converts a ToolDefinition to OpenAI tool format.
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

// OpenAIToCustomToolCalls converts OpenAI tool calls to our custom format.
func OpenAIToCustomToolCalls(openaiToolCalls []openai.ChatCompletionMessageToolCall) []ToolCall {
	customToolCalls := make([]ToolCall, 0, len(openaiToolCalls))
	for _, tc := range openaiToolCalls {
		customToolCalls = append(customToolCalls, ToolCall{
			ID:   tc.ID,
			Type: string(tc.Type), // Convert from constant.Function to string
			Function: ToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		})
	}
	return customToolCalls
}

// CustomToOpenAIToolCalls converts our custom tool calls to OpenAI format.
func CustomToOpenAIToolCalls(
	customToolCalls []ToolCall,
) []openai.ChatCompletionMessageToolCallParam {
	openaiToolCalls := make([]openai.ChatCompletionMessageToolCallParam, 0, len(customToolCalls))
	for _, tc := range customToolCalls {
		// Create a new tool call without explicitly setting the Type field
		// This allows the OpenAI library to handle it internally
		toolCall := openai.ChatCompletionMessageToolCallParam{
			ID: tc.ID,
			Function: openai.ChatCompletionMessageToolCallFunctionParam{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
		openaiToolCalls = append(openaiToolCalls, toolCall)
	}
	return openaiToolCalls
}

// SystemMessage creates a system message.
func SystemMessage(content string) Message {
	return Message{
		Role: MessageRoleSystem,
		System: &SystemMessageParams{
			Content: content,
		},
	}
}

// UserMessage creates a user message.
func UserMessage(content string) Message {
	return Message{
		Role: MessageRoleUser,
		User: &UserMessageParams{
			Content: content,
		},
	}
}

// AssistantMessage creates an assistant message with optional tool calls.
func AssistantMessage(content string, toolCalls []openai.ChatCompletionMessageToolCall) Message {
	return Message{
		Role: MessageRoleAssistant,
		Assistant: &AssistantMessageParams{
			Content:   content,
			ToolCalls: OpenAIToCustomToolCalls(toolCalls),
		},
	}
}

// ToolMessage creates a tool message.
func ToolMessage(content, toolCallID string) Message {
	return Message{
		Role: MessageRoleTool,
		Tool: &ToolMessageParams{
			Content:    content,
			ToolCallID: toolCallID,
		},
	}
}

// ToOpenAIMessages converts our simplified message format to OpenAI's format.
func ToOpenAIMessages(messages []Message) []openai.ChatCompletionMessageParamUnion {
	result := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))

	for _, msg := range messages {
		// var oaiMsg openai.ChatCompletionMessageParamUnion  // can append to result on success

		switch msg.Role {
		case MessageRoleSystem:
			if msg.System != nil {
				// Create OpenAI system message from our custom type
				sysMsg := openai.SystemMessage(msg.System.Content)
				if msg.System.Name != "" {
					sysMsg.OfSystem.Name = param.NewOpt(msg.System.Name)
				}
				result = append(result, sysMsg)
			}
		case MessageRoleUser:
			if msg.User != nil {
				// Create OpenAI user message from our custom type
				userMsg := openai.UserMessage(msg.User.Content)
				if msg.User.Name != "" {
					userMsg.OfUser.Name = param.NewOpt(msg.User.Name)
				}
				result = append(result, userMsg)
			}
		case MessageRoleAssistant:
			if msg.Assistant != nil {
				// Create OpenAI assistant message from our custom type
				assistantMsg := openai.AssistantMessage(msg.Assistant.Content)

				// Add tool calls if present
				if len(msg.Assistant.ToolCalls) > 0 {
					// Convert our stored tool calls to OpenAI param format
					tcsParams := CustomToOpenAIToolCalls(msg.Assistant.ToolCalls)

					assistantMsg.OfAssistant.ToolCalls = tcsParams
				}

				// Add name if present
				if msg.Assistant.Name != "" {
					assistantMsg.OfAssistant.Name = param.NewOpt(msg.Assistant.Name)
				}

				result = append(result, assistantMsg)
			}
		case MessageRoleTool:
			if msg.Tool != nil {
				// Create OpenAI tool message from our custom type
				toolMsg := openai.ToolMessage(msg.Tool.Content, msg.Tool.ToolCallID)
				result = append(result, toolMsg)
			}
		}
	}

	return result
}
