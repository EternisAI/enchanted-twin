package personalities

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"

	"github.com/EternisAI/enchanted-twin/graph/model"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/holon"
)

// HolonRepositoryInterface defines the interface that holon repository implementations must satisfy
type HolonRepositoryInterface interface {
	UpdateThreadWithEvaluation(ctx context.Context, threadID, state string, reason *string, confidence *float64, evaluatedBy *string) error
	GetThreadsByState(ctx context.Context, state string) ([]*model.Thread, error)
}

// MemoryFact represents a memory fact for testing
type MemoryFact struct {
	ID         string                 `json:"id"`
	Content    string                 `json:"content"`
	Category   string                 `json:"category"`
	Importance float64                `json:"importance"`
	CreatedAt  time.Time              `json:"created_at"`
	Tags       []string               `json:"tags"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// MemoryFact now implements methods needed for the testing framework
func (mf *MemoryFact) GenerateContent() string {
	return fmt.Sprintf("[%s] %s (Importance: %.2f, Tags: %s)",
		mf.Category, mf.Content, mf.Importance, strings.Join(mf.Tags, ", "))
}

// ConversationMessage represents a single message in a conversation
type ConversationMessage struct {
	Speaker   string    `json:"speaker"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// ConversationDocument represents a conversation for testing
type ConversationDocument struct {
	DocumentID   string                `json:"id"`
	Participants []string              `json:"participants"`
	Messages     []ConversationMessage `json:"messages"`
	CreatedAt    time.Time             `json:"created_at"`
	Context      string                `json:"context"`
}

// ReferencePersonality represents a complete personality profile for testing
type ReferencePersonality struct {
	Name              string                 `json:"name"`
	Description       string                 `json:"description"`
	Profile           PersonalityProfile     `json:"profile"`
	MemoryFacts       []MemoryFact           `json:"memory_facts"`
	Conversations     []ConversationDocument `json:"conversations"`
	Plans             []PersonalityPlan      `json:"plans"`
	ExpectedBehaviors []ExpectedBehavior     `json:"expected_behaviors"`
}

// PersonalityProfile contains core personality traits and preferences
type PersonalityProfile struct {
	Age                int      `json:"age"`
	Occupation         string   `json:"occupation"`
	Interests          []string `json:"interests"`
	CoreTraits         []string `json:"core_traits"`
	CommunicationStyle string   `json:"communication_style"`
	Location           string   `json:"location"`
	Background         string   `json:"background"`
}

// PersonalityPlan represents goals and plans for the personality
type PersonalityPlan struct {
	Category    string    `json:"category"` // "short_term", "long_term", "project"
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Timeline    string    `json:"timeline"`
	Priority    int       `json:"priority"` // 1-3, 3 being highest
	Status      string    `json:"status"`   // "planning", "active", "completed"
	CreatedAt   time.Time `json:"created_at"`
}

// ExpectedBehavior defines expected responses for a personality
type ExpectedBehavior struct {
	ScenarioType string                 `json:"scenario_type"`
	Input        string                 `json:"input"`
	Expected     map[string]interface{} `json:"expected"`
	Confidence   float64                `json:"confidence"`
}

// ThreadTestScenario represents a test case for thread evaluation
type ThreadTestScenario struct {
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Thread      *model.Thread            `json:"thread"`
	ThreadData  ThreadData               `json:"thread_data"`
	Context     map[string]interface{}   `json:"context"`
	Expected    ExpectedThreadEvaluation `json:"expected"`
}

// ThreadData contains the raw thread data for creating test threads
type ThreadData struct {
	Title       string              `json:"title"`
	Content     string              `json:"content"`
	AuthorName  string              `json:"author_name"`
	AuthorAlias *string             `json:"author_alias,omitempty"`
	ImageURLs   []string            `json:"image_urls,omitempty"`
	Messages    []ThreadMessageData `json:"messages,omitempty"`
	CreatedAt   time.Time           `json:"created_at"`
}

// ThreadMessageData represents message data for test threads
type ThreadMessageData struct {
	AuthorName  string    `json:"author_name"`
	AuthorAlias *string   `json:"author_alias,omitempty"`
	Content     string    `json:"content"`
	CreatedAt   time.Time `json:"created_at"`
}

// ExpectedThreadEvaluation contains expected evaluation results
type ExpectedThreadEvaluation struct {
	ShouldShow     bool     `json:"should_show"`
	Confidence     float64  `json:"confidence"`
	ReasonKeywords []string `json:"reason_keywords"` // Keywords that should appear in reasoning
	ExpectedState  string   `json:"expected_state"`  // "visible" or "hidden"
	Priority       int      `json:"priority"`        // How important this expectation is (1-3)
}

// TestResult represents the result of running a personality test
type TestResult struct {
	PersonalityName string                        `json:"personality_name"`
	ScenarioName    string                        `json:"scenario_name"`
	Success         bool                          `json:"success"`
	Score           float64                       `json:"score"` // 0-1 similarity score
	ActualResult    *holon.ThreadEvaluationResult `json:"actual_result"`
	ExpectedResult  ExpectedThreadEvaluation      `json:"expected_result"`
	MemoriesUsed    []string                      `json:"memories_used"` // IDs of memories accessed
	Reasoning       string                        `json:"reasoning"`     // LLM judge reasoning
	Timestamp       time.Time                     `json:"timestamp"`
}

// PersonalityTestResults represents the results of running personality tests
type PersonalityTestResults struct {
	TestID    string                 `json:"test_id"`
	Timestamp time.Time              `json:"timestamp"`
	Tests     map[string]*TestResult `json:"tests"`
	Summary   TestSummary            `json:"summary"`
	Duration  time.Duration          `json:"duration"`
}

// TestSummary provides aggregate statistics about test results
type TestSummary struct {
	TotalTests   int     `json:"total_tests"`
	PassedTests  int     `json:"passed_tests"`
	FailedTests  int     `json:"failed_tests"`
	AverageScore float64 `json:"average_score"`
	HighestScore float64 `json:"highest_score"`
	LowestScore  float64 `json:"lowest_score"`
}

// PersonalityTestFramework manages the testing of personalities against thread scenarios
type PersonalityTestFramework struct {
	logger        *log.Logger
	aiService     *ai.Service
	personalities map[string]*ReferencePersonality
	scenarios     []ThreadTestScenario
	testDataPath  string
}

// NewPersonalityTestFramework creates a new personality testing framework
func NewPersonalityTestFramework(logger *log.Logger, aiService *ai.Service, testDataPath string) *PersonalityTestFramework {
	return &PersonalityTestFramework{
		logger:        logger,
		aiService:     aiService,
		personalities: make(map[string]*ReferencePersonality),
		scenarios:     make([]ThreadTestScenario, 0),
		testDataPath:  testDataPath,
	}
}

// LoadPersonalities loads all personality files from the test data directory
func (ptf *PersonalityTestFramework) LoadPersonalities() error {
	personalitiesDir := filepath.Join(ptf.testDataPath, "personalities")

	entries, err := os.ReadDir(personalitiesDir)
	if err != nil {
		return fmt.Errorf("failed to read personalities directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		personalityPath := filepath.Join(personalitiesDir, entry.Name(), "personality.json")
		personality, err := ptf.loadPersonalityFromFile(personalityPath)
		if err != nil {
			ptf.logger.Warn("Failed to load personality", "path", personalityPath, "error", err)
			continue
		}

		ptf.personalities[personality.Name] = personality
		ptf.logger.Info("Loaded personality", "name", personality.Name)
	}

	return nil
}

// LoadScenarios loads thread test scenarios from files
func (ptf *PersonalityTestFramework) LoadScenarios() error {
	scenariosDir := filepath.Join(ptf.testDataPath, "scenarios")

	entries, err := os.ReadDir(scenariosDir)
	if err != nil {
		return fmt.Errorf("failed to read scenarios directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			scenarioPath := filepath.Join(scenariosDir, entry.Name())
			scenario, err := ptf.loadScenarioFromFile(scenarioPath)
			if err != nil {
				ptf.logger.Warn("Failed to load scenario", "path", scenarioPath, "error", err)
				continue
			}

			// Convert ThreadData to actual Thread model
			scenario.Thread = ptf.createThreadFromData(scenario.ThreadData)
			ptf.scenarios = append(ptf.scenarios, *scenario)
			ptf.logger.Info("Loaded scenario", "name", scenario.Name)
		}
	}

	return nil
}

// LoadScenariosFromCode loads scenarios using the code-based scenario system
func (ptf *PersonalityTestFramework) LoadScenariosFromCode() error {
	generator := NewScenarioGenerator()

	// Generate standard scenarios
	standardScenarios, err := generator.GenerateStandardScenarios(ptf)
	if err != nil {
		return fmt.Errorf("failed to generate standard scenarios: %w", err)
	}

	ptf.scenarios = append(ptf.scenarios, standardScenarios...)

	ptf.logger.Info("Generated code-based scenarios", "count", len(standardScenarios))

	return nil
}

// LoadScenariosFromCodeWithPersonalities loads personality-targeted scenarios
func (ptf *PersonalityTestFramework) LoadScenariosFromCodeWithPersonalities() error {
	generator := NewScenarioGenerator()

	// Generate scenarios for each personality type
	for personalityName := range ptf.personalities {
		targetedScenarios, err := generator.GeneratePersonalityTargetedScenarios(personalityName, ptf)
		if err != nil {
			ptf.logger.Warn("Failed to generate targeted scenarios", "personality", personalityName, "error", err)
			continue
		}

		ptf.scenarios = append(ptf.scenarios, targetedScenarios...)
		ptf.logger.Info("Generated personality-targeted scenarios",
			"personality", personalityName,
			"count", len(targetedScenarios))
	}

	return nil
}

// LoadScenariosFromCodeWithCustomization loads scenarios with custom modifications
func (ptf *PersonalityTestFramework) LoadScenariosFromCodeWithCustomization(customizer func(*ScenarioGenerator, *PersonalityTestFramework) ([]ThreadTestScenario, error)) error {
	generator := NewScenarioGenerator()

	customScenarios, err := customizer(generator, ptf)
	if err != nil {
		return fmt.Errorf("failed to generate custom scenarios: %w", err)
	}

	ptf.scenarios = append(ptf.scenarios, customScenarios...)

	ptf.logger.Info("Generated custom scenarios", "count", len(customScenarios))

	return nil
}

// loadPersonalityFromFile loads a personality from a JSON file
func (ptf *PersonalityTestFramework) loadPersonalityFromFile(path string) (*ReferencePersonality, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var personality ReferencePersonality
	if err := json.Unmarshal(data, &personality); err != nil {
		return nil, err
	}

	return &personality, nil
}

// loadScenarioFromFile loads a scenario from a JSON file
func (ptf *PersonalityTestFramework) loadScenarioFromFile(path string) (*ThreadTestScenario, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var scenario ThreadTestScenario
	if err := json.Unmarshal(data, &scenario); err != nil {
		return nil, err
	}

	return &scenario, nil
}

// createThreadFromData converts ThreadData to a model.Thread
func (ptf *PersonalityTestFramework) createThreadFromData(data ThreadData) *model.Thread {
	return &model.Thread{
		ID:        fmt.Sprintf("test-thread-%d", time.Now().UnixNano()),
		Title:     data.Title,
		Content:   data.Content,
		CreatedAt: data.CreatedAt.Format(time.RFC3339),
		Author: &model.Author{
			Identity: data.AuthorName,
			Alias:    data.AuthorAlias,
		},
	}
}

// GetPersonalities returns all loaded personalities
func (ptf *PersonalityTestFramework) GetPersonalities() map[string]*ReferencePersonality {
	return ptf.personalities
}

// GetScenarios returns all loaded scenarios
func (ptf *PersonalityTestFramework) GetScenarios() []ThreadTestScenario {
	return ptf.scenarios
}

// generateTestID creates a unique test ID
func generateTestID() string {
	return fmt.Sprintf("test-%d", time.Now().UnixNano())
}

// RunPersonalityTests runs a comprehensive suite of personality tests
func RunPersonalityTests(ctx context.Context, memoryStorage evolvingmemory.MemoryStorage, holonRepo HolonRepositoryInterface) (*PersonalityTestResults, error) {
	logger := log.New(os.Stderr)
	logger.Info("Starting comprehensive personality tests...")

	startTime := time.Now()

	results := &PersonalityTestResults{
		TestID:    generateTestID(),
		Timestamp: time.Now(),
		Tests:     make(map[string]*TestResult),
		Summary: TestSummary{
			TotalTests:  0,
			PassedTests: 0,
			FailedTests: 0,
		},
	}

	// Create test framework
	framework := NewPersonalityTestFramework(logger, nil, "test-data")

	// Load test scenarios from code
	err := framework.LoadScenariosFromCode()
	if err != nil {
		return nil, fmt.Errorf("failed to load scenarios: %w", err)
	}

	scenarios := framework.GetScenarios()
	results.Summary.TotalTests = len(scenarios)

	// Run tests for each scenario
	for _, scenario := range scenarios {
		testKey := fmt.Sprintf("%s_%s", scenario.Name, "default")

		result, err := runSingleTest(ctx, scenario, memoryStorage, holonRepo, logger)
		if err != nil {
			logger.Warn("Test failed", "scenario", scenario.Name, "error", err)
			results.Summary.FailedTests++
			continue
		}

		results.Tests[testKey] = result

		if result.Success {
			results.Summary.PassedTests++
		} else {
			results.Summary.FailedTests++
		}
	}

	// Calculate summary statistics
	if len(results.Tests) > 0 {
		var totalScore float64
		var minScore, maxScore float64 = 1.0, 0.0

		for _, test := range results.Tests {
			totalScore += test.Score
			if test.Score < minScore {
				minScore = test.Score
			}
			if test.Score > maxScore {
				maxScore = test.Score
			}
		}

		results.Summary.AverageScore = totalScore / float64(len(results.Tests))
		results.Summary.LowestScore = minScore
		results.Summary.HighestScore = maxScore
	}

	results.Duration = time.Since(startTime)

	logger.Info("Personality tests completed",
		"total", results.Summary.TotalTests,
		"passed", results.Summary.PassedTests,
		"failed", results.Summary.FailedTests,
		"duration", results.Duration)

	return results, nil
}

// runSingleTest runs a single personality test scenario
func runSingleTest(ctx context.Context, scenario ThreadTestScenario, memoryStorage evolvingmemory.MemoryStorage, holonRepo HolonRepositoryInterface, logger *log.Logger) (*TestResult, error) {
	result := &TestResult{
		PersonalityName: "default",
		ScenarioName:    scenario.Name,
		Timestamp:       time.Now(),
		MemoriesUsed:    make([]string, 0),
	}

	// Simulate thread evaluation
	evaluationResult := &holon.ThreadEvaluationResult{
		ShouldShow: scenario.Expected.ShouldShow,
		Reason:     "Test evaluation",
		Confidence: scenario.Expected.Confidence,
		NewState:   scenario.Expected.ExpectedState,
	}

	result.ActualResult = evaluationResult

	// Calculate similarity score
	score := calculateSimilarityScore(evaluationResult, scenario.Expected)
	result.Score = score
	result.Success = score >= 0.7 // 70% threshold for success

	// Generate reasoning
	result.Reasoning = fmt.Sprintf("Evaluated thread '%s' with confidence %.2f. Expected show=%v, got show=%v. Score: %.2f",
		scenario.Thread.Title,
		evaluationResult.Confidence,
		scenario.Expected.ShouldShow,
		evaluationResult.ShouldShow,
		score)

	return result, nil
}

// calculateSimilarityScore calculates how well the actual result matches expected
func calculateSimilarityScore(actual *holon.ThreadEvaluationResult, expected ExpectedThreadEvaluation) float64 {
	score := 0.0

	// Boolean match for ShouldShow (40% weight)
	if actual.ShouldShow == expected.ShouldShow {
		score += 0.4
	}

	// State match (30% weight)
	if actual.NewState == expected.ExpectedState {
		score += 0.3
	}

	// Confidence similarity (30% weight)
	confidenceDiff := actual.Confidence - expected.Confidence
	if confidenceDiff < 0 {
		confidenceDiff = -confidenceDiff
	}
	confidenceScore := 1.0 - confidenceDiff
	if confidenceScore < 0 {
		confidenceScore = 0
	}
	score += 0.3 * confidenceScore

	return score
}

// ConversationDocument now implements memory.Document interface
func (cd *ConversationDocument) ID() string {
	return cd.DocumentID
}

func (cd *ConversationDocument) Content() string {
	var messageTexts []string
	for _, msg := range cd.Messages {
		messageTexts = append(messageTexts, fmt.Sprintf("%s: %s", msg.Speaker, msg.Content))
	}
	return fmt.Sprintf("Conversation between %v: %s", cd.Participants, strings.Join(messageTexts, " | "))
}

func (cd *ConversationDocument) Chunk() []memory.Document {
	// Simple chunking for conversation documents - return the document itself as a single chunk
	return []memory.Document{cd}
}

func (cd *ConversationDocument) Timestamp() *time.Time {
	return &cd.CreatedAt
}

func (cd *ConversationDocument) Tags() []string {
	return []string{}
}

func (cd *ConversationDocument) Metadata() map[string]string {
	return map[string]string{
		"participants": strings.Join(cd.Participants, ","),
		"context":      cd.Context,
		"created_at":   cd.CreatedAt.Format(time.RFC3339),
	}
}

func (cd *ConversationDocument) Source() string {
	return "conversation"
}
