//go:build test
// +build test

package personalities

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

func TestModularPersonalitySystem(t *testing.T) {
	logger := log.New(os.Stdout)
	logger.SetLevel(log.InfoLevel)

	// Create a mock AI service
	mockAIService := &ai.Service{}
	framework := NewPersonalityTestFramework(logger, mockAIService, "testdata")

	t.Run("LoadBasePersonalities", func(t *testing.T) {
		// Load base personalities (without expected behaviors)
		err := framework.LoadBasePersonalities()
		require.NoError(t, err, "Failed to load base personalities")

		// Verify base personalities are loaded
		assert.Contains(t, framework.basePersonalities, "creative_artist")
		assert.Contains(t, framework.basePersonalities, "tech_entrepreneur")

		// Verify they don't contain expected behaviors (in base form)
		creativeArtist := framework.basePersonalities["creative_artist"]
		assert.NotNil(t, creativeArtist)
		assert.Equal(t, "creative_artist", creativeArtist.Name)
		assert.Contains(t, creativeArtist.Profile.CoreTraits, "creative")
		assert.Len(t, creativeArtist.MemoryFacts, 2)
	})

	t.Run("LoadPersonalityExtensions", func(t *testing.T) {
		// Load personality extensions
		err := framework.LoadPersonalityExtensions()
		require.NoError(t, err, "Failed to load personality extensions")

		// Verify extensions are loaded
		logger.Info("Extensions loaded", "count", len(framework.extensions))
	})

	t.Run("CreatePersonalityVariant", func(t *testing.T) {
		// Load base personalities first
		err := framework.LoadBasePersonalities()
		require.NoError(t, err)

		// Create a programmatic personality variant
		variant, err := framework.CreatePersonalityVariant("tech_entrepreneur", "funding_focused", func(ext *PersonalityExtension) *PersonalityExtension {
			// Add funding-specific modifications
			ext.Description = "Variant focused on funding and investment scenarios"

			// Add funding-specific memory fact
			ext.AdditionalFacts = append(ext.AdditionalFacts, MemoryFact{
				ID:         "memory_funding_001",
				Content:    "Currently in discussions with tier-1 VCs for Series C round",
				Category:   "funding_activity",
				Importance: 0.95,
				CreatedAt:  time.Now(),
				Tags:       []string{"series_c", "tier1_vc", "current"},
				Metadata:   map[string]interface{}{"round": "Series C", "stage": "negotiations"},
			})

			// Add funding-specific expected behavior
			ext.ExpectedBehaviors = append(ext.ExpectedBehaviors, ExpectedBehavior{
				ScenarioType: "startup_funding_news",
				Input:        "Major startup funding announcement",
				Expected: map[string]interface{}{
					"interest_level":  "very_high",
					"likely_response": "strategic_analysis",
					"key_concerns":    []string{"market_validation", "valuation_trends", "investor_sentiment"},
				},
				Confidence: 0.91,
			})

			// Add profile override to emphasize funding focus
			ext.ProfileOverrides = &PersonalityProfile{
				Interests: []string{"venture_capital", "startup_valuations", "market_trends"},
			}

			return ext
		})
		require.NoError(t, err, "Failed to create personality variant")

		// Convert to reference personality for testing
		refPersonality := variant.ToReferencePersonality()

		// Verify the variant has been created correctly
		assert.Equal(t, "tech_entrepreneur_funding_focused", refPersonality.Name)
		assert.Contains(t, refPersonality.Description, "funding and investment scenarios")

		// Verify additional memory fact
		hasFundingFact := false
		for _, fact := range refPersonality.MemoryFacts {
			if fact.ID == "memory_funding_001" {
				hasFundingFact = true
				assert.Contains(t, fact.Content, "Series C")
				break
			}
		}
		assert.True(t, hasFundingFact, "Should contain funding-specific memory fact")

		// Verify expected behavior
		hasFundingBehavior := false
		for _, behavior := range refPersonality.ExpectedBehaviors {
			if behavior.ScenarioType == "startup_funding_news" {
				hasFundingBehavior = true
				assert.Equal(t, 0.91, behavior.Confidence)
				break
			}
		}
		assert.True(t, hasFundingBehavior, "Should contain funding-specific expected behavior")

		// Verify profile override
		assert.Contains(t, refPersonality.Profile.Interests, "venture_capital")
	})

	t.Run("BackwardCompatibility", func(t *testing.T) {
		// Load legacy personalities (should still work)
		err := framework.LoadPersonalities()
		require.NoError(t, err, "Failed to load legacy personalities")

		// Verify legacy loading still works
		personalities := framework.GetPersonalities()
		assert.Greater(t, len(personalities), 0, "Should load legacy personalities")
	})
}

