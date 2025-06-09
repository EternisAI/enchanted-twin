package holon

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"

	agenttypes "github.com/EternisAI/enchanted-twin/pkg/agent/types"
)

type ThreadPreviewTool struct {
	Service *Service
}

func NewThreadPreviewTool(service *Service) *ThreadPreviewTool {
	return &ThreadPreviewTool{
		Service: service,
	}
}

func (t *ThreadPreviewTool) Execute(ctx context.Context, inputs map[string]any) (agenttypes.ToolResult, error) {
	context, ok := inputs["context"].(string)
	if !ok || context == "" {
		return &agenttypes.StructuredToolResult{
			ToolName:   "preview_thread",
			ToolParams: inputs,
			ToolError:  "context parameter is required and must be a non-empty string",
		}, fmt.Errorf("context parameter is required")
	}

	authorIdentity, ok := inputs["author_identity"].(string)
	if !ok || authorIdentity == "" {
		return &agenttypes.StructuredToolResult{
			ToolName:   "preview_thread",
			ToolParams: inputs,
			ToolError:  "author_identity parameter is required and must be a non-empty string",
		}, fmt.Errorf("author_identity parameter is required")
	}

	var content string
	if c, ok := inputs["content"].(string); ok && c != "" {
		content = c
	} else {
		// TODO: Use LLM to generate content from context
		// For now, use context as content
		content = "Generated content based on: " + context
	}

	previewID := "preview-" + time.Now().Format("20060102150405")
	title := extractTitleFromContent(content)

	var imageURLs []string
	if urls, ok := inputs["image_urls"].([]interface{}); ok {
		for _, url := range urls {
			if urlStr, ok := url.(string); ok {
				imageURLs = append(imageURLs, urlStr)
			}
		}
	}

	structuredData := map[string]any{
		"id":              previewID,
		"title":           title,
		"content":         content,
		"author_identity": authorIdentity,
		"image_urls":      imageURLs,
	}

	structuredJSON, err := json.Marshal(structuredData)
	if err != nil {
		return &agenttypes.StructuredToolResult{
			ToolName:   "preview_thread",
			ToolParams: inputs,
			ToolError:  fmt.Sprintf("Failed to marshal structured data: %v", err),
		}, fmt.Errorf("failed to marshal structured data: %v", err)
	}

	return &agenttypes.StructuredToolResult{
		ToolName:   "preview_thread",
		ToolParams: inputs,
		Output: map[string]any{
			"content": string(structuredJSON),
		},
	}, nil
}

func (t *ThreadPreviewTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        "preview_thread",
			Description: param.NewOpt("Generate a preview of a thread for a holon network. The LLM will use the context to create appropriate title and content."),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"context": map[string]any{
						"type":        "string",
						"description": "The context or topic for the thread that the LLM should use to generate title and content",
					},
					"content": map[string]any{
						"type":        "string",
						"description": "Optional specific content for the thread (if not provided, LLM will generate from context)",
					},
					"author_identity": map[string]any{
						"type":        "string",
						"description": "The identity of the thread author (must be a valid user ID from the authors table)",
					},
					"image_urls": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Optional array of image URLs to include with the thread",
					},
					"actions": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Optional array of actions for the thread (defaults to ['like', 'reply', 'share'])",
					},
				},
				"required": []string{"context", "author_identity"},
			},
		},
	}
}

type SendToHolonTool struct {
	Service *Service
}

func NewSendToHolonTool(service *Service) *SendToHolonTool {
	return &SendToHolonTool{
		Service: service,
	}
}

func (t *SendToHolonTool) Execute(ctx context.Context, inputs map[string]any) (agenttypes.ToolResult, error) {
	// Extract inputs
	id, ok := inputs["id"].(string)
	if !ok || id == "" {
		return &agenttypes.StructuredToolResult{
			ToolName:   "send_to_holon",
			ToolParams: inputs,
			ToolError:  "id is required and must be a string",
		}, nil
	}

	title, ok := inputs["title"].(string)
	if !ok || title == "" {
		return &agenttypes.StructuredToolResult{
			ToolName:   "send_to_holon",
			ToolParams: inputs,
			ToolError:  "title is required and must be a string",
		}, nil
	}

	content, ok := inputs["content"].(string)
	if !ok || content == "" {
		return &agenttypes.StructuredToolResult{
			ToolName:   "send_to_holon",
			ToolParams: inputs,
			ToolError:  "content is required and must be a string",
		}, nil
	}

	authorIdentity, ok := inputs["author_identity"].(string)
	if !ok || authorIdentity == "" {
		return &agenttypes.StructuredToolResult{
			ToolName:   "send_to_holon",
			ToolParams: inputs,
			ToolError:  "author_identity is required and must be a string",
		}, nil
	}

	// Create thread using the holon service
	imageURLs := []string{}
	actions := []string{}
	var expiresAt *string = nil

	thread, err := t.Service.repo.CreateThread(
		ctx,
		id,
		title,
		content,
		authorIdentity,
		imageURLs,
		actions,
		expiresAt,
		"pending", // Mark as pending so it gets pushed to HolonZero API
	)
	if err != nil {
		return &agenttypes.StructuredToolResult{
			ToolName:   "send_to_holon",
			ToolParams: inputs,
			ToolError:  fmt.Sprintf("failed to create thread: %v", err),
		}, nil
	}

	// Prepare result with proper field names matching swagger
	result := map[string]interface{}{
		"id":            thread.ID,
		"title":         thread.Title,
		"content":       thread.Content,
		"creatorId":     authorIdentity,
		"createdAt":     thread.CreatedAt, // Already a string in RFC3339 format
		"dedupThreadId": id,               // Use the provided ID as dedup ID
		"actions":       actions,
	}

	return &agenttypes.StructuredToolResult{
		ToolName:   "send_to_holon",
		ToolParams: inputs,
		Output:     result,
	}, nil
}

