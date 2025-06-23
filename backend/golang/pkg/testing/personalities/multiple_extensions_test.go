package personalities

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMultipleExtensionsCombination tests the ability to combine multiple personality extensions.
func TestMultipleExtensionsCombination(t *testing.T) {
	// Setup test framework
	logger := log.New(os.Stdout)
	logger.SetLevel(log.ErrorLevel) // Reduce noise in tests

	// Use nil AI service for mock testing (like in existing tests)
	framework := NewPersonalityTestFramework(logger, nil, "testdata")

	// Load base personalities and extensions
	err := framework.LoadBasePersonalities()
	require.NoError(t, err)

	err = framework.LoadPersonalityExtensions()
	require.NoError(t, err)

	t.Run("CreateExtendedPersonalityWithMultipleExtensions", func(t *testing.T) {
		// Test creating a personality with multiple extensions
		extended, err := framework.CreateExtendedPersonality(
			"tech_entrepreneur",
			"ai_research_focused",
			"startup_ecosystem_focused",
		)
		require.NoError(t, err)
		assert.NotNil(t, extended)

		// Should have 2 extensions
		assert.Len(t, extended.Extensions, 2)

		// Convert to reference personality to test merging
		refPersonality := extended.ToReferencePersonality()
		assert.NotNil(t, refPersonality)

		// Name should reflect both extensions
		expectedName := "tech_entrepreneur_ai_research_focused_startup_ecosystem_focused"
		assert.Equal(t, expectedName, refPersonality.Name)

		// Should have memory facts from both extensions
		hasAIResearchFact := false
		hasStartupEcosystemFact := false

		for _, fact := range refPersonality.MemoryFacts {
			if fact.ID == "memory_006" { // from ai_research_focused
				hasAIResearchFact = true
			}
			if fact.ID == "memory_007" || fact.ID == "memory_008" { // from startup_ecosystem_focused
				hasStartupEcosystemFact = true
			}
		}

		assert.True(t, hasAIResearchFact, "Should include AI research extension memory facts")
		assert.True(t, hasStartupEcosystemFact, "Should include startup ecosystem extension memory facts")

		// Should have plans from extensions
		hasStartupPlan := false
		for _, plan := range refPersonality.Plans {
			if plan.Title == "Q3 Startup Investment Review" {
				hasStartupPlan = true
			}
		}
		assert.True(t, hasStartupPlan, "Should include plans from startup ecosystem extension")
	})

	t.Run("TestMultipleExtensionsScenarioMatching", func(t *testing.T) {
		// Load the new scenario that tests multiple extensions
		err := framework.LoadScenarios()
		require.NoError(t, err)

		scenarios := framework.GetScenarios()

		// Find our test scenario
		var testScenario *ThreadTestScenario
		for _, scenario := range scenarios {
			if scenario.Name == "ai_startup_funding_announcement" {
				testScenario = &scenario
				break
			}
		}

		require.NotNil(t, testScenario, "Should find the ai_startup_funding_announcement scenario")

		// Test that we can find expectations for combined extensions
		combinedExpectation := testScenario.GetExpectedOutcomeForPersonality(
			"tech_entrepreneur",
			[]string{"ai_research_focused", "startup_ecosystem_focused"},
		)

		require.NotNil(t, combinedExpectation, "Should find expectation for combined extensions")
		assert.Equal(t, 0.98, combinedExpectation.Confidence)
		assert.Contains(t, combinedExpectation.ReasonKeywords, "AI research")
		assert.Contains(t, combinedExpectation.ReasonKeywords, "funding")
		assert.Contains(t, combinedExpectation.ReasonKeywords, "valuation")
	})

	t.Run("TestExtensionCombinationVariant", func(t *testing.T) {
		// Test creating a programmatic variant with multiple modifications
		variant, err := framework.CreatePersonalityVariant(
			"tech_entrepreneur",
			"multi_focus_investor",
			func(ext *PersonalityExtension) *PersonalityExtension {
				// Add facts that combine both interests
				ext.AdditionalFacts = append(ext.AdditionalFacts, MemoryFact{
					ID:         "memory_009",
					Content:    "Led $500K seed round for AI research startup developing Constitutional AI for enterprise security",
					Category:   "investment_research",
					Importance: 0.95,
					CreatedAt:  time.Now(),
					Tags:       []string{"investment", "AI_research", "Constitutional_AI", "enterprise"},
					Metadata: map[string]interface{}{
						"investment_amount": "500K",
						"stage":             "seed",
						"focus_area":        "AI_security",
					},
				})

				ext.AdditionalPlans = append(ext.AdditionalPlans, PersonalityPlan{
					Category:    "long_term",
					Title:       "AI Research Investment Fund",
					Description: "Establish $10M fund focused on early-stage AI research commercialization",
					Timeline:    "Next 6 months",
					Priority:    3,
					Status:      "planning",
					CreatedAt:   time.Now(),
				})

				return ext
			},
		)

		require.NoError(t, err)
		assert.NotNil(t, variant)

		refPersonality := variant.ToReferencePersonality()
		assert.Contains(t, refPersonality.Name, "multi_focus_investor")

		// Should have the combined fact
		hasCombinedFact := false
		for _, fact := range refPersonality.MemoryFacts {
			if fact.ID == "memory_009" {
				hasCombinedFact = true
				assert.Contains(t, fact.Content, "Constitutional AI")
				assert.Contains(t, fact.Content, "seed round")
			}
		}
		assert.True(t, hasCombinedFact, "Should include the combined investment/research fact")
	})
}

