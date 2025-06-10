package holon

import (
	"context"
	"encoding/json"
	"fmt"
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
	title, ok := inputs["title"].(string)
	if !ok || title == "" {
		return &agenttypes.StructuredToolResult{
			ToolName:   "preview_thread",
			ToolParams: inputs,
			ToolError:  "title parameter is required and must be a non-empty string",
		}, fmt.Errorf("title parameter is required")
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
		// If no content provided, use title as basic content
		content = title
	}

	previewID := "preview-" + time.Now().UTC().Format("20060102150405")

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
			Description: param.NewOpt("Should be used whenever user mentions holon or network. Create content for a holon network including threads, posts, invitations, announcements, or any network communication. Use this tool when users want to interact with the holon network, send invitations, create posts, share content, or communicate within the network. Title is required and content is optional for additional description."),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"title": map[string]any{
						"type":        "string",
						"description": "The title/subject of the thread, post, or invitation",
					},
					"content": map[string]any{
						"type":        "string",
						"description": "Optional additional content/description for the thread",
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
				"required": []string{"title", "author_identity"},
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


func (t *SendToHolonTool) Execute(ctx context.Context, inputs map[string]any) (agenttypes.ToolResult, error) {
	previewID, ok := inputs["id"].(string)
	if !ok || previewID == "" {
		return &agenttypes.StructuredToolResult{
			ToolName:   "send_to_holon",
			ToolParams: inputs,
			ToolError:  "id parameter is required and must be a non-empty string from a previous preview_thread call",
		}, fmt.Errorf("id parameter is required")
	}

	fmt.Println("previewID", previewID)

	// if !strings.HasPrefix(previewID, "preview-") {
	// 	return &agenttypes.StructuredToolResult{
	// 		ToolName:   "send_to_holon",
	// 		ToolParams: inputs,
	// 		ToolError:  "id must be a valid preview ID from a previous preview_thread call",
	// 	}, fmt.Errorf("invalid preview ID")
	// }

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
		return &agenttypes.StructuredToolResult{
			ToolName:   "send_to_holon",
			ToolParams: inputs,
			ToolError:  "author_identity parameter is required and must be a non-empty string",
		}, fmt.Errorf("author_identity parameter is required")
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
		actions = []string{"Like", "Reply"}
	}

	publishedThread, err := t.Service.SendToHolon(ctx, previewID, title, content, authorIdentity, imageURLs, actions)
	if err != nil {
		return &agenttypes.StructuredToolResult{
			ToolName:   "send_to_holon",
			ToolParams: inputs,
			ToolError:  fmt.Sprintf("Failed to publish thread: %v", err),
		}, err
	}

	// Create structured JSON for the content field
	structuredData := map[string]any{
		"id":        publishedThread.ID,
		"title":     publishedThread.Title,
		"content":   publishedThread.Content,
		"createdAt": publishedThread.CreatedAt,
		"views":     int(publishedThread.Views),
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
