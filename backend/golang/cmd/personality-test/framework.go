package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/log"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/storage"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/chatgpt"
)

// TestingFramework manages personality testing operations
type TestingFramework struct {
	logger    *log.Logger
	storage   StorageInterface
	generator *ScenarioGenerator
	Memory    memory.MemoryInterface
}

// NewTestingFramework creates a new personality testing framework
func NewTestingFramework(storage StorageInterface, memoryInterface memory.MemoryInterface) *TestingFramework {
	logger := log.NewWithOptions(io.Discard, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		TimeFormat:      time.Kitchen,
	})

	return &TestingFramework{
		logger:    logger,
		storage:   storage,
		generator: NewScenarioGenerator(),
		Memory:    memoryInterface,
	}
}

// PersonalityTestResult represents the result of a personality test
type PersonalityTestResult struct {
	PersonalityID string                 `json:"personality_id"`
	ScenarioID    string                 `json:"scenario_id"`
	Score         float64                `json:"score"`
	Details       map[string]interface{} `json:"details"`
	Timestamp     time.Time              `json:"timestamp"`
	Responses     []ResponseMetrics      `json:"responses"`
}

// ResponseMetrics contains metrics for individual responses
type ResponseMetrics struct {
	Prompt           string            `json:"prompt"`
	Response         string            `json:"response"`
	ResponseTime     time.Duration     `json:"response_time"`
	EmotionalTone    string            `json:"emotional_tone"`
	ConsistencyScore float64           `json:"consistency_score"`
	Metadata         map[string]string `json:"metadata"`
}

// ConversationDocument represents a stored conversation with ID method
type ConversationDocument struct {
	DocumentID   string                        `json:"id"`
	Title        string                        `json:"title"`
	Messages     []chatgpt.ConversationMessage `json:"messages"`
	CreatedAt    string                        `json:"created_at"`
	UpdatedAt    string                        `json:"updated_at"`
	Participants []string                      `json:"participants"`
	Metadata     map[string]interface{}        `json:"metadata"`
}

// ID returns the document ID
func (cd *ConversationDocument) ID() string {
	return cd.DocumentID
}

// createThreadFromData creates a conversation thread from document data
func (tf *TestingFramework) createThreadFromData(data *storage.DocumentReference) ConversationThread {
	// Parse the document content into messages
	messages := parseDocumentIntoMessages(data.Content)

	return ConversationThread{
		ID:       data.ID,
		Messages: messages,
		// ...existing code...
	}
}

// parseDocumentIntoMessages parses document content into conversation messages
func parseDocumentIntoMessages(content string) []ConversationMessage {
	lines := strings.Split(content, "\n")
	var messages []ConversationMessage

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Simple parsing - look for "Speaker: Content" format
		if parts := strings.SplitN(line, ":", 2); len(parts) == 2 {
			speaker := strings.TrimSpace(parts[0])
			content := strings.TrimSpace(parts[1])

			messages = append(messages, ConversationMessage{
				Speaker:   speaker,
				Content:   content,
				Timestamp: time.Now(), // Use current time as placeholder
			})
		}
	}

	return messages
}

