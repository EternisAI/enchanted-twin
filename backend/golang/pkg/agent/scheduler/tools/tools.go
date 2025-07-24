package tools

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"

	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
)

type ScheduleTask struct {
	Logger         *log.Logger
	TemporalClient client.Client
	ToolsRegistry  tools.ToolRegistry
}

func (e *ScheduleTask) Execute(ctx context.Context, inputs map[string]any) (types.ToolResult, error) {
	// Log all available tools in the registry for debugging
	if e.ToolsRegistry != nil {
		availableTools := e.ToolsRegistry.List()
		e.Logger.Info("Available tools in registry", "count", len(availableTools), "tools", availableTools)
		for i, toolName := range availableTools {
			if tool, exists := e.ToolsRegistry.Get(toolName); exists {
				def := tool.Definition()
				e.Logger.Debug("Tool details", "index", i+1, "name", toolName, "description", def.Function.Description)
			}
		}
	} else {
		e.Logger.Warn("ToolsRegistry is nil - no tools available for validation")
	}

	task, ok := inputs["task"].(string)
	if !ok {
		return nil, errors.New("task is required")
	}
	delay := 0.0
	delayValue, ok := inputs["delay"].(float64)
	if ok {
		delay = delayValue
	}

	var cron string
	cronValue, ok := inputs["cron"].(string)
	if ok {
		cron = cronValue
	}

	name, ok := inputs["name"].(string)
	if !ok {
		return nil, errors.New("name is required")
	}

	chatID, ok := inputs["chat_id"].(string)
	if !ok {
		return nil, errors.New("chat_id is required")
	}

	// Check for required tools
	var requiredTools []string
	if reqToolsInput, ok := inputs["required_tools"]; ok {
		if reqToolsArray, ok := reqToolsInput.([]interface{}); ok {
			for _, tool := range reqToolsArray {
				if toolStr, ok := tool.(string); ok {
					requiredTools = append(requiredTools, toolStr)
				}
			}
		}
	}

	// Auto-detect tools from task content
	detectedTools := e.detectRequiredTools(task)
	for _, tool := range detectedTools {
		if !contains(requiredTools, tool) {
			requiredTools = append(requiredTools, tool)
		}
	}

	// Validate that required tools are available
	if len(requiredTools) > 0 {
		unavailableTools := e.validateRequiredTools(requiredTools)
		if len(unavailableTools) > 0 {
			return &types.StructuredToolResult{
				ToolName:   "schedule_task",
				ToolParams: inputs,
				ToolError:  fmt.Sprintf("Required tools are not available: %v. Please ensure these tools are enabled before scheduling the task.", unavailableTools),
			}, fmt.Errorf("required tools not available: %v", unavailableTools)
		}
	}

	id := fmt.Sprintf("scheduled-task-%s-%s", toSnake(name), uuid.New().String())
	opts := client.ScheduleOptions{
		ID: id,
		Action: &client.ScheduleWorkflowAction{
			ID:        id,
			Workflow:  "TaskScheduleWorkflow",
			Args:      []any{map[string]any{"task": task, "name": name, "chat_id": chatID, "delay": delay, "cron": cron, "notify": true}},
			TaskQueue: "default",
		},
		Overlap: enums.SCHEDULE_OVERLAP_POLICY_SKIP,
	}

	if cron == "" {
		opts.Spec = client.ScheduleSpec{
			Intervals: []client.ScheduleIntervalSpec{{
				Every: time.Duration(delay) * time.Second,
			}},
			StartAt: time.Now(),
		}
		opts.RemainingActions = 1
	} else {
		opts.Spec = client.ScheduleSpec{
			CronExpressions: []string{cron},
		}
	}

	scheduleHandle, err := e.TemporalClient.ScheduleClient().Create(ctx, opts)
	if err != nil {
		e.Logger.Error("failed to schedule task", "error", err, "task", task, "name", name, "chat_id", chatID, "delay", delay, "cron", cron)
		return nil, err
	}

	e.Logger.Info("Schedule created", "scheduleID", scheduleHandle.GetID())

	return &types.StructuredToolResult{
		ToolName:   "schedule_task",
		ToolParams: inputs,
		Output: map[string]any{
			"content": fmt.Sprintf("Task `%s` has been scheduled successfully.", task),
		},
	}, nil
}

