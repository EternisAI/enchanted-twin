package personalities

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/holon"
)

// TestEnvironment represents a complete test environment for a personality
type TestEnvironment struct {
	PersonalityName string
	Memory          evolvingmemory.MemoryStorage
	ThreadProcessor *holon.ThreadProcessor
	Repository      *holon.Repository
	MemoryTracker   *MemoryTracker
	Context         context.Context
}

// MemoryTracker tracks which memories are accessed during evaluation
type MemoryTracker struct {
	accessedMemories []string
	originalQuery    func(ctx context.Context, queryText string, filter *memory.Filter) (memory.QueryResult, error)
}

// NewMemoryTracker creates a new memory tracker
func NewMemoryTracker() *MemoryTracker {
	return &MemoryTracker{
		accessedMemories: make([]string, 0),
	}
}

// TrackMemoryAccess records that a memory was accessed
func (mt *MemoryTracker) TrackMemoryAccess(memoryID string) {
	mt.accessedMemories = append(mt.accessedMemories, memoryID)
}

// GetAccessedMemories returns the list of accessed memory IDs
func (mt *MemoryTracker) GetAccessedMemories() []string {
	return mt.accessedMemories
}

// Reset clears the tracked memories
func (mt *MemoryTracker) Reset() {
	mt.accessedMemories = mt.accessedMemories[:0]
}

// RunPersonalityTests runs all test scenarios against all personalities
func (ptf *PersonalityTestFramework) RunPersonalityTests(ctx context.Context, memoryStorage evolvingmemory.MemoryStorage, repository HolonRepositoryInterface) ([]TestResult, error) {
	results := make([]TestResult, 0)

	for personalityName, personality := range ptf.personalities {
		ptf.logger.Info("Testing personality", "name", personalityName)

		// Setup test environment for this personality
		env, err := ptf.setupTestEnvironment(ctx, personality, memoryStorage, repository)
		if err != nil {
			ptf.logger.Error("Failed to setup test environment", "personality", personalityName, "error", err)
			continue
		}

		// Run all scenarios for this personality
		for _, scenario := range ptf.scenarios {
			result, err := ptf.runSingleTest(ctx, env, scenario)
			if err != nil {
				ptf.logger.Error("Failed to run test", "personality", personalityName, "scenario", scenario.Name, "error", err)
				continue
			}

			results = append(results, *result)
		}
	}

	return results, nil
}

// setupTestEnvironment creates a test environment with personality data loaded into memory
func (ptf *PersonalityTestFramework) setupTestEnvironment(ctx context.Context, personality *ReferencePersonality, memoryStorage evolvingmemory.MemoryStorage, repository HolonRepositoryInterface) (*TestEnvironment, error) {
	// Create memory tracker
	tracker := NewMemoryTracker()

	// Store personality memory facts
	documents := make([]memory.Document, 0)

	// Add conversation documents
	for _, conv := range personality.Conversations {
		documents = append(documents, &conv)
	}

	// Store documents in memory
	if len(documents) > 0 {
		err := memoryStorage.Store(ctx, documents, func(processed, total int) {
			ptf.logger.Debug("Storing personality documents", "personality", personality.Name, "processed", processed, "total", total)
		})
		if err != nil {
			return nil, fmt.Errorf("failed to store personality documents: %w", err)
		}
	}

	// Store memory facts directly
	for _, fact := range personality.MemoryFacts {
		// Store each fact individually through the memory engine
		doc := &memory.TextDocument{
			FieldID:      fmt.Sprintf("personality-fact-%s-%d", personality.Name, time.Now().UnixNano()),
			FieldContent: fact.GenerateContent(),
		}

		err := memoryStorage.Store(ctx, []memory.Document{doc}, nil)
		if err != nil {
			ptf.logger.Warn("Failed to store memory fact", "fact", fact.GenerateContent(), "error", err)
		}
	}

	// Create thread processor with memory tracking
	var threadProcessor *holon.ThreadProcessor
	var holonRepo *holon.Repository

	// Try to get concrete repository for full functionality
	if repo, ok := repository.(*holon.Repository); ok {
		holonRepo = repo
		threadProcessor = holon.NewThreadProcessor(
			ptf.logger,
			ptf.aiService,
			"gpt-4o-mini", // Use a consistent model for testing
			repo,
			memoryStorage,
		)
	} else {
		// For mock testing, create a minimal processor
		// This allows us to test the framework logic without requiring a real database
		ptf.logger.Info("Using mock repository, creating simplified test environment")

		// Return a test environment that can simulate thread processing
		return &TestEnvironment{
			PersonalityName: personality.Name,
			Memory:          memoryStorage,
			ThreadProcessor: nil, // Will be handled in test execution
			Repository:      nil, // Mock doesn't need concrete repository
			MemoryTracker:   tracker,
			Context:         ctx,
		}, nil
	}

	return &TestEnvironment{
		PersonalityName: personality.Name,
		Memory:          memoryStorage,
		ThreadProcessor: threadProcessor,
		Repository:      holonRepo,
		MemoryTracker:   tracker,
		Context:         ctx,
	}, nil
}

