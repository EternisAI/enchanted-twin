package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/pkg/microscheduler"
)

type PrivateCompletionsService struct {
	completionsService CompletionsService
	anonymizer         Anonymizer
	executor           *microscheduler.TaskExecutor
	logger             *log.Logger
}

type CompletionsService interface {
	Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string, priority Priority) (PrivateCompletionResult, error)
	CompletionsStream(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) Stream
}

type PrivateCompletionsConfig struct {
	CompletionsService CompletionsService
	Anonymizer         Anonymizer
	ExecutorWorkers    int
	Logger             *log.Logger
}

func NewPrivateCompletionsService(config PrivateCompletionsConfig) (*PrivateCompletionsService, error) {
	// Validate required dependencies
	if config.CompletionsService == nil {
		return nil, fmt.Errorf("completionsService is required but was nil")
	}
	if config.Anonymizer == nil {
		return nil, fmt.Errorf("anonymizer is required but was nil")
	}
	if config.Logger == nil {
		return nil, fmt.Errorf("logger is required but was nil")
	}

	workers := config.ExecutorWorkers
	if workers <= 0 {
		workers = 1
	}

	executor := microscheduler.NewTaskExecutor(workers, config.Logger)

	service := &PrivateCompletionsService{
		completionsService: config.CompletionsService,
		anonymizer:         config.Anonymizer,
		executor:           executor,
		logger:             config.Logger,
	}

	config.Logger.Info("PrivateCompletionsService created", "workers", workers)
	return service, nil
}

func (s *PrivateCompletionsService) Shutdown() {
	if s.executor != nil {
		s.executor.Shutdown()
		s.logger.Info("PrivateCompletionsService shut down")
	}
}

func (s *PrivateCompletionsService) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string, priority Priority) (PrivateCompletionResult, error) {
	// Call new method with empty conversation ID (memory-only mode)
	return s.CompletionsWithContext(ctx, "", messages, tools, model, priority)
}

func (s *PrivateCompletionsService) CompletionsWithContext(ctx context.Context, conversationID string, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string, priority Priority) (PrivateCompletionResult, error) {
	s.logger.Debug("Starting private completion processing", "model", model, "conversationID", conversationID, "messageCount", len(messages), "toolCount", len(tools))

	anonymizedMessages, allRules, err := s.scheduleAnonymization(ctx, conversationID, messages, priority)
	if err != nil {
		return PrivateCompletionResult{}, fmt.Errorf("failed to anonymize messages: %w", err)
	}

	s.logger.Debug("Calling underlying completions service with anonymized content")

	var completionMessage openai.ChatCompletionMessage

	// Cast to *Service to access the RawCompletions method that bypasses private completions
	if rawService, ok := s.completionsService.(*Service); ok {
		result, err := rawService.RawCompletions(ctx, anonymizedMessages, tools, model)
		if err != nil {
			return PrivateCompletionResult{}, fmt.Errorf("underlying completions service failed: %w", err)
		}
		completionMessage = result.Message
	} else {
		// Fallback to regular Completions method (for tests or other implementations)
		result, err := s.completionsService.Completions(ctx, anonymizedMessages, tools, model, priority)
		if err != nil {
			return PrivateCompletionResult{}, fmt.Errorf("underlying completions service failed: %w", err)
		}
		completionMessage = result.Message
	}

	deAnonymizedMessage := s.deAnonymizeMessage(completionMessage, allRules)

	s.logger.Debug("Private completion processing complete", "originalRulesCount", len(allRules))

	return PrivateCompletionResult{
		Message:          deAnonymizedMessage,
		ReplacementRules: allRules,
	}, nil
}

func (s *PrivateCompletionsService) scheduleAnonymization(ctx context.Context, conversationID string, messages []openai.ChatCompletionMessageParamUnion, priority Priority) ([]openai.ChatCompletionMessageParamUnion, map[string]string, error) {
	task := microscheduler.Task{
		Name:         "AnonymizeMessages",
		Priority:     priority,
		InitialState: &microscheduler.NoOpTaskState{},
		Compute: func(resource interface{}, state microscheduler.TaskState, interrupt *microscheduler.InterruptContext, interruptChan <-chan struct{}) (interface{}, error) {
			// Check for context cancellation before starting
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("anonymization canceled before starting: %w", ctx.Err())
			default:
			}

			// Check for task interruption before starting
			if interrupt.CheckAndConsumeInterrupt() {
				return nil, fmt.Errorf("anonymization task interrupted before starting")
			}

			anonymizedMessages, _, rules, err := s.anonymizer.AnonymizeMessages(ctx, conversationID, messages, nil, interruptChan)

			// Check for context cancellation after anonymization
			select {
			case <-ctx.Done():
				if err != nil {
					return nil, fmt.Errorf("anonymization failed and context canceled: %w (original error: %v)", ctx.Err(), err)
				}
				return nil, fmt.Errorf("anonymization canceled after completion: %w", ctx.Err())
			default:
			}

			// Handle anonymization errors with context information
			if err != nil {
				// Check if the error was due to interruption
				if interrupt.CheckAndConsumeInterrupt() {
					return nil, fmt.Errorf("anonymization failed due to task interruption: %w", err)
				}
				return nil, fmt.Errorf("anonymization failed: %w", err)
			}

			return AnonymizationResult{
				Messages: anonymizedMessages,
				Rules:    rules,
			}, nil
		},
	}

	result, err := s.executor.Execute(ctx, task, priority)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to execute anonymization task: %w", err)
	}

	anonymizationResult, ok := result.(AnonymizationResult)
	if !ok {
		return nil, nil, fmt.Errorf("unexpected anonymization result type: %T", result)
	}

	return anonymizationResult.Messages, anonymizationResult.Rules, nil
}

