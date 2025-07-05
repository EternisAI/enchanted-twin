package holon

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"

	"github.com/EternisAI/enchanted-twin/graph/model"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

// ThreadProcessor handles LLM-based thread evaluation and state management.
type ThreadProcessor struct {
	logger           *log.Logger
	aiService        *ai.Service
	completionsModel string
	repo             *Repository
	memoryService    evolvingmemory.MemoryStorage
}

// NewThreadProcessor creates a new thread processor.
func NewThreadProcessor(
	logger *log.Logger,
	aiService *ai.Service,
	completionsModel string,
	repo *Repository,
	memoryService evolvingmemory.MemoryStorage,
) *ThreadProcessor {
	return &ThreadProcessor{
		logger:           logger,
		aiService:        aiService,
		completionsModel: completionsModel,
		repo:             repo,
		memoryService:    memoryService,
	}
}

// ThreadEvaluationResult represents the result of evaluating a thread.
type ThreadEvaluationResult struct {
	ShouldShow bool
	Reason     string
	Confidence float64
	NewState   string
}

// EvaluateThread uses LLM to determine if a thread is interesting for the user.
func (tp *ThreadProcessor) EvaluateThread(ctx context.Context, thread *model.Thread) (*ThreadEvaluationResult, error) {
	// Get user context from memory
	userContext, err := tp.getUserContext(ctx)
	if err != nil {
		tp.logger.Warn("Failed to get user context from memory", "error", err)
		userContext = "No specific user preferences available"
	}

	// Build evaluation prompt
	systemPrompt := tp.buildEvaluationSystemPrompt(userContext)
	userPrompt := tp.buildThreadEvaluationPrompt(thread)

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
		openai.UserMessage(userPrompt),
	}

	// Define evaluation tool
	evaluationTool := openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        "evaluate_thread_interest",
			Description: param.NewOpt("Evaluate whether a thread is interesting for the user based on their preferences and context"),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"is_interesting": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether the thread is interesting for the user",
					},
					"reason": map[string]interface{}{
						"type":        "string",
						"description": "Explanation for why the thread is or isn't interesting",
					},
					"confidence": map[string]interface{}{
						"type":        "number",
						"minimum":     0.0,
						"maximum":     1.0,
						"description": "Confidence level in the evaluation (0.0 to 1.0)",
					},
				},
				"required": []string{"is_interesting", "reason", "confidence"},
			},
		},
	}

	// Get LLM evaluation
	response, err := tp.aiService.Completions(ctx, messages, []openai.ChatCompletionToolParam{evaluationTool}, tp.completionsModel, ai.Background)
	if err != nil {
		return nil, fmt.Errorf("failed to get LLM evaluation: %w", err)
	}

	// Parse the response
	result, err := tp.parseEvaluationResponse(response.Message)
	if err != nil {
		return nil, fmt.Errorf("failed to parse evaluation response: %w", err)
	}

	tp.logger.Info("Thread evaluated",
		"thread_id", thread.ID,
		"is_interesting", result.ShouldShow,
		"reason", result.Reason,
		"confidence", result.Confidence,
		"new_state", result.NewState)

	return result, nil
}

// ProcessReceivedThreads processes all threads with 'received' state.
func (tp *ThreadProcessor) ProcessReceivedThreads(ctx context.Context) error {
	// Get all threads with 'received' state
	receivedThreads, err := tp.repo.GetThreadsByState(ctx, "received")
	if err != nil {
		return fmt.Errorf("failed to get received threads: %w", err)
	}

	tp.logger.Info("Processing received threads", "count", len(receivedThreads))

	for _, thread := range receivedThreads {
		if err := tp.ProcessSingleThread(ctx, thread); err != nil {
			tp.logger.Error("Failed to process thread", "thread_id", thread.ID, "error", err)
			// Continue processing other threads even if one fails
		}
	}

	return nil
}

// ProcessSingleThread processes a single thread and updates its state with evaluation data.
func (tp *ThreadProcessor) ProcessSingleThread(ctx context.Context, thread *model.Thread) error {
	evaluation, err := tp.EvaluateThread(ctx, thread)
	if err != nil {
		return fmt.Errorf("failed to evaluate thread %s: %w", thread.ID, err)
	}

	// Convert values to pointers for nullable parameters
	evaluatedBy := "llm-processor"

	// Update thread state with evaluation data using nullable pointer parameters
	if err := tp.repo.UpdateThreadWithEvaluation(
		ctx,
		thread.ID,
		evaluation.NewState,
		&evaluation.Reason,     // Pass pointer to allow nil
		&evaluation.Confidence, // Pass pointer to allow nil
		&evaluatedBy,           // Pass pointer to allow nil
	); err != nil {
		return fmt.Errorf("failed to update thread with evaluation: %w", err)
	}

	tp.logger.Info("Thread processed and evaluation stored",
		"thread_id", thread.ID,
		"old_state", "received",
		"new_state", evaluation.NewState,
		"reason", evaluation.Reason,
		"confidence", evaluation.Confidence)

	return nil
}