func (t *SendToHolonTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        "send_to_holon",
			Description: param.NewOpt("Publish a previewed thread to a holon network. This will make the thread live and visible to other holon members. CRITICAL: Only call this tool when the user has explicitly confirmed they want to publish the preview. Look for confirmation phrases like 'yes', 'publish it', 'looks good', 'send it', 'go ahead', or similar. Do NOT call this tool unless the user has clearly indicated they approve of the preview."),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]any{
						"type":        "string",
						"description": "The ID of the thread preview to publish (from preview_thread tool)",
					},
					"title": map[string]any{
						"type":        "string",
						"description": "The title of the thread",
					},
					"content": map[string]any{
						"type":        "string",
						"description": "The main content/body of the thread",
					},
					"author_identity": map[string]any{
						"type":        "string",
						"description": "The identity of the thread author (must be a valid user ID from the authors table)",
					},
					"image_urls": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Optional array of image URLs to include with the thread",
					},
					"actions": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Available actions for the thread. For normal posts: ['Reply']. For invitations: ['Accept the invitation', 'Reply']. Defaults to ['Reply']",
					},
				},
				"required": []string{"id", "title", "content", "author_identity"},
			},
		},
	}
}

type AddMessageToThreadTool struct {
	Service *Service
}

func NewAddMessageToThreadTool(service *Service) *AddMessageToThreadTool {
	return &AddMessageToThreadTool{
		Service: service,
	}
}

func (t *AddMessageToThreadTool) Execute(ctx context.Context, inputs map[string]any) (agenttypes.ToolResult, error) {
	threadID, ok := inputs["thread_id"].(string)
	if !ok || threadID == "" {
		return &agenttypes.StructuredToolResult{
			ToolName:   "add_message_to_thread",
			ToolParams: inputs,
			ToolError:  "thread_id parameter is required and must be a non-empty string",
		}, fmt.Errorf("thread_id parameter is required")
	}

	message, ok := inputs["message"].(string)
	if !ok || message == "" {
		return &agenttypes.StructuredToolResult{
			ToolName:   "add_message_to_thread",
			ToolParams: inputs,
			ToolError:  "message parameter is required and must be a non-empty string",
		}, fmt.Errorf("message parameter is required")
	}

	authorIdentity, ok := inputs["author_identity"].(string)
	if !ok || authorIdentity == "" {
		return &agenttypes.StructuredToolResult{
			ToolName:   "add_message_to_thread",
			ToolParams: inputs,
			ToolError:  "author_identity parameter is required and must be a non-empty string",
		}, fmt.Errorf("author_identity parameter is required")
	}

	// Extract optional image URLs
	var imageURLs []string
	if urls, ok := inputs["image_urls"].([]interface{}); ok {
		for _, url := range urls {
			if urlStr, ok := url.(string); ok {
				imageURLs = append(imageURLs, urlStr)
			}
		}
	}
	if imageURLs == nil {
		imageURLs = []string{}
	}

	// Use the service to add message to the thread
	addedMessage, err := t.Service.AddMessageToThread(ctx, threadID, message, authorIdentity, imageURLs)
	if err != nil {
		return &agenttypes.StructuredToolResult{
			ToolName:   "add_message_to_thread",
			ToolParams: inputs,
			ToolError:  fmt.Sprintf("Failed to add message to thread: %v", err),
		}, err
	}

	// Create structured JSON for the content field
	structuredData := map[string]any{
		"id":        addedMessage.ID,
		"threadId":  threadID,
		"content":   addedMessage.Content,
		"author":    addedMessage.Author,
		"createdAt": addedMessage.CreatedAt,
	}

	structuredJSON, err := json.Marshal(structuredData)
	if err != nil {
		return &agenttypes.StructuredToolResult{
			ToolName:   "add_message_to_thread",
			ToolParams: inputs,
			ToolError:  fmt.Sprintf("Failed to marshal structured data: %v", err),
		}, fmt.Errorf("failed to marshal structured data: %v", err)
	}

	return &agenttypes.StructuredToolResult{
		ToolName:   "add_message_to_thread",
		ToolParams: inputs,
		Output: map[string]any{
			"content": string(structuredJSON),
		},
	}, nil
}

func (t *AddMessageToThreadTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        "add_message_to_thread",
			Description: param.NewOpt("Add a message (reply) to an existing thread in the holon network. This allows for threaded conversations and replies."),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"thread_id": map[string]any{
						"type":        "string",
						"description": "The ID of the existing thread to add the message to",
					},
					"message": map[string]any{
						"type":        "string",
						"description": "The message content to add to the thread",
					},
					"author_identity": map[string]any{
						"type":        "string",
						"description": "The identity of the message author (must be a valid user ID from the authors table)",
					},
					"image_urls": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Optional array of image URLs to include with the message",
					},
				},
				"required": []string{"thread_id", "message", "author_identity"},
			},
		},
	}
}