type AnonymizationResult struct {
	Messages []openai.ChatCompletionMessageParamUnion
	Rules    map[string]string
}

func (s *PrivateCompletionsService) deAnonymizeMessage(message openai.ChatCompletionMessage, rules map[string]string) openai.ChatCompletionMessage {
	result := message

	if message.Content != "" {
		originalContent := s.anonymizer.DeAnonymize(message.Content, rules)
		result.Content = originalContent
	}

	if len(message.ToolCalls) > 0 {
		result.ToolCalls = make([]openai.ChatCompletionMessageToolCall, len(message.ToolCalls))
		copy(result.ToolCalls, message.ToolCalls)

		for i, toolCall := range message.ToolCalls {
			if toolCall.Function.Arguments != "" {
				originalArgs := s.anonymizer.DeAnonymize(toolCall.Function.Arguments, rules)
				result.ToolCalls[i].Function.Arguments = originalArgs
			}
		}
	}

	return result
}

func (s *PrivateCompletionsService) CompletionsStream(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string, priority Priority, onDelta func(StreamDelta)) (PrivateCompletionResult, error) {
	// Call new method with empty conversation ID (memory-only mode)
	return s.CompletionsStreamWithContext(ctx, "", messages, tools, model, priority, onDelta)
}

func (s *PrivateCompletionsService) CompletionsStreamWithContext(ctx context.Context, conversationID string, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string, priority Priority, onDelta func(StreamDelta)) (PrivateCompletionResult, error) {
	s.logger.Debug("Starting private completion streaming", "model", model, "conversationID", conversationID, "messageCount", len(messages), "toolCount", len(tools))

	// 1. Anonymize input messages
	anonymizedMessages, allRules, err := s.scheduleAnonymization(ctx, conversationID, messages, priority)
	if err != nil {
		return PrivateCompletionResult{}, fmt.Errorf("failed to anonymize messages: %w", err)
	}

	// 2. Set up streaming with accumulation
	var accumulatedContent strings.Builder
	var finalMessage openai.ChatCompletionMessage
	var toolCalls []openai.ChatCompletionMessageToolCall

	// 3. Start streaming from the underlying service
	stream := s.completionsService.CompletionsStream(ctx, anonymizedMessages, tools, model)

	// 4. Process streaming chunks
streamLoop:
	for {
		select {
		case delta, ok := <-stream.Content:
			if !ok {
				// Content channel closed, mark as nil
				stream.Content = nil
			} else {
				// Accumulate the anonymized content
				accumulatedContent.WriteString(delta.ContentDelta)
				currentAccumulated := accumulatedContent.String()

				// Deanonymize the entire accumulated content
				deanonymizedAccumulated := s.anonymizer.DeAnonymize(currentAccumulated, allRules)

				// Call user's onDelta with both versions
				if onDelta != nil {
					onDelta(StreamDelta{
						ContentDelta:                   delta.ContentDelta,
						IsCompleted:                    delta.IsCompleted,
						AccumulatedAnonymizedMessage:   currentAccumulated,
						AccumulatedDeanonymizedMessage: deanonymizedAccumulated,
					})
				}

				if delta.IsCompleted {
					finalMessage.Content = currentAccumulated
					finalMessage.Role = "assistant"
				}
			}

		case toolCall, ok := <-stream.ToolCalls:
			if !ok {
				// Tool calls channel closed, mark as nil
				stream.ToolCalls = nil
			} else {
				toolCalls = append(toolCalls, toolCall)
			}

		case err, ok := <-stream.Err:
			if !ok {
				// Error channel closed, mark as nil
				stream.Err = nil
			} else {
				if err != nil {
					return PrivateCompletionResult{}, fmt.Errorf("streaming error: %w", err)
				}
			}

		case <-ctx.Done():
			return PrivateCompletionResult{}, fmt.Errorf("context canceled during streaming: %w", ctx.Err())
		}

		// Check if all channels are closed
		if stream.Content == nil && stream.ToolCalls == nil && stream.Err == nil {
			break streamLoop
		}
	}

	// 5. Handle tool calls if any
	if len(toolCalls) > 0 {
		finalMessage.ToolCalls = toolCalls
	}

	// 6. Deanonymize the final message
	deanonymizedMessage := s.deAnonymizeMessage(finalMessage, allRules)

	s.logger.Debug("Private completion streaming complete", "originalRulesCount", len(allRules))

	return PrivateCompletionResult{
		Message:          deanonymizedMessage,
		ReplacementRules: allRules,
	}, nil
}