func TestExtensionMerging(t *testing.T) {
	// Test profile merging functionality
	baseProfile := PersonalityProfile{
		Age:                28,
		Occupation:         "Artist",
		Interests:          []string{"art", "creativity"},
		CoreTraits:         []string{"creative", "intuitive"},
		CommunicationStyle: "expressive",
		Location:           "Brooklyn",
		Background:         "Fine arts graduate",
	}

	override := PersonalityProfile{
		Age:        30,                           // Override age
		Interests:  []string{"technology", "AI"}, // Additional interests
		CoreTraits: []string{"analytical"},       // Additional traits
		Location:   "San Francisco",              // Override location
	}

	merged := mergeProfiles(baseProfile, override)

	// Verify overrides took effect
	assert.Equal(t, 30, merged.Age, "Age should be overridden")
	assert.Equal(t, "San Francisco", merged.Location, "Location should be overridden")

	// Verify additions took effect
	assert.Contains(t, merged.Interests, "art", "Should keep original interests")
	assert.Contains(t, merged.Interests, "technology", "Should add new interests")
	assert.Contains(t, merged.CoreTraits, "creative", "Should keep original traits")
	assert.Contains(t, merged.CoreTraits, "analytical", "Should add new traits")

	// Verify non-overridden fields remain
	assert.Equal(t, "Artist", merged.Occupation, "Occupation should remain unchanged")
	assert.Equal(t, "expressive", merged.CommunicationStyle, "Communication style should remain unchanged")
}