// detectRequiredTools analyzes the task content to automatically detect tool dependencies
func (e *ScheduleTask) detectRequiredTools(task string) []string {
	var detectedTools []string
	taskLower := strings.ToLower(task)

	// Tool detection patterns
	toolPatterns := map[string][]string{
		"telegram_send_message": {"telegram", "send telegram", "telegram message", "message telegram"},
		"twitter":               {"twitter", "tweet", "x.com", "post tweet", "twitter post"},
		"whatsapp":              {"whatsapp", "whatsapp message", "send whatsapp"},
		"gmail":                 {"gmail", "email", "send email", "compose email"},
		"slack":                 {"slack", "slack message", "send slack"},
		"screenpipe":            {"screenpipe", "screen capture", "screenshot"},
	}

	for toolName, patterns := range toolPatterns {
		for _, pattern := range patterns {
			if strings.Contains(taskLower, pattern) {
				if !contains(detectedTools, toolName) {
					detectedTools = append(detectedTools, toolName)
				}
				break
			}
		}
	}

	return detectedTools
}

// validateRequiredTools checks if the required tools are available in the registry
func (e *ScheduleTask) validateRequiredTools(requiredTools []string) []string {
	var unavailableTools []string

	if e.ToolsRegistry == nil {
		e.Logger.Warn("ToolsRegistry is nil, cannot validate required tools")
		return requiredTools
	}

	for _, toolName := range requiredTools {
		if _, exists := e.ToolsRegistry.Get(toolName); !exists {
			unavailableTools = append(unavailableTools, toolName)
		}
	}

	return unavailableTools
}

// contains checks if a string slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func (e *ScheduleTask) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        "schedule_task",
			Description: param.NewOpt("Schedule a task to be executed once or on a recurring basis. The system will automatically detect required tools from the task content and validate their availability."),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]string{
						"type":        "string",
						"description": "The name of the task, should be witty and under 30 characters. Use spaces to separate words.",
					},
					"task": map[string]string{
						"type":        "string",
						"description": "The task that agent should execute. It should contain all information nescessary to accomplish the task and be as detailed as user provided. Task must not include cron, delay or name of your human.",
					},
					"delay": map[string]string{
						"type":        "number",
						"description": "The delay in seconds before the task is executed.",
					},
					"cron": map[string]string{
						"type":        "string",
						"description": "Cron expression for the task to be executed periodically. Uses standard 5-field format: minute hour day-of-month month day-of-week. Examples: `*/30 * * * *` (every 30 minutes), `15 10 * * *` (daily at 10:15 AM), `*/5 9-17 * * 1-5` (every 5 minutes, 9 AM to 5 PM, weekdays only).",
					},
					"chat_id": map[string]string{
						"type":        "string",
						"description": "The ID of the chat to send the message to. No chat_id specified would send the message to a new chat.",
					},
					"required_tools": map[string]any{
						"type": "array",
						"items": map[string]string{
							"type": "string",
						},
						"description": "Optional list of tools required for this task. If not specified, the system will auto-detect from task content. Common tools: telegram_send_message, twitter, whatsapp, gmail, slack, screenpipe.",
					},
				},
				"required": []string{"task", "delay", "name", "chat_id"},
			},
		},
	}
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func toSnake(s string) string {
	s = strings.ToLower(s)
	s = nonAlnum.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	if len(s) > 20 {
		runes := []rune(s)
		if len(runes) > 20 {
			s = string(runes[:20])
		}
	}
	return s
}
