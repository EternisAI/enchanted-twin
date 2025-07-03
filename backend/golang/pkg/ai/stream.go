// Owner: august@eternis.ai
package ai

import (
	"context"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
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

		s.logger.Debug("Starting stream", "model", model, "messages_count", len(messages), "tools_count", len(tools))

		// Add timeout context to prevent hanging
		timeoutCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
		defer cancel()
		opts := s.opts

		if s.getAccessToken != nil {
			firebaseToken, err := s.getAccessToken()
			if err != nil {
				return
			}
			opts = append(opts, option.WithHeader("Authorization", "Bearer "+firebaseToken))
		}

		stream := s.client.Chat.Completions.NewStreaming(timeoutCtx, params, opts...)
		defer func() {
			if err := stream.Close(); err != nil {
				s.logger.Error("Error closing stream", "error", err)
			}
		}()

		acc := openai.ChatCompletionAccumulator{}

		for stream.Next() {
			chunk := stream.Current()

			acc.AddChunk(chunk)

			if tc, ok := acc.JustFinishedToolCall(); ok {
				s.logger.Debug("Tool call completed", "tool_call_id", tc.ID, "tool_name", tc.Name, "arguments", tc.Arguments)
				toolCh <- openai.ChatCompletionMessageToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: openai.ChatCompletionMessageToolCallFunction{
						Name:      tc.Name,
						Arguments: tc.Arguments,
						JSON:      tc.JSON,
					},
				}
			}

			if _, ok := acc.JustFinishedContent(); ok {
				s.logger.Debug("Content finished")
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
				s.logger.Error("Context canceled during streaming", "error", ctx.Err())
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
