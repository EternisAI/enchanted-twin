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

	authorIdentity := "current-user"
	if author, ok := inputs["author_identity"].(string); ok && author != "" {
		authorIdentity = author
	}

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
			Description: param.NewOpt("Generate a preview of a thread for a holon network. The LLM will use the context to create appropriate title and content. This must be called before send_to_holon."),
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
						"description": "Optional author identity (defaults to 'current-user')",
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
				"required": []string{"context"},
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
	// Check if user_confirmed is true
	userConfirmed, ok := inputs["user_confirmed"].(bool)
	if !ok || !userConfirmed {
		return &agenttypes.StructuredToolResult{
			ToolName:   "send_to_holon",
			ToolParams: inputs,
			ToolError:  "user_confirmed parameter must be set to true. This tool should only be called after the user has confirmed the preview is good to go.",
		}, fmt.Errorf("user confirmation required")
	}

	previewID, ok := inputs["id"].(string)
	if !ok || previewID == "" {
		return &agenttypes.StructuredToolResult{
			ToolName:   "send_to_holon",
			ToolParams: inputs,
			ToolError:  "id parameter is required and must be a non-empty string from a previous preview_thread call",
		}, fmt.Errorf("id parameter is required")
	}

	// Validate that this is a preview ID (should start with "preview-")
	if !strings.HasPrefix(previewID, "preview-") {
		return &agenttypes.StructuredToolResult{
			ToolName:   "send_to_holon",
			ToolParams: inputs,
			ToolError:  "id must be a valid preview ID from a previous preview_thread call",
		}, fmt.Errorf("invalid preview ID")
	}

	// Extract thread parameters
	title, ok := inputs["title"].(string)
	if !ok || title == "" {
		title = "Untitled Thread"
	}

	content, ok := inputs["content"].(string)
	if !ok || content == "" {
		content = "No content provided"
	}

	authorIdentity, ok := inputs["author_identity"].(string)
	if !ok || authorIdentity == "" {
		authorIdentity = "current-user"
	}

	// Extract imageURLs array
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

	// Extract actions array
	var actions []string
	if actionsInput, ok := inputs["actions"].([]interface{}); ok {
		for _, action := range actionsInput {
			if actionStr, ok := action.(string); ok {
				actions = append(actions, actionStr)
			}
		}
	}
	if actions == nil {
		actions = []string{"like", "reply", "share"}
	}

	// Use the service to publish the thread
	publishedThread, err := t.Service.SendToHolon(ctx, previewID, title, content, authorIdentity, imageURLs, actions)
	if err != nil {
		return &agenttypes.StructuredToolResult{
			ToolName:   "send_to_holon",
			ToolParams: inputs,
			ToolError:  fmt.Sprintf("Failed to publish thread: %v", err),
		}, err
	}

	networkName := "default-holon"
	if network, ok := inputs["network"].(string); ok && network != "" {
		networkName = network
	}

	// Create structured JSON for the content field
	structuredData := map[string]any{
		"thread_id":    publishedThread.ID,
		"title":        publishedThread.Title,
		"content":      publishedThread.Content,
		"network":      networkName,
		"published_at": publishedThread.CreatedAt,
		"views":        int(publishedThread.Views),
		"status":       "published",
		"message":      fmt.Sprintf("Thread successfully published to %s! Thread ID: %s", networkName, publishedThread.ID),
	}
	
	structuredJSON, err := json.Marshal(structuredData)
	if err != nil {
		return &agenttypes.StructuredToolResult{
			ToolName:   "send_to_holon",
			ToolParams: inputs,
			ToolError:  fmt.Sprintf("Failed to marshal structured data: %v", err),
		}, fmt.Errorf("failed to marshal structured data: %v", err)
	}

	return &agenttypes.StructuredToolResult{
		ToolName:   "send_to_holon",
		ToolParams: inputs,
		Output: map[string]any{
			"content": string(structuredJSON),
		},
	}, nil
}

func (t *SendToHolonTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        "send_to_holon",
			Description: param.NewOpt("Publish a previewed thread to a holon network. This will make the thread live and visible to other holon members. IMPORTANT: This tool should only be called after preview_thread has been called and the user has explicitly confirmed the preview is good to go."),
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
						"description": "The identity of the thread author (defaults to 'current-user')",
					},
					"image_urls": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Optional array of image URLs to include with the thread",
					},
					"actions": map[string]any{
						"type":        "array", 
						"items":       map[string]any{"type": "string"},
						"description": "Available actions for the thread (defaults to ['like', 'reply', 'share'])",
					},
					"network": map[string]any{
						"type":        "string",
						"description": "Optional holon network override (e.g., 'ai-research-holon')",
					},
					"user_confirmed": map[string]any{
						"type":        "boolean",
						"description": "Must be set to true to confirm the user has approved the preview",
					},
				},
				"required": []string{"id", "title", "content", "user_confirmed"},
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
		authorIdentity = "current-user"
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
		"message_id":       addedMessage.ID,
		"thread_id":        threadID,
		"message":          addedMessage.Content,
		"author_identity":  addedMessage.Author,
		"created_at":       addedMessage.CreatedAt,
		"status":           "sent",
		"message_response": fmt.Sprintf("Message successfully added to thread %s! Message ID: %s", threadID, addedMessage.ID),
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
						"description": "The identity of the message author (defaults to 'current-user')",
					},
					"image_urls": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Optional array of image URLs to include with the message",
					},
				},
				"required": []string{"thread_id", "message"},
			},
		},
	}
}

func generateSuggestedTags(content string) []string {
	tags := []string{"discussion"}

	if len(content) > 200 {
		tags = append(tags, "long-form")
	}

	// Simple keyword detection
	contentLower := strings.ToLower(content)
	keywords := map[string]string{
		"ai":            "artificial-intelligence",
		"blockchain":    "blockchain",
		"research":      "research",
		"collaboration": "collaboration",
		"question":      "question",
		"idea":          "idea",
	}

	for keyword, tag := range keywords {
		if strings.Contains(contentLower, keyword) {
			tags = append(tags, tag)
		}
	}

	return tags
}

