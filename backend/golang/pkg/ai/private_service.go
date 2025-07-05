package ai

import (
	"context"
	"fmt"

	"github.com/EternisAI/enchanted-twin/pkg/microscheduler"
	"github.com/openai/openai-go"
	"github.com/charmbracelet/log"
)

// PrivateCompletionsService implements PrivateCompletions with anonymization and microscheduler integration
type PrivateCompletionsService struct {
	completionsService CompletionsService // Duck-typed interface for the underlying AI service
	anonymizer         Anonymizer
	executor           *microscheduler.TaskExecutor
	logger             *log.Logger
}

// CompletionsService interface for the underlying AI service (duck typing)
type CompletionsService interface {
	Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (PrivateCompletionResult, error)
}

// PrivateCompletionsConfig holds configuration for the private completions service
type PrivateCompletionsConfig struct {
	CompletionsService CompletionsService
	Anonymizer         Anonymizer
	ExecutorWorkers    int
	Logger             *log.Logger
}

// NewPrivateCompletionsService creates a new PrivateCompletionsService instance
func NewPrivateCompletionsService(config PrivateCompletionsConfig) *PrivateCompletionsService {
	workers := config.ExecutorWorkers
	if workers <= 0 {
		workers = 1 // Default to single worker for limited throughput
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

// Shutdown gracefully shuts down the service
func (s *PrivateCompletionsService) Shutdown() {
	if s.executor != nil {
		s.executor.Shutdown()
		s.logger.Info("PrivateCompletionsService shut down")
	}
}

// Completions processes completion requests with anonymization and priority scheduling
// Only the anonymization step goes through the microscheduler and can be interrupted.
// The actual completion call and de-anonymization happen directly without scheduler interference.
func (s *PrivateCompletionsService) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string, priority Priority) (PrivateCompletionResult, error) {
	s.logger.Debug("Starting private completion processing", "model", model, "messageCount", len(messages), "toolCount", len(tools))
	
	// Step 1: Anonymize input messages through the microscheduler (can be interrupted)
	anonymizedMessages, allRules, err := s.scheduleAnonymization(ctx, messages, priority)
	if err != nil {
		return PrivateCompletionResult{}, fmt.Errorf("failed to anonymize messages: %w", err)
	}
	
	// Step 2: Call the underlying completions service directly (NOT through scheduler, cannot be interrupted)
	s.logger.Debug("Calling underlying completions service with anonymized content")
	result, err := s.completionsService.Completions(ctx, anonymizedMessages, tools, model)
	if err != nil {
		return PrivateCompletionResult{}, fmt.Errorf("underlying completions service failed: %w", err)
	}
	completionMessage := result.Message
	
	// Step 3: De-anonymize the response (also direct, no interruption)
	deAnonymizedMessage := s.deAnonymizeMessage(completionMessage, allRules)
	
	s.logger.Debug("Private completion processing complete", "originalRulesCount", len(allRules))
	
	return PrivateCompletionResult{
		Message:          deAnonymizedMessage,
		ReplacementRules: allRules,
	}, nil
}

// scheduleAnonymization runs the anonymization process through the microscheduler
// The anonymizer will either complete its full delay OR be interrupted by the scheduler
func (s *PrivateCompletionsService) scheduleAnonymization(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, priority Priority) ([]openai.ChatCompletionMessageParamUnion, map[string]string, error) {
	// Create a task for anonymization through the microscheduler
	task := microscheduler.Task{
		Name:         "AnonymizeMessages",
		Priority:     priority,
		InitialState: &microscheduler.NoOpTaskState{},
		Compute: func(resource interface{}, state microscheduler.TaskState, interrupt *microscheduler.InterruptContext, interruptChan <-chan struct{}) (interface{}, error) {
			// Call the anonymizer's AnonymizeMessages method directly
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
	
	// Execute the anonymization task through the microscheduler
	result, err := s.executor.Execute(ctx, task, priority)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to execute anonymization task: %w", err)
	}
	
	// Type assert the result
	anonymizationResult, ok := result.(AnonymizationResult)
	if !ok {
		return nil, nil, fmt.Errorf("unexpected anonymization result type: %T", result)
	}
	
	return anonymizationResult.Messages, anonymizationResult.Rules, nil
}

// AnonymizationResult holds the result of message anonymization
type AnonymizationResult struct {
	Messages []openai.ChatCompletionMessageParamUnion
	Rules    map[string]string
}


// deAnonymizeMessage restores original content in the completion response
func (s *PrivateCompletionsService) deAnonymizeMessage(message openai.ChatCompletionMessage, rules map[string]string) openai.ChatCompletionMessage {
	// Create a copy of the message
	result := message
	
	// De-anonymize content if present
	if message.Content != "" {
		originalContent := s.anonymizer.DeAnonymize(message.Content, rules)
		result.Content = originalContent
	}
	
	// De-anonymize function calls if present
	if len(message.ToolCalls) > 0 {
		// Create a copy of tool calls slice
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