// RunPersonalityTest executes a personality test for a given scenario
func (tf *TestingFramework) RunPersonalityTest(ctx context.Context, personalityID string, scenario TestScenario) (*PersonalityTestResult, error) {
	tf.logger.Info("Running personality test", "personality_id", personalityID, "scenario_id", scenario.ID)

	startTime := time.Now()
	var responses []ResponseMetrics

	// Generate test data for the scenario
	testData, err := tf.generateTestData(scenario)
	if err != nil {
		return nil, fmt.Errorf("failed to generate test data: %w", err)
	}

	// Create conversation thread from test data
	thread, err := tf.createThreadFromData(testData)
	if err != nil {
		return nil, fmt.Errorf("failed to create thread: %w", err)
	}

	// Process each prompt in the scenario
	for i, prompt := range scenario.Prompts {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context canceled during test execution: %w", err)
		}

		promptStart := time.Now()

		// Simulate getting a response (in real implementation, this would call the AI)
		response := fmt.Sprintf("Test response for prompt %d: %s", i+1, prompt)

		responseTime := time.Since(promptStart)

		metrics := ResponseMetrics{
			Prompt:           prompt,
			Response:         response,
			ResponseTime:     responseTime,
			EmotionalTone:    "neutral", // Would be analyzed in real implementation
			ConsistencyScore: 0.85,      // Would be calculated in real implementation
			Metadata: map[string]string{
				"thread_id":    thread.ID(),
				"prompt_index": fmt.Sprintf("%d", i),
			},
		}
		responses = append(responses, metrics)
	}

	// Calculate overall score based on responses
	score := tf.calculatePersonalityScore(responses, scenario)

	result := &PersonalityTestResult{
		PersonalityID: personalityID,
		ScenarioID:    scenario.ID,
		Score:         score,
		Details: map[string]interface{}{
			"total_prompts":     len(scenario.Prompts),
			"execution_time_ms": time.Since(startTime).Milliseconds(),
			"scenario_type":     scenario.Type,
		},
		Timestamp: time.Now(),
		Responses: responses,
	}

	tf.logger.Info("Personality test completed",
		"personality_id", personalityID,
		"scenario_id", scenario.ID,
		"score", score,
		"response_count", len(responses))

	return result, nil
}

// generateTestData creates test conversation data for the scenario
func (tf *TestingFramework) generateTestData(scenario TestScenario) ([]chatgpt.ConversationMessage, error) {
	var messages []chatgpt.ConversationMessage

	// Create initial context message
	messages = append(messages, chatgpt.ConversationMessage{
		Role: "system",
		Text: scenario.Context,
	})

	// Add scenario-specific messages
	for _, prompt := range scenario.Prompts {
		messages = append(messages, chatgpt.ConversationMessage{
			Role: "user",
			Text: prompt,
		})
	}

	return messages, nil
}

// createThreadFromData creates a conversation thread from message data
func (tf *TestingFramework) createThreadFromData(messages []chatgpt.ConversationMessage) (*ConversationDocument, error) {
	threadID := fmt.Sprintf("test_thread_%d", time.Now().Unix())

	doc := &ConversationDocument{
		DocumentID:   threadID,
		Title:        "Personality Test Conversation",
		Messages:     messages,
		CreatedAt:    time.Now().Format(time.RFC3339),
		UpdatedAt:    time.Now().Format(time.RFC3339),
		Participants: []string{"system", "user"},
		Metadata: map[string]interface{}{
			"test_type": "personality",
			"generated": true,
		},
	}

	return doc, nil
}

// calculatePersonalityScore computes a personality score based on responses
func (tf *TestingFramework) calculatePersonalityScore(responses []ResponseMetrics, scenario TestScenario) float64 {
	if len(responses) == 0 {
		return 0.0
	}

	var totalScore float64
	for _, response := range responses {
		// Simple scoring based on response length and consistency
		lengthScore := float64(len(response.Response)) / 100.0
		if lengthScore > 1.0 {
			lengthScore = 1.0
		}

		responseScore := (lengthScore + response.ConsistencyScore) / 2.0
		totalScore += responseScore
	}

	return totalScore / float64(len(responses))
}

// SaveResults saves test results to a file
func (tf *TestingFramework) SaveResults(results []*PersonalityTestResult, outputPath string) error {
	tf.logger.Info("Saving test results", "count", len(results), "output_path", outputPath)

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(results); err != nil {
		return fmt.Errorf("failed to encode results: %w", err)
	}

	tf.logger.Info("Test results saved successfully", "output_path", outputPath)
	return nil
}

// LoadResults loads test results from a file
func (tf *TestingFramework) LoadResults(inputPath string) ([]*PersonalityTestResult, error) {
	tf.logger.Info("Loading test results", "input_path", inputPath)

	file, err := os.Open(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open input file: %w", err)
	}
	defer file.Close()

	var results []*PersonalityTestResult
	decoder := json.NewDecoder(file)

	if err := decoder.Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to decode results: %w", err)
	}

	tf.logger.Info("Test results loaded successfully", "count", len(results))
	return results, nil
}