// runSingleTest runs a single test scenario against a personality
func (ptf *PersonalityTestFramework) runSingleTest(ctx context.Context, env *TestEnvironment, scenario ThreadTestScenario) (*TestResult, error) {
	// Reset memory tracker
	env.MemoryTracker.Reset()

	ptf.logger.Info("Running test", "personality", env.PersonalityName, "scenario", scenario.Name)

	var evaluation *holon.ThreadEvaluationResult
	var err error

	// Execute thread evaluation
	if env.ThreadProcessor != nil {
		// Use real thread processor for full integration testing
		evaluation, err = env.ThreadProcessor.EvaluateThread(ctx, scenario.Thread)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate thread: %w", err)
		}
	} else {
		// For mock testing, simulate thread evaluation based on expected results
		// This allows us to test the framework logic without requiring a real database
		ptf.logger.Debug("Using simulated thread evaluation for mock testing")

		evaluation = &holon.ThreadEvaluationResult{
			ShouldShow: scenario.Expected.ShouldShow,
			Reason:     fmt.Sprintf("Simulated evaluation for %s personality", env.PersonalityName),
			Confidence: scenario.Expected.Confidence,
			NewState:   scenario.Expected.ExpectedState,
		}
	}

	// Use LLM-as-a-judge to evaluate the result
	score, reasoning, err := ptf.evaluateWithLLMJudge(ctx, scenario, evaluation)
	if err != nil {
		ptf.logger.Warn("LLM judge evaluation failed", "error", err)
		score = ptf.calculateBasicScore(scenario.Expected, evaluation)
		reasoning = "LLM judge failed, using basic scoring"
	}

	// Determine success based on score threshold
	success := score >= 0.7 // 70% threshold for success

	result := &TestResult{
		PersonalityName: env.PersonalityName,
		ScenarioName:    scenario.Name,
		Success:         success,
		Score:           score,
		ActualResult:    evaluation,
		ExpectedResult:  scenario.Expected,
		MemoriesUsed:    env.MemoryTracker.GetAccessedMemories(),
		Reasoning:       reasoning,
		Timestamp:       time.Now(),
	}

	ptf.logger.Info("Test completed",
		"personality", env.PersonalityName,
		"scenario", scenario.Name,
		"success", success,
		"score", score)

	return result, nil
}

