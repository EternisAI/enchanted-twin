package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

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
	Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (PrivateResult, error)
}

// Singleton instance
var (
	privateInstance *PrivateCompletionsService
	privateOnce     sync.Once
)

// PrivateCompletionsConfig holds configuration for the private completions service
type PrivateCompletionsConfig struct {
	CompletionsService CompletionsService
	Anonymizer         Anonymizer
	ExecutorWorkers    int
	Logger             *log.Logger
}

// GetPrivateCompletions returns the singleton instance of PrivateCompletionsService
func GetPrivateCompletions() *PrivateCompletionsService {
	if privateInstance == nil {
		panic("PrivateCompletionsService not initialized. Call InitPrivateCompletions first.")
	}
	return privateInstance
}

// InitPrivateCompletions initializes the singleton PrivateCompletionsService
func InitPrivateCompletions(config PrivateCompletionsConfig) *PrivateCompletionsService {
	privateOnce.Do(func() {
		workers := config.ExecutorWorkers
		if workers <= 0 {
			workers = 1 // Default to single worker for limited throughput
		}
		
		executor := microscheduler.NewTaskExecutor(workers, config.Logger)
		
		privateInstance = &PrivateCompletionsService{
			completionsService: config.CompletionsService,
			anonymizer:         config.Anonymizer,
			executor:           executor,
			logger:             config.Logger,
		}
		
		config.Logger.Info("PrivateCompletionsService initialized", "workers", workers)
	})
	
	return privateInstance
}

// Shutdown gracefully shuts down the service
func (s *PrivateCompletionsService) Shutdown() {
	if s.executor != nil {
		s.executor.Shutdown()
		s.logger.Info("PrivateCompletionsService shut down")
	}
}

// Completions processes completion requests with anonymization and priority scheduling
func (s *PrivateCompletionsService) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string, priority Priority) (PrivateResult, error) {
	// Create a task for the microscheduler
	task := microscheduler.Task{
		Name:         fmt.Sprintf("PrivateCompletion-%s", model),
		Priority:     priority,
		InitialState: &microscheduler.NoOpTaskState{},
		Compute: func(resource interface{}, state microscheduler.TaskState, interrupt *microscheduler.InterruptContext, interruptChan <-chan struct{}) (interface{}, error) {
			return s.processCompletion(ctx, messages, tools, model, interruptChan)
		},
	}
	
	// Execute the task through the microscheduler
	result, err := s.executor.Execute(ctx, task, priority)
	if err != nil {
		return PrivateResult{}, fmt.Errorf("failed to execute completion task: %w", err)
	}
	
	// Type assert the result
	privateResult, ok := result.(PrivateResult)
	if !ok {
		return PrivateResult{}, fmt.Errorf("unexpected result type: %T", result)
	}
	
	return privateResult, nil
}

// processCompletion handles the actual completion processing with anonymization
func (s *PrivateCompletionsService) processCompletion(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string, interruptChan <-chan struct{}) (PrivateResult, error) {
	s.logger.Debug("Starting private completion processing", "model", model, "messageCount", len(messages), "toolCount", len(tools))
	
	// Step 1: Anonymize input messages
	anonymizedMessages, allRules, err := s.anonymizeMessages(ctx, messages, interruptChan)
	if err != nil {
		return PrivateResult{}, fmt.Errorf("failed to anonymize messages: %w", err)
	}
	
	// Check for interruption after anonymization
	select {
	case <-interruptChan:
		return PrivateResult{}, fmt.Errorf("completion interrupted during anonymization")
	default:
	}
	
	// Step 2: Call the underlying completions service with anonymized content
	s.logger.Debug("Calling underlying completions service with anonymized content")
	result, err := s.completionsService.Completions(ctx, anonymizedMessages, tools, model)
	if err != nil {
		return PrivateResult{}, fmt.Errorf("underlying completions service failed: %w", err)
	}
	completionMessage := result.Message
	
	// Check for interruption after completion
	select {
	case <-interruptChan:
		return PrivateResult{}, fmt.Errorf("completion interrupted during processing")
	default:
	}
	
	// Step 3: De-anonymize the response
	deAnonymizedMessage := s.deAnonymizeMessage(completionMessage, allRules)
	
	s.logger.Debug("Private completion processing complete", "originalRulesCount", len(allRules))
	
	return PrivateResult{
		Message:          deAnonymizedMessage,
		ReplacementRules: allRules,
	}, nil
}

// anonymizeMessages processes all input messages and anonymizes their content
func (s *PrivateCompletionsService) anonymizeMessages(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, interruptChan <-chan struct{}) ([]openai.ChatCompletionMessageParamUnion, map[string]string, error) {
	allRules := make(map[string]string)
	anonymizedMessages := make([]openai.ChatCompletionMessageParamUnion, len(messages))
	
	for i, message := range messages {
		// Check for interruption during processing
		select {
		case <-interruptChan:
			return nil, nil, fmt.Errorf("anonymization interrupted")
		default:
		}
		
		anonymizedMsg, rules, err := s.anonymizeMessage(ctx, message)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to anonymize message %d: %w", i, err)
		}
		
		anonymizedMessages[i] = anonymizedMsg
		
		// Merge rules (handle conflicts by keeping first occurrence)
		for token, original := range rules {
			if existing, exists := allRules[token]; exists && existing != original {
				s.logger.Warn("Rule conflict detected", "token", token, "existing", existing, "new", original)
			}
			allRules[token] = original
		}
	}
	
	return anonymizedMessages, allRules, nil
}

// anonymizeMessage anonymizes a single message
func (s *PrivateCompletionsService) anonymizeMessage(ctx context.Context, message openai.ChatCompletionMessageParamUnion) (openai.ChatCompletionMessageParamUnion, map[string]string, error) {
	// Convert message to JSON to extract content
	messageBytes, err := json.Marshal(message)
	if err != nil {
		return message, nil, fmt.Errorf("failed to marshal message: %w", err)
	}
	
	var messageMap map[string]interface{}
	if err := json.Unmarshal(messageBytes, &messageMap); err != nil {
		return message, nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}
	
	// Anonymize content field if it exists
	if content, exists := messageMap["content"]; exists {
		if contentStr, ok := content.(string); ok {
			anonymizedContent, rules, err := s.anonymizer.Anonymize(ctx, contentStr)
			if err != nil {
				return message, nil, fmt.Errorf("failed to anonymize content: %w", err)
			}
			
			messageMap["content"] = anonymizedContent
			
			// Convert back to the original message type
			anonymizedBytes, err := json.Marshal(messageMap)
			if err != nil {
				return message, nil, fmt.Errorf("failed to marshal anonymized message: %w", err)
			}
			
			var anonymizedMessage openai.ChatCompletionMessageParamUnion
			if err := json.Unmarshal(anonymizedBytes, &anonymizedMessage); err != nil {
				return message, nil, fmt.Errorf("failed to unmarshal anonymized message: %w", err)
			}
			
			return anonymizedMessage, rules, nil
		}
	}
	
	// If no content field or not a string, return as-is
	return message, make(map[string]string), nil
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