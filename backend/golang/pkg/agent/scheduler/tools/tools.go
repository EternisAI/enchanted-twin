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

	isPossible, ok := inputs["is_possible"].(bool)
	if !ok {
		return &types.StructuredToolResult{
			ToolName:   "schedule_task",
			ToolParams: inputs,
			ToolError:  "is_possible is required",
		}, errors.New("is_possible is required")
	}
	if !isPossible {
		reasoning, reasoningOk := inputs["is_possible_reasoning"].(string)
		if !reasoningOk || reasoning == "" {
			reasoning = "No reasoning provided"
		}
		return &types.StructuredToolResult{
			ToolName:   "schedule_task",
			ToolParams: inputs,
			ToolError:  fmt.Sprintf("Task is not possible to be executed: %s", reasoning),
		}, errors.New("task is not possible to be executed")
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

	e.Logger.Info("🟡 Required tools", "required_tools", inputs["required_tools"])
	e.Logger.Info("🟡 Is possible", "is_possible", inputs["is_possible"])
	e.Logger.Info("🟡 Is possible reasoning", "is_possible_reasoning", inputs["is_possible_reasoning"])

	var requiredTools []string
	if reqToolsInput, ok := inputs["required_tools"]; ok {
		if reqToolsArray, ok := reqToolsInput.([]interface{}); ok {
			for i, tool := range reqToolsArray {
				if toolStr, ok := tool.(string); ok {
					requiredTools = append(requiredTools, toolStr)
				} else {
					e.Logger.Warn("Invalid tool type in required_tools array", "index", i, "type", fmt.Sprintf("%T", tool), "value", tool)
					return &types.StructuredToolResult{
						ToolName:   "schedule_task",
						ToolParams: inputs,
						ToolError:  fmt.Sprintf("Invalid tool type at index %d in required_tools: expected string, got %T", i, tool),
					}, fmt.Errorf("invalid tool type at index %d in required_tools: expected string, got %T", i, tool)
				}
			}
		} else {
			e.Logger.Warn("Invalid required_tools format", "type", fmt.Sprintf("%T", reqToolsInput), "value", reqToolsInput)
			return &types.StructuredToolResult{
				ToolName:   "schedule_task",
				ToolParams: inputs,
				ToolError:  fmt.Sprintf("Invalid required_tools format: expected array, got %T", reqToolsInput),
			}, fmt.Errorf("invalid required_tools format: expected array, got %T", reqToolsInput)
		}
	}

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

// validateRequiredTools checks if the required tools are available in the registry.
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
					"is_possible": map[string]string{
						"type":        "boolean",
						"description": "Whether the task is possible to be executed",
					},
					"is_possible_reasoning": map[string]string{
						"type":        "string",
						"description": "Reasoning about why the task is possible to be executed given available tools",
					},
				},
				"required": []string{"task", "delay", "name", "chat_id", "required_tools", "is_possible", "is_possible_reasoning"},
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
