package ai

import (
	"context"

	"github.com/openai/openai-go"
)

// ServiceAdapter wraps PrivateCompletions to provide the full Service interface
// This allows seamless replacement of the old Service with the new private completions
type ServiceAdapter struct {
	privateCompletions PrivateCompletions
	originalService    *Service // For embeddings and other non-completion methods
	defaultPriority    Priority
}

// NewServiceAdapter creates an adapter that wraps PrivateCompletions and delegates other methods to the original service
func NewServiceAdapter(privateCompletions PrivateCompletions, originalService *Service, defaultPriority Priority) *ServiceAdapter {
	return &ServiceAdapter{
		privateCompletions: privateCompletions,
		originalService:    originalService,
		defaultPriority:    defaultPriority,
	}
}

// Completions implements the Service interface by calling the new PrivateCompletions interface
func (a *ServiceAdapter) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (PrivateResult, error) {
	return a.privateCompletions.Completions(ctx, messages, tools, model, a.defaultPriority)
}

// CompletionsWithPriority allows specifying custom priority while maintaining the old interface
func (a *ServiceAdapter) CompletionsWithPriority(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string, priority Priority) (PrivateResult, error) {
	return a.privateCompletions.Completions(ctx, messages, tools, model, priority)
}

// GetReplacementRules provides access to anonymization rules from the last completion
// This is useful for debugging or auditing purposes
func (a *ServiceAdapter) GetReplacementRules(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string, priority Priority) (PrivateResult, error) {
	return a.privateCompletions.Completions(ctx, messages, tools, model, priority)
}

// ParamsCompletions delegates to the original service's ParamsCompletions method
func (a *ServiceAdapter) ParamsCompletions(ctx context.Context, params openai.ChatCompletionNewParams) (openai.ChatCompletionMessage, error) {
	// For now, delegate to original service - we could enhance this to use private completions with params conversion
	return a.originalService.ParamsCompletions(ctx, params)
}

// CompletionsStream delegates to the original service's streaming method
func (a *ServiceAdapter) CompletionsStream(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) Stream {
	// For now, delegate to original service - streaming with anonymization would need special handling
	return a.originalService.CompletionsStream(ctx, messages, tools, model)
}

// Embeddings delegates to the original service (embeddings don't need anonymization)
func (a *ServiceAdapter) Embeddings(ctx context.Context, inputs []string, model string) ([][]float64, error) {
	return a.originalService.Embeddings(ctx, inputs, model)
}

// Embedding delegates to the original service (embeddings don't need anonymization)
func (a *ServiceAdapter) Embedding(ctx context.Context, input string, model string) ([]float64, error) {
	return a.originalService.Embedding(ctx, input, model)
}

// FallbackCompletionsService provides a fallback implementation when private completions are disabled
type FallbackCompletionsService struct {
	completionsService CompletionsService
}

// NewFallbackCompletionsService creates a fallback service that wraps the regular completions service
func NewFallbackCompletionsService(completionsService CompletionsService) PrivateCompletions {
	return &FallbackCompletionsService{
		completionsService: completionsService,
	}
}

// Completions implements PrivateCompletions without anonymization (fallback mode)
func (f *FallbackCompletionsService) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string, priority Priority) (PrivateResult, error) {
	// Call the underlying service directly (no anonymization)
	result, err := f.completionsService.Completions(ctx, messages, tools, model)
	if err != nil {
		return PrivateResult{}, err
	}
	
	// Return the result directly (already contains Message and ReplacementRules)
	return result, nil
}