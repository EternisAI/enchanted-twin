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

func NewPrivateCompletionsService(config PrivateCompletionsConfig) *PrivateCompletionsService {
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
	return service
}

func (s *PrivateCompletionsService) Shutdown() {
	if s.executor != nil {
		s.executor.Shutdown()
		s.logger.Info("PrivateCompletionsService shut down")
	}
}

func (s *PrivateCompletionsService) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string, priority Priority) (PrivateCompletionResult, error) {
	s.logger.Debug("Starting private completion processing", "model", model, "messageCount", len(messages), "toolCount", len(tools))

	anonymizedMessages, allRules, err := s.scheduleAnonymization(ctx, messages, priority)
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

func (s *PrivateCompletionsService) scheduleAnonymization(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, priority Priority) ([]openai.ChatCompletionMessageParamUnion, map[string]string, error) {
	task := microscheduler.Task{
		Name:         "AnonymizeMessages",
		Priority:     priority,
		InitialState: &microscheduler.NoOpTaskState{},
		Compute: func(resource interface{}, state microscheduler.TaskState, interrupt *microscheduler.InterruptContext, interruptChan <-chan struct{}) (interface{}, error) {
			anonymizedMessages, rules, err := s.anonymizer.AnonymizeMessages(ctx, messages, interruptChan)
			if err != nil {
				return nil, err
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
