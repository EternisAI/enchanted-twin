package personalities

import (
	"context"
	"time"

	"github.com/EternisAI/enchanted-twin/graph/model"
)

// HolonRepositoryInterface defines the interface that holon repository implementations must satisfy.
type HolonRepositoryInterface interface {
	UpdateThreadWithEvaluation(ctx context.Context, threadID, state string, reason *string, confidence *float64, evaluatedBy *string) error
	GetThreadsByState(ctx context.Context, state string) ([]*model.Thread, error)
}

// MemoryFact represents a memory fact for testing.
type MemoryFact struct {
	ID         string                 `json:"id"`
	Content    string                 `json:"content"`
	Category   string                 `json:"category"`
	Importance float64                `json:"importance"`
	CreatedAt  time.Time              `json:"created_at"`
	Tags       []string               `json:"tags"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// ConversationMessage represents a single message in a conversation.
type ConversationMessage struct {
	Speaker   string    `json:"speaker"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// ConversationDocument represents a conversation for testing.
type ConversationDocument struct {
	DocumentID   string                `json:"id"`
	Participants []string              `json:"participants"`
	Messages     []ConversationMessage `json:"messages"`
	CreatedAt    time.Time             `json:"created_at"`
	Context      string                `json:"context"`
}

// PersonalityProfile contains core personality traits and preferences.
type PersonalityProfile struct {
	Age                int      `json:"age"`
	Occupation         string   `json:"occupation"`
	Interests          []string `json:"interests"`
	CoreTraits         []string `json:"core_traits"`
	CommunicationStyle string   `json:"communication_style"`
	Location           string   `json:"location"`
	Background         string   `json:"background"`
}

// PersonalityPlan represents goals and plans for the personality.
type PersonalityPlan struct {
	Category    string    `json:"category"` // "short_term", "long_term", "project"
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Timeline    string    `json:"timeline"`
	Priority    int       `json:"priority"` // 1-3, 3 being highest
	Status      string    `json:"status"`   // "planning", "active", "completed"
	CreatedAt   time.Time `json:"created_at"`
}

// ExpectedBehavior defines expected responses for a personality.
type ExpectedBehavior struct {
	ScenarioType string                 `json:"scenario_type"`
	Input        string                 `json:"input"`
	Expected     map[string]interface{} `json:"expected"`
	Confidence   float64                `json:"confidence"`
}

// BasePersonality represents the core personality data without test-specific expectations.
type BasePersonality struct {
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	Profile       PersonalityProfile     `json:"profile"`
	MemoryFacts   []MemoryFact           `json:"memory_facts"`
	Conversations []ConversationDocument `json:"conversations"`
	Plans         []PersonalityPlan      `json:"plans"`
}

// PersonalityExtension contains test-specific extensions and modifications.
type PersonalityExtension struct {
	TestName          string              `json:"test_name"`
	Description       string              `json:"description"`
	AdditionalFacts   []MemoryFact        `json:"additional_facts,omitempty"`
	AdditionalPlans   []PersonalityPlan   `json:"additional_plans,omitempty"`
	ProfileOverrides  *PersonalityProfile `json:"profile_overrides,omitempty"`
	ExpectedBehaviors []ExpectedBehavior  `json:"expected_behaviors"`
	Tags              []string            `json:"tags,omitempty"`
}

// ReferencePersonality represents a complete personality profile for testing.
type ReferencePersonality struct {
	Name              string                 `json:"name"`
	Description       string                 `json:"description"`
	Profile           PersonalityProfile     `json:"profile"`
	MemoryFacts       []MemoryFact           `json:"memory_facts"`
	Conversations     []ConversationDocument `json:"conversations"`
	Plans             []PersonalityPlan      `json:"plans"`
	ExpectedBehaviors []ExpectedBehavior     `json:"expected_behaviors"`
}

// ExtendedPersonality combines a base personality with one or more extensions for testing.
type ExtendedPersonality struct {
	Base       *BasePersonality        `json:"base"`
	Extensions []*PersonalityExtension `json:"extensions"`
	TestID     string                  `json:"test_id"`
	CreatedAt  time.Time               `json:"created_at"`
}

// ThreadData contains the raw thread data for creating test threads.
type ThreadData struct {
	Title       string              `json:"title"`
	Content     string              `json:"content"`
	AuthorName  string              `json:"author_name"`
	AuthorAlias *string             `json:"author_alias,omitempty"`
	ImageURLs   []string            `json:"image_urls,omitempty"`
	Messages    []ThreadMessageData `json:"messages,omitempty"`
	CreatedAt   time.Time           `json:"created_at"`
}

// ThreadMessageData represents message data for test threads.
type ThreadMessageData struct {
	AuthorName  string    `json:"author_name"`
	AuthorAlias *string   `json:"author_alias,omitempty"`
	Content     string    `json:"content"`
	CreatedAt   time.Time `json:"created_at"`
}

// PersonalityExpectedOutcome defines what a specific personality should do with a scenario.
type PersonalityExpectedOutcome struct {
	PersonalityName string   `json:"personality_name"`
	ExtensionNames  []string `json:"extension_names,omitempty"` // Optional extensions to use
	ShouldShow      bool     `json:"should_show"`
	Confidence      float64  `json:"confidence"`
	ReasonKeywords  []string `json:"reason_keywords"` // Keywords that should appear in reasoning
	ExpectedState   string   `json:"expected_state"`  // "visible" or "hidden"
	Priority        int      `json:"priority"`        // How important this expectation is (1-3)
	Rationale       string   `json:"rationale"`       // Why this personality should react this way
}

// ExpectedThreadEvaluation contains expected evaluation results.
type ExpectedThreadEvaluation struct {
	ShouldShow     bool     `json:"should_show"`
	Confidence     float64  `json:"confidence"`
	ReasonKeywords []string `json:"reason_keywords"` // Keywords that should appear in reasoning
	ExpectedState  string   `json:"expected_state"`  // "visible" or "hidden"
	Priority       int      `json:"priority"`        // How important this expectation is (1-3)
}

// ThreadTestScenario represents a test case for thread evaluation.
type ThreadTestScenario struct {
	Name                    string                       `json:"name"`
	Description             string                       `json:"description"`
	Thread                  *model.Thread                `json:"thread"`
	ThreadData              ThreadData                   `json:"thread_data"`
	Context                 map[string]interface{}       `json:"context"`
	PersonalityExpectations []PersonalityExpectedOutcome `json:"personality_expectations"`
	DefaultExpected         *ExpectedThreadEvaluation    `json:"default_expected,omitempty"`
}

// TestResult represents the result of running a personality test.
type TestResult struct {
	PersonalityName string                   `json:"personality_name"`
	ScenarioName    string                   `json:"scenario_name"`
	Success         bool                     `json:"success"`
	Score           float64                  `json:"score"`
	ActualResult    *ThreadEvaluationResult  `json:"actual_result"`
	ExpectedResult  ExpectedThreadEvaluation `json:"expected_result"`
	MemoriesUsed    []string                 `json:"memories_used"`
	Reasoning       string                   `json:"reasoning"`
	Timestamp       time.Time                `json:"timestamp"`
}

// PersonalityTestResults represents the results of running personality tests.
type PersonalityTestResults struct {
	TestID    string                 `json:"test_id"`
	Timestamp time.Time              `json:"timestamp"`
	Tests     map[string]*TestResult `json:"tests"`
	Summary   TestSummary            `json:"summary"`
	Duration  time.Duration          `json:"duration"`
}

// TestSummary provides aggregate statistics about test results.
type TestSummary struct {
	TotalTests   int     `json:"total_tests"`
	PassedTests  int     `json:"passed_tests"`
	FailedTests  int     `json:"failed_tests"`
	AverageScore float64 `json:"average_score"`
	HighestScore float64 `json:"highest_score"`
	LowestScore  float64 `json:"lowest_score"`
}

// ThreadEvaluationResult represents the result of evaluating a thread.
type ThreadEvaluationResult struct {
	ShouldShow bool    `json:"should_show"`
	Reason     string  `json:"reason"`
	Confidence float64 `json:"confidence"`
	NewState   string  `json:"new_state"`
}

// TestEnvironment represents the test environment for a personality.
type TestEnvironment struct {
	PersonalityName string
	Memory          interface{} // Using interface{} to avoid import cycles
	ThreadProcessor interface{} // Using interface{} to avoid import cycles
	Repository      interface{} // Using interface{} to avoid import cycles
	MemoryTracker   *MemoryTracker
	Context         context.Context
}

// MemoryTracker tracks memory access during tests.
type MemoryTracker struct {
	accessedMemories []string
}
