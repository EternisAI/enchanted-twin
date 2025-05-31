// Owner: august@eternis.ai
package ai

import (
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

// TODO: replace with openai.ChatCompletionMessageParamUnion once https://github.com/openai/openai-go/issues/247 gets resolved.
type Message struct {
	Role       MessageRole `json:"role"`
	Content    string      `json:"content,omitempty"`
	Name       string      `json:"name,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

// ToolCall represents a tool call in a serializable format.
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"` // Usually "function"
	Function ToolCallFunction `json:"function"`
}

// ToOpenAIToolCallParam converts the ToolCall to OpenAI format.
func (tc ToolCall) ToOpenAIToolCallParam() openai.ChatCompletionMessageToolCallParam {
	return openai.ChatCompletionMessageToolCallParam{
		ID: tc.ID,
		Function: openai.ChatCompletionMessageToolCallFunctionParam{
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		},
	}
}

// FromOpenAIToolCall creates a ToolCall from an OpenAI tool call.
func FromOpenAIToolCall(tc openai.ChatCompletionMessageToolCall) ToolCall {
	return ToolCall{
		ID:   tc.ID,
		Type: string(tc.Type),
		Function: ToolCallFunction{
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		},
	}
}

// Note: ChatCompletionMessage will always be an assistant message.
func FromOpenAIMessage(msg openai.ChatCompletionMessage) Message {
	// Convert any tool calls
	toolCalls := make([]ToolCall, 0, len(msg.ToolCalls))
	for _, tc := range msg.ToolCalls {
		toolCalls = append(toolCalls, FromOpenAIToolCall(tc))
	}
	return NewAssistantMessage(msg.Content, toolCalls)
}

// ToolCallFunction represents a function call in a serializable format.
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// NewAssistantMessage creates an assistant message.
func NewAssistantMessage(content string, toolCalls []ToolCall) Message {
	return Message{
		Role:      MessageRoleAssistant,
		Content:   content,
		ToolCalls: toolCalls,
	}
}

// ToOpenAIMessage converts a Message to OpenAI format.
func (m Message) ToOpenAIMessage() openai.ChatCompletionMessageParamUnion {
	switch m.Role {
	case MessageRoleSystem:
		msg := openai.SystemMessage(m.Content)
		if m.Name != "" {
			msg.OfSystem.Name = param.NewOpt(m.Name)
		}
		return msg

	case MessageRoleUser:
		msg := openai.UserMessage(m.Content)
		if m.Name != "" {
			msg.OfUser.Name = param.NewOpt(m.Name)
		}
		return msg

	case MessageRoleAssistant:
		msg := openai.AssistantMessage(m.Content)
		if m.Name != "" {
			msg.OfAssistant.Name = param.NewOpt(m.Name)
		}

		if len(m.ToolCalls) > 0 {
			// Convert our tool calls to OpenAI format
			toolCalls := make([]openai.ChatCompletionMessageToolCallParam, 0, len(m.ToolCalls))
			for _, tc := range m.ToolCalls {
				toolCalls = append(toolCalls, tc.ToOpenAIToolCallParam())
			}
			msg.OfAssistant.ToolCalls = toolCalls
		}
		return msg

	case MessageRoleTool:
		return openai.ToolMessage(m.Content, m.ToolCallID)

	default:
		// Default to user message if unknown role
		return openai.UserMessage(m.Content)
	}
}

// ToOpenAIMessages converts a slice of Messages to OpenAI format.
func ToOpenAIMessages(messages []Message) []openai.ChatCompletionMessageParamUnion {
	result := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))
	for _, msg := range messages {
		result = append(result, msg.ToOpenAIMessage())
	}
	return result
}
