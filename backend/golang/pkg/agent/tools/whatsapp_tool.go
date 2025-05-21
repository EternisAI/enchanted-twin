package tools

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/charmbracelet/log"
	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
	"github.com/pkg/errors"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"

	agenttypes "github.com/EternisAI/enchanted-twin/pkg/agent/types"
)

type WhatsAppTool struct {
	Logger *log.Logger
	Client *whatsmeow.Client
}

func NewWhatsAppTool(logger *log.Logger, client *whatsmeow.Client) *WhatsAppTool {
	return &WhatsAppTool{
		Logger: logger,
		Client: client,
	}
}

func (t *WhatsAppTool) Execute(ctx context.Context, input map[string]any) (agenttypes.ToolResult, error) {
	if t.Client == nil {
		return &agenttypes.StructuredToolResult{
			ToolName:   "whatsapp",
			ToolParams: input,
			ToolError:  "WhatsApp client not initialized",
		}, fmt.Errorf("WhatsApp client not initialized")
	}

	message, ok := input["message"].(string)
	if !ok {
		return &agenttypes.StructuredToolResult{
			ToolName:   "whatsapp",
			ToolParams: input,
			ToolError:  "message parameter is required and must be a string",
		}, fmt.Errorf("message parameter is required and must be a string")
	}

	recipient, ok := input["recipient"].(string)
	if !ok {
		return &agenttypes.StructuredToolResult{
			ToolName:   "whatsapp",
			ToolParams: input,
			ToolError:  "recipient parameter is required and must be a string",
		}, fmt.Errorf("recipient parameter is required and must be a string")
	}

	// Create JID from recipient
	jid := types.JID{
		User:   recipient,
		Server: "s.whatsapp.net",
	}

	// Create message payload
	msg := &waE2E.Message{
		Conversation: proto.String(message),
	}

	// Send message
	resp, err := t.Client.SendMessage(ctx, jid, msg)
	if err != nil {
		t.Logger.Error("Error sending WhatsApp message", slog.Any("error", err))
		return &agenttypes.StructuredToolResult{
			ToolName:   "whatsapp",
			ToolParams: input,
			ToolError:  fmt.Sprintf("failed to send WhatsApp message: %v", err),
		}, errors.Wrap(err, "failed to send WhatsApp message")
	}

	t.Logger.Info("WhatsApp message sent", "response", resp)
	return &agenttypes.StructuredToolResult{
		ToolName:   "whatsapp",
		ToolParams: input,
		Output: map[string]any{
			"content": fmt.Sprintf("Message sent successfully to %s", recipient),
			"id":      resp.ID,
		},
	}, nil
}

func (t *WhatsAppTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        "whatsapp_send_message",
			Description: param.NewOpt("Send a WhatsApp message to a recipient"),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"message":   map[string]string{"type": "string", "description": "The message content to send"},
					"recipient": map[string]string{"type": "string", "description": "The recipient's phone number without the '+' sign (e.g. '33687866890')"},
				},
				"required": []string{"message", "recipient"},
			},
		},
	}
}
