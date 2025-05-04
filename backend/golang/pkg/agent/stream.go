package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
)

func (a *Agent) ExecuteStream(
	ctx context.Context,
	messages []openai.ChatCompletionMessageParamUnion,
	currentTools []tools.Tool,
	onDelta func(string),
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
		allResults   []tools.ToolResult
		allImages    []string
	)

	runTool := func(tc openai.ChatCompletionMessageToolCall) (tools.ToolResult, error) {
		if a.PreToolCallback != nil {
			a.PreToolCallback(tc)
		}
		tool, ok := toolMap[tc.Function.Name]
		if !ok {
			return tools.ToolResult{}, fmt.Errorf("tool %q not found", tc.Function.Name)
		}
		var args map[string]any
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			return tools.ToolResult{}, err
		}
		res, err := tool.Execute(ctx, args)
		if err != nil {
			return tools.ToolResult{}, err
		}
		if a.PostToolCallback != nil {
			a.PostToolCallback(tc, res)
		}
		if res.ImageURLs != nil {
			allImages = append(allImages, res.ImageURLs...)
		}
		return res, nil
	}

	for step := 0; step < MAX_STEPS; step++ {
		stepContent := ""
		stepCalls := []openai.ChatCompletionMessageToolCall{}
		stepResults := []tools.ToolResult{}

		stream := a.aiService.CompletionsStream(ctx, messages, toolDefs, a.CompletionsModel)

	loop:
		for {
			select {
			case delta, ok := <-stream.Content:
				if ok {
					stepContent += delta
					if onDelta != nil {
						onDelta(delta)
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

		// ----- append chat messages for next round -----
		if stepContent != "" || len(stepCalls) > 0 {
			messages = append(messages, openai.ChatCompletionMessage{
				Role:      "assistant",
				Content:   stepContent,
				ToolCalls: stepCalls,
			}.ToParam())
		}
		for i, call := range stepCalls {
			messages = append(messages, openai.ToolMessage(stepResults[i].Content, call.ID))
		}
		// ------------------------------------------------

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
