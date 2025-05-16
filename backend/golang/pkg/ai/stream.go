// Owner: august@eternis.ai
package ai

import (
	"context"

	"github.com/openai/openai-go"
)

type StreamDelta struct {
	ContentDelta string
	IsCompleted  bool
}

type Stream struct {
	Content   <-chan StreamDelta
	ToolCalls <-chan openai.ChatCompletionMessageToolCall
	Err       <-chan error
}

func (s *Service) CompletionsStream(
	ctx context.Context,
	messages []openai.ChatCompletionMessageParamUnion,
	tools []openai.ChatCompletionToolParam,
	model string,
) Stream {
	contentCh := make(chan StreamDelta, 64)
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
			s.logger.Debug("chunk", "chunk", chunk)

			if tc, ok := acc.JustFinishedToolCall(); ok {
				s.logger.Debug("tool call", "tool call", tc)
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

			if _, ok := acc.JustFinishedContent(); ok {
				s.logger.Debug("finished content")
				contentCh <- StreamDelta{
					ContentDelta: "",
					IsCompleted:  true,
				}
			}

			// Content delta?
			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				contentCh <- StreamDelta{
					ContentDelta: chunk.Choices[0].Delta.Content,
					IsCompleted:  false,
				}
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