// TestExtensionProcessingInRunPersonalityTests tests that the test runner properly handles combined extensions.
func TestExtensionProcessingInRunPersonalityTests(t *testing.T) {
	logger := log.New(os.Stdout)
	logger.SetLevel(log.ErrorLevel)

	framework := NewPersonalityTestFramework(logger, nil, "testdata")

	// Load test data
	err := framework.LoadBasePersonalities()
	require.NoError(t, err)

	err = framework.LoadPersonalityExtensions()
	require.NoError(t, err)

	err = framework.LoadScenarios()
	require.NoError(t, err)

	// Create mock storage and repository
	mockStorage := NewMockMemoryStorage()
	mockRepo := NewMockHolonRepository()

	// Run personality tests - this should process the combined extensions
	ctx := context.Background()
	results, err := framework.RunPersonalityTests(ctx, mockStorage, mockRepo)
	require.NoError(t, err)

	// Check that we have results for combined extensions
	hasCombinedExtensionResult := false
	for _, result := range results {
		if result.PersonalityName == "tech_entrepreneur_ai_research_focused_startup_ecosystem_focused" {
			hasCombinedExtensionResult = true
			assert.True(t, result.Success, "Combined extension test should succeed")
			assert.Greater(t, result.Score, 0.7, "Combined extension should have high score")
		}
	}

	assert.True(t, hasCombinedExtensionResult, "Should have test result for combined extensions")

	// Verify that we get different results for different extension combinations
	baseResult := findResult(results, "tech_entrepreneur", "")
	aiOnlyResult := findResult(results, "tech_entrepreneur", "ai_research_focused")
	startupOnlyResult := findResult(results, "tech_entrepreneur", "startup_ecosystem_focused")
	combinedResult := findResult(results, "tech_entrepreneur", "ai_research_focused_startup_ecosystem_focused")

	// All should be found for the ai_startup_funding_announcement scenario
	assert.NotNil(t, baseResult, "Should have base personality result")
	assert.NotNil(t, aiOnlyResult, "Should have AI research only result")
	assert.NotNil(t, startupOnlyResult, "Should have startup ecosystem only result")
	assert.NotNil(t, combinedResult, "Should have combined extensions result")

	// Combined result should have highest confidence/score
	if combinedResult != nil {
		assert.Greater(t, combinedResult.Score, 0.9, "Combined extensions should have highest score")
	}
}

// Helper function to find a specific test result.
func findResult(results []TestResult, personalityName, extensionName string) *TestResult {
	targetName := personalityName
	if extensionName != "" {
		targetName = personalityName + "_" + extensionName
	}

	for _, result := range results {
		if result.PersonalityName == targetName && result.ScenarioName == "ai_startup_funding_announcement" {
			return &result
		}
	}
	return nil
}
