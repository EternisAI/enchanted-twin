// ai/stream.go
package ai

import (
	"context"

	"github.com/openai/openai-go"
)

type Stream struct {
	Content   <-chan string
	ToolCalls <-chan openai.ChatCompletionMessageToolCall
	Err       <-chan error
}

func (s *Service) CompletionsStream(
	ctx context.Context,
	messages []openai.ChatCompletionMessageParamUnion,
	tools []openai.ChatCompletionToolParam,
	model string,
) Stream {
	contentCh := make(chan string, 64)
	toolCh := make(chan openai.ChatCompletionMessageToolCall, 8)
	errCh := make(chan error, 1)

	go func() {
		defer close(contentCh)
		defer close(toolCh)
		defer close(errCh)

		params := openai.ChatCompletionNewParams{
			Model:    model,
			Messages: messages,
			Tools:    tools,
		}

		stream := s.client.Chat.Completions.NewStreaming(ctx, params)
		defer func() {
			_ = stream.Close()
		}()

		acc := openai.ChatCompletionAccumulator{}

		for stream.Next() {
			chunk := stream.Current()
			acc.AddChunk(chunk)

			if tc, ok := acc.JustFinishedToolCall(); ok {
				toolCh <- openai.ChatCompletionMessageToolCall{
					ID:   tc.Id,
					Type: "function",
					Function: openai.ChatCompletionMessageToolCallFunction{
						Name:      tc.Name,
						Arguments: tc.Arguments,
						JSON:      tc.JSON,
					},
				}
			}

			// Content delta?
			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				contentCh <- chunk.Choices[0].Delta.Content
			}

			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			default:
			}
		}

		if err := stream.Err(); err != nil {
			errCh <- err
		}
	}()

	return Stream{
		Content:   contentCh,
		ToolCalls: toolCh,
		Err:       errCh,
	}
}
