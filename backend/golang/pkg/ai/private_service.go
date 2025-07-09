package ai

import (
	"context"
	"fmt"

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
	result, err := s.completionsService.Completions(ctx, anonymizedMessages, tools, model, priority)
	if err != nil {
		return PrivateCompletionResult{}, fmt.Errorf("underlying completions service failed: %w", err)
	}
	completionMessage := result.Message

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