// evaluateWithLLMJudge uses an LLM to evaluate how well the actual result matches the expected result
func (ptf *PersonalityTestFramework) evaluateWithLLMJudge(ctx context.Context, scenario ThreadTestScenario, actual *holon.ThreadEvaluationResult) (float64, string, error) {
	systemPrompt := `You are an expert judge evaluating whether an AI agent's thread evaluation matches the expected behavior for a specific personality.

Your task is to compare the actual evaluation result with the expected result and provide a similarity score from 0.0 to 1.0, where:
- 1.0 = Perfect match
- 0.8-0.9 = Very good match with minor differences
- 0.6-0.7 = Good match with some differences
- 0.4-0.5 = Partial match with significant differences
- 0.0-0.3 = Poor match or opposite behavior

Consider these factors:
1. Does the ShouldShow decision match expectations?
2. Is the confidence level reasonable?
3. Does the reasoning contain expected keywords or concepts?
4. Is the overall evaluation consistent with the personality profile?

Be nuanced in your evaluation - small differences in confidence or reasoning style should not heavily penalize the score if the core decision is correct.`

	userPrompt := fmt.Sprintf(`Thread Scenario: %s
Description: %s

Thread Content:
Title: %s
Content: %s
Author: %s

Expected Result:
- Should Show: %t
- Expected State: %s
- Confidence: %.2f
- Expected Keywords: %v
- Priority: %d

Actual Result:
- Should Show: %t
- New State: %s
- Confidence: %.2f
- Reason: %s

Please evaluate how well the actual result matches the expected behavior and provide your assessment.`,
		scenario.Name,
		scenario.Description,
		scenario.Thread.Title,
		scenario.Thread.Content,
		scenario.Thread.Author.Identity,
		scenario.Expected.ShouldShow,
		scenario.Expected.ExpectedState,
		scenario.Expected.Confidence,
		scenario.Expected.ReasonKeywords,
		scenario.Expected.Priority,
		actual.ShouldShow,
		actual.NewState,
		actual.Confidence,
		actual.Reason)

	// Define evaluation tool
	evaluationTool := openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        "evaluate_match",
			Description: param.NewOpt("Evaluate how well the actual result matches the expected result"),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"score": map[string]interface{}{
						"type":        "number",
						"minimum":     0.0,
						"maximum":     1.0,
						"description": "Similarity score from 0.0 to 1.0",
					},
					"reasoning": map[string]interface{}{
						"type":        "string",
						"description": "Detailed explanation of the evaluation and score",
					},
					"key_differences": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "List of key differences between expected and actual results",
					},
				},
				"required": []string{"score", "reasoning"},
			},
		},
	}

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
		openai.UserMessage(userPrompt),
	}

	response, err := ptf.aiService.Completions(ctx, messages, []openai.ChatCompletionToolParam{evaluationTool}, "gpt-4o-mini")
	if err != nil {
		return 0, "", err
	}

	// Parse the response
	if len(response.ToolCalls) == 0 {
		return 0, "No evaluation provided", fmt.Errorf("no tool call in response")
	}

	toolCall := response.ToolCalls[0]
	if toolCall.Function.Name != "evaluate_match" {
		return 0, "", fmt.Errorf("unexpected tool call: %s", toolCall.Function.Name)
	}

	var evaluation struct {
		Score          float64  `json:"score"`
		Reasoning      string   `json:"reasoning"`
		KeyDifferences []string `json:"key_differences"`
	}

	if err := ai.UnmarshalToolCall(toolCall, &evaluation); err != nil {
		return 0, "", fmt.Errorf("failed to unmarshal evaluation: %w", err)
	}

	return evaluation.Score, evaluation.Reasoning, nil
}

// calculateBasicScore provides a fallback scoring mechanism when LLM judge fails
func (ptf *PersonalityTestFramework) calculateBasicScore(expected ExpectedThreadEvaluation, actual *holon.ThreadEvaluationResult) float64 {
	score := 0.0

	// Core decision match (most important)
	if expected.ShouldShow == actual.ShouldShow {
		score += 0.6
	}

	// State match
	if expected.ExpectedState == actual.NewState {
		score += 0.2
	}

	// Confidence similarity (within 0.3 range)
	confidenceDiff := abs(expected.Confidence - actual.Confidence)
	if confidenceDiff <= 0.3 {
		score += 0.1
	}

	// Keyword matching in reasoning
	keywordMatches := 0
	reasonLower := strings.ToLower(actual.Reason)
	for _, keyword := range expected.ReasonKeywords {
		if strings.Contains(reasonLower, strings.ToLower(keyword)) {
			keywordMatches++
		}
	}
	if len(expected.ReasonKeywords) > 0 {
		keywordScore := float64(keywordMatches) / float64(len(expected.ReasonKeywords))
		score += keywordScore * 0.1
	}

	return score
}

// abs returns the absolute value of a float64
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
