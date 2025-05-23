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

	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
)

type ScheduleTask struct {
	Logger         *log.Logger
	TemporalClient client.Client
}

func (e *ScheduleTask) Execute(ctx context.Context, inputs map[string]any) (types.ToolResult, error) {
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
		// If delay was given, schedule the task to be executed after the delay once
		opts.Spec = client.ScheduleSpec{
			Intervals: []client.ScheduleIntervalSpec{{
				Every: time.Duration(delay) * time.Second,
			}},
			StartAt: time.Now(),
		}
		opts.RemainingActions = 1
	} else {
		// If cron string was given, schedule the task to be executed periodically
		opts.Spec = client.ScheduleSpec{
			CronExpressions: []string{cron},
		}
	}

	scheduleHandle, err := e.TemporalClient.ScheduleClient().Create(ctx, opts)
	if err != nil {
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

func (e *ScheduleTask) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        "schedule_task",
			Description: param.NewOpt("Schedule a task to be executed once or on a recurring basis"),
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
						"description": "Cron expression for the task to be executed periodically. Cron expressions only accept integers. Examples `*/30 * * * * *`, `0 15 10 15 * *`, `0 */5 9-17 * * 1-5`.",
					},
					"chat_id": map[string]string{
						"type":        "string",
						"description": "The ID of the chat to send the message to. No chat_id specified would send the message to a new chat.",
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
