package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
)

type StreamDelta struct {
	ContentDelta string
	IsCompleted  bool
	ImageURLs    []string
}

func (a *Agent) ExecuteStream(
	ctx context.Context,
	messages []openai.ChatCompletionMessageParamUnion,
	currentTools []tools.Tool,
	onDelta func(StreamDelta),
) (AgentResponse, error) {
	// Build lookup + OpenAI tool defs once.
	toolDefs := make([]openai.ChatCompletionToolParam, 0, len(currentTools))
	toolMap := map[string]tools.Tool{}
	for _, t := range currentTools {
		d := t.Definition()
		toolDefs = append(toolDefs, d)
		toolMap[d.Function.Name] = t
	}

	var (
		finalContent string
		allCalls     []openai.ChatCompletionMessageToolCall
		allResults   []types.ToolResult
		allImages    []string
	)

	runTool := func(tc openai.ChatCompletionMessageToolCall) (types.ToolResult, error) {
		a.logger.Debug("Pre tool callback", "tool_call", tc)
		if a.PreToolCallback != nil {
			a.PreToolCallback(tc)
		}
		tool, ok := toolMap[tc.Function.Name]
		if !ok {
			return nil, fmt.Errorf("tool %q not found", tc.Function.Name)
		}
		var args map[string]any
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			return nil, err
		}
		toolResult, err := tool.Execute(ctx, args)
		if err != nil {
			return nil, err
		}

		a.logger.Debug("Post tool callback", "result", toolResult)
		if a.PostToolCallback != nil {
			a.PostToolCallback(tc, toolResult)
		}

		if urls := toolResult.ImageURLs(); len(urls) > 0 {
			allImages = append(allImages, urls...)
		}
		return toolResult, nil
	}

	for step := 0; step < MAX_STEPS; step++ {
		stepContent := ""
		stepCalls := []openai.ChatCompletionMessageToolCall{}
		stepResults := []types.ToolResult{}

		stream := a.aiService.CompletionsStream(ctx, messages, toolDefs, a.CompletionsModel)

	loop:
		for {
			select {
			case delta, ok := <-stream.Content:
				if ok {
					stepContent += delta.ContentDelta
					if onDelta != nil {
						onDelta(StreamDelta{
							ContentDelta: delta.ContentDelta,
							IsCompleted:  delta.IsCompleted,
						})
					}
				} else {
					stream.Content = nil
				}

			case tc, ok := <-stream.ToolCalls:
				if ok {
					res, err := runTool(tc)
					if err != nil {
						return AgentResponse{}, err
					}
					stepCalls = append(stepCalls, tc)
					stepResults = append(stepResults, res)

					// Send image URLs if any
					imageURLs := res.ImageURLs()
					if len(imageURLs) > 0 {
						onDelta(StreamDelta{
							ImageURLs: imageURLs,
						})
					}
				} else {
					stream.ToolCalls = nil
				}

			case err, ok := <-stream.Err:
				if ok && err != nil {
					return AgentResponse{}, err
				}
				stream.Err = nil

			case <-ctx.Done():
				return AgentResponse{}, ctx.Err()
			}

			if stream.Content == nil && stream.ToolCalls == nil && stream.Err == nil {
				break loop
			}
		}

		// append chat messages for next round
		if stepContent != "" || len(stepCalls) > 0 {
			messages = append(messages, openai.ChatCompletionMessage{
				Role:      "assistant",
				Content:   stepContent,
				ToolCalls: stepCalls,
			}.ToParam())
		}
		for i, call := range stepCalls {
			messages = append(messages, openai.ToolMessage(stepResults[i].Content(), call.ID))
		}

		// keep global history
		allCalls = append(allCalls, stepCalls...)
		allResults = append(allResults, stepResults...)
		if stepContent != "" {
			finalContent = stepContent
		}

		// finished when no tool-calls in this step
		if len(stepCalls) == 0 {
			break
		}
		// small yield helps fairness when tight-looping
		time.Sleep(0)
	}

	return AgentResponse{
		Content:     finalContent,
		ToolCalls:   allCalls,
		ToolResults: allResults,
		ImageURLs:   allImages,
	}, nil
}