// getUserContext retrieves user preferences and context from memory.
func (tp *ThreadProcessor) getUserContext(ctx context.Context) (string, error) {
	if tp.memoryService == nil {
		return "No memory service available", nil
	}

	// Query for user preferences and interests
	result, err := tp.memoryService.Query(ctx, "user preferences interests hobbies work topics", nil)
	if err != nil {
		return "", err
	}

	if len(result.Facts) == 0 {
		return "No specific user preferences found in memory", nil
	}

	// Build context from memory facts
	var contextParts []string
	for _, fact := range result.Facts {
		if fact.Content != "" {
			contextParts = append(contextParts, fact.Content)
		}
	}

	if len(contextParts) == 0 {
		return "No detailed user context available", nil
	}

	return strings.Join(contextParts, "\n"), nil
}

// buildEvaluationSystemPrompt creates the system prompt for thread evaluation.
func (tp *ThreadProcessor) buildEvaluationSystemPrompt(userContext string) string {
	return fmt.Sprintf(`You are an intelligent content curator that helps filter threads based on user interests and preferences.

Your task is to evaluate whether a newly received thread would be interesting and relevant to the user based on:
1. The user's known preferences and interests
2. The content and context of the thread
3. The potential value or relevance to the user

User Context and Preferences:
%s

Guidelines for evaluation:
- Consider both explicit interests and implicit patterns in user behavior
- Prioritize threads that match known interests, work contexts, or personal situations
- Consider the potential utility or entertainment value
- Be conservative with low confidence - when in doubt, lean toward showing the thread
- Threads from close contacts or on important topics should generally be shown
- Spam, irrelevant advertisements, or clearly uninteresting content should be hidden

Use the evaluate_thread_interest tool to provide your assessment.`, userContext)
}

// buildThreadEvaluationPrompt creates the user prompt with thread details.
func (tp *ThreadProcessor) buildThreadEvaluationPrompt(thread *model.Thread) string {
	var messagePreview strings.Builder

	// Include first few messages as context
	messageCount := len(thread.Messages)
	previewCount := 3
	if messageCount > 0 {
		messagePreview.WriteString("\nRecent messages:\n")
		for i, msg := range thread.Messages {
			if i >= previewCount {
				messagePreview.WriteString(fmt.Sprintf("... and %d more messages", messageCount-previewCount))
				break
			}
			author := "Unknown"
			if msg.Author != nil {
				if msg.Author.Alias != nil && *msg.Author.Alias != "" {
					author = *msg.Author.Alias
				} else {
					author = msg.Author.Identity
				}
			}
			messagePreview.WriteString(fmt.Sprintf("- %s: %s\n", author, msg.Content))
		}
	}

	authorName := "Unknown"
	if thread.Author != nil {
		if thread.Author.Alias != nil && *thread.Author.Alias != "" {
			authorName = *thread.Author.Alias
		} else {
			authorName = thread.Author.Identity
		}
	}

	return fmt.Sprintf(`Please evaluate this thread for user interest:

Title: %s
Author: %s
Content: %s
Created: %s
Message Count: %d%s

Please assess whether this thread would be interesting and relevant to the user based on their preferences and context.`,
		thread.Title,
		authorName,
		thread.Content,
		thread.CreatedAt,
		len(thread.Messages),
		messagePreview.String())
}

// parseEvaluationResponse parses the LLM response and returns evaluation result.
func (tp *ThreadProcessor) parseEvaluationResponse(response openai.ChatCompletionMessage) (*ThreadEvaluationResult, error) {
	if len(response.ToolCalls) == 0 {
		// Default to showing if no tool call (conservative approach)
		return &ThreadEvaluationResult{
			ShouldShow: true,
			Reason:     "No evaluation provided, defaulting to show",
			Confidence: 0.5,
			NewState:   "visible",
		}, nil
	}

	toolCall := response.ToolCalls[0]
	if toolCall.Function.Name != "evaluate_thread_interest" {
		return nil, fmt.Errorf("unexpected tool call: %s", toolCall.Function.Name)
	}

	var evaluation struct {
		IsInteresting bool    `json:"is_interesting"`
		Reason        string  `json:"reason"`
		Confidence    float64 `json:"confidence"`
	}

	if err := ai.UnmarshalToolCall(toolCall, &evaluation); err != nil {
		return nil, fmt.Errorf("failed to unmarshal evaluation: %w", err)
	}

	// Determine new state based on evaluation
	newState := "hidden"
	if evaluation.IsInteresting {
		newState = "visible"
	}

	return &ThreadEvaluationResult{
		ShouldShow: evaluation.IsInteresting,
		Reason:     evaluation.Reason,
		Confidence: evaluation.Confidence,
		NewState:   newState,
	}, nil
}