func TestPersonalitySpecificExpectations(t *testing.T) {
	logger := log.New(os.Stdout)
	logger.SetLevel(log.InfoLevel)

	// Create framework with nil AI service for mock testing
	framework := NewPersonalityTestFramework(logger, nil, "testdata")

	t.Run("LoadScenariosWithPersonalityExpectations", func(t *testing.T) {
		// Load scenarios that have personality-specific expectations
		err := framework.LoadScenarios()
		require.NoError(t, err, "Failed to load scenarios")

		scenarios := framework.GetScenarios()
		assert.Greater(t, len(scenarios), 0, "Should load scenarios")

		// Verify each scenario has personality-specific expectations
		for _, scenario := range scenarios {
			assert.Greater(t, len(scenario.PersonalityExpectations), 0,
				"Scenario %s should have personality expectations", scenario.Name)

			// Verify expectations have proper structure
			for _, expectation := range scenario.PersonalityExpectations {
				assert.NotEmpty(t, expectation.PersonalityName, "Expectation should have personality name")
				assert.NotEmpty(t, expectation.Rationale, "Expectation should have rationale")
				assert.NotEmpty(t, expectation.ExpectedState, "Expectation should have expected state")
				assert.Greater(t, expectation.Priority, 0, "Expectation should have priority")
			}
		}
	})

	t.Run("GetExpectedOutcomeForPersonality", func(t *testing.T) {
		// Load scenarios first
		err := framework.LoadScenarios()
		require.NoError(t, err)

		scenarios := framework.GetScenarios()
		require.Greater(t, len(scenarios), 0)

		// Test getting specific personality expectations for scenarios that actually have them
		var aiStartupScenario *ThreadTestScenario
		for _, scenario := range scenarios {
			if scenario.Name == "ai_startup_funding_announcement" {
				aiStartupScenario = &scenario
				break
			}
		}

		if aiStartupScenario != nil {
			// Test base personality expectation
			techExpectation := aiStartupScenario.GetExpectedOutcomeForPersonality("tech_entrepreneur", []string{})
			require.NotNil(t, techExpectation, "Should find base tech_entrepreneur expectation")
			assert.True(t, techExpectation.ShouldShow)
			assert.Equal(t, 0, len(techExpectation.ExtensionNames))

			// Test personality with extension expectation
			techExtensionExpectation := aiStartupScenario.GetExpectedOutcomeForPersonality("tech_entrepreneur", []string{"ai_research_focused"})
			require.NotNil(t, techExtensionExpectation, "Should find ai_research_focused extension expectation")
			assert.True(t, techExtensionExpectation.ShouldShow)
			assert.Contains(t, techExtensionExpectation.ExtensionNames, "ai_research_focused")

			// Test combined extension expectation
			techCombinedExpectation := aiStartupScenario.GetExpectedOutcomeForPersonality("tech_entrepreneur", []string{"ai_research_focused", "startup_ecosystem_focused"})
			require.NotNil(t, techCombinedExpectation, "Should find combined extension expectation")
			assert.True(t, techCombinedExpectation.ShouldShow)
			assert.Contains(t, techCombinedExpectation.ExtensionNames, "ai_research_focused")
			assert.Contains(t, techCombinedExpectation.ExtensionNames, "startup_ecosystem_focused")

			// Test other personality expectation
			artistExpectation := aiStartupScenario.GetExpectedOutcomeForPersonality("creative_artist", []string{})
			require.NotNil(t, artistExpectation, "Should find creative_artist expectation")
			assert.True(t, artistExpectation.ShouldShow)
		}

		// Test that we can handle scenarios without extensions gracefully
		for _, scenario := range scenarios {
			if scenario.Name != "ai_startup_funding_announcement" {
				// These scenarios might not have extension-specific expectations
				techExpectation := scenario.GetExpectedOutcomeForPersonality("tech_entrepreneur", []string{})
				if techExpectation != nil {
					// If we find an expectation, it should be valid
					assert.NotEmpty(t, techExpectation.PersonalityName)
					assert.NotEmpty(t, techExpectation.ExpectedState)
				}
			}
		}
	})

	t.Run("PersonalityExpectationDifferences", func(t *testing.T) {
		// Load scenarios first
		err := framework.LoadScenarios()
		require.NoError(t, err)

		scenarios := framework.GetScenarios()
		require.Greater(t, len(scenarios), 0)

		// Find a scenario that has different expectations for different personalities
		var testScenario *ThreadTestScenario
		for _, scenario := range scenarios {
			if len(scenario.PersonalityExpectations) >= 2 {
				testScenario = &scenario
				break
			}
		}

		require.NotNil(t, testScenario, "Should have a scenario with multiple personality expectations")

		// Verify that different personalities have different expectations
		expectationMap := make(map[string]*PersonalityExpectedOutcome)
		for _, expectation := range testScenario.PersonalityExpectations {
			key := expectation.PersonalityName
			if len(expectation.ExtensionNames) > 0 {
				key = fmt.Sprintf("%s_%s", expectation.PersonalityName, strings.Join(expectation.ExtensionNames, "_"))
			}
			expectationMap[key] = &expectation
		}

		// Check that expectations differ meaningfully
		var expectations []*PersonalityExpectedOutcome
		for _, exp := range expectationMap {
			expectations = append(expectations, exp)
		}

		if len(expectations) >= 2 {
			exp1, exp2 := expectations[0], expectations[1]
			// At least one aspect should be different
			different := exp1.ShouldShow != exp2.ShouldShow ||
				exp1.Confidence != exp2.Confidence ||
				exp1.Priority != exp2.Priority ||
				exp1.ExpectedState != exp2.ExpectedState

			assert.True(t, different,
				"Different personalities should have different expectations for scenario %s",
				testScenario.Name)
		}
	})

	t.Run("RunPersonalitySpecificTests", func(t *testing.T) {
		// Load base personalities, extensions, and scenarios
		err := framework.LoadBasePersonalities()
		require.NoError(t, err)

		err = framework.LoadPersonalityExtensions()
		require.NoError(t, err)

		err = framework.LoadScenarios()
		require.NoError(t, err)

		// Create mock storage and repository
		mockStorage := NewMockMemoryStorage()
		mockRepo := NewMockHolonRepository()

		// Run personality-specific tests
		ctx := context.Background()
		results, err := framework.RunPersonalityTests(ctx, mockStorage, mockRepo)
		require.NoError(t, err, "Failed to run personality tests")

		// Verify we got results for different personality-scenario combinations
		assert.Greater(t, len(results), 0, "Should have test results")

		// Verify results have personality-specific information
		personalityResults := make(map[string]int)
		scenarioResults := make(map[string]int)

		for _, result := range results {
			assert.NotEmpty(t, result.PersonalityName, "Result should have personality name")
			assert.NotEmpty(t, result.ScenarioName, "Result should have scenario name")

			personalityResults[result.PersonalityName]++
			scenarioResults[result.ScenarioName]++
		}

		// Should have results for multiple personalities and scenarios
		assert.Greater(t, len(personalityResults), 1, "Should have results for multiple personalities")
		assert.Greater(t, len(scenarioResults), 1, "Should have results for multiple scenarios")

		logger.Info("Personality-specific test results",
			"total_results", len(results),
			"personalities_tested", len(personalityResults),
			"scenarios_tested", len(scenarioResults))
	})
}
