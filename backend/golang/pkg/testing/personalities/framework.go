//go:build test
// +build test

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
	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/graph/model"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

// PersonalityTestFramework provides comprehensive testing capabilities for personality models.
type PersonalityTestFramework struct {
	logger            *log.Logger
	aiService         *ai.Service // Can be nil for mock testing
	testDataPath      string
	personalities     map[string]*ReferencePersonality
	basePersonalities map[string]*BasePersonality
	extensions        map[string]map[string]*PersonalityExtension // personality -> extension name -> extension
	scenarios         []ThreadTestScenario
	genericScenarios  []GenericTestScenario
	useMockEvaluation bool // New field to control evaluation mode
}

// NewPersonalityTestFramework creates a new personality test framework.
func NewPersonalityTestFramework(logger *log.Logger, aiService *ai.Service, testDataPath string) *PersonalityTestFramework {
	return &PersonalityTestFramework{
		logger:            logger,
		aiService:         aiService,
		testDataPath:      testDataPath,
		personalities:     make(map[string]*ReferencePersonality),
		basePersonalities: make(map[string]*BasePersonality),
		extensions:        make(map[string]map[string]*PersonalityExtension),
		scenarios:         make([]ThreadTestScenario, 0),
		useMockEvaluation: aiService == nil, // Use mock mode if no AI service provided
	}
}

// SetEvaluationMode sets whether to use mock evaluation or real evaluation.
func (ptf *PersonalityTestFramework) SetEvaluationMode(useMock bool) {
	ptf.useMockEvaluation = useMock
}

// IsUsingMockEvaluation returns whether the framework is using mock evaluation.
func (ptf *PersonalityTestFramework) IsUsingMockEvaluation() bool {
	return ptf.useMockEvaluation
}

// LoadPersonalities loads all personality profiles from the test data directory.
func (ptf *PersonalityTestFramework) LoadPersonalities() error {
	personalitiesDir := filepath.Join(ptf.testDataPath, "personalities")

	entries, err := os.ReadDir(personalitiesDir)
	if err != nil {
		return fmt.Errorf("failed to read personalities directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			personalityFile := filepath.Join(personalitiesDir, entry.Name(), "personality.json")
			if _, err := os.Stat(personalityFile); err == nil {
				personality, err := ptf.loadPersonalityFromFile(personalityFile)
				if err != nil {
					ptf.logger.Warn("Failed to load personality", "file", personalityFile, "error", err)
					continue
				}
				ptf.personalities[personality.Name] = personality
				ptf.logger.Info("Loaded personality", "name", personality.Name)
			}
		}
	}

	return nil
}

// LoadBasePersonalities loads base personality profiles.
func (ptf *PersonalityTestFramework) LoadBasePersonalities() error {
	personalitiesDir := filepath.Join(ptf.testDataPath, "personalities")

	entries, err := os.ReadDir(personalitiesDir)
	if err != nil {
		return fmt.Errorf("failed to read personalities directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// Look for base.json file instead of personality.json
			baseFile := filepath.Join(personalitiesDir, entry.Name(), "base.json")
			if _, err := os.Stat(baseFile); err == nil {
				basePersonality, err := ptf.loadBasePersonalityFromFile(baseFile)
				if err != nil {
					ptf.logger.Warn("Failed to load base personality", "file", baseFile, "error", err)
					continue
				}
				ptf.basePersonalities[basePersonality.Name] = basePersonality
				ptf.logger.Info("Loaded base personality", "name", basePersonality.Name)
			}
		}
	}

	return nil
}

// LoadPersonalityExtensions loads personality extensions.
func (ptf *PersonalityTestFramework) LoadPersonalityExtensions() error {
	extensionsDir := filepath.Join(ptf.testDataPath, "extensions")

	if _, err := os.Stat(extensionsDir); os.IsNotExist(err) {
		ptf.logger.Info("No extensions directory found, skipping extension loading")
		return nil
	}

	entries, err := os.ReadDir(extensionsDir)
	if err != nil {
		return fmt.Errorf("failed to read extensions directory: %w", err)
	}

	// Check if extensions are organized by personality (subdirectories) or flat (JSON files)
	hasSubdirectories := false
	for _, entry := range entries {
		if entry.IsDir() {
			hasSubdirectories = true
			break
		}
	}

	if hasSubdirectories {
		// Handle personality-organized structure: extensions/{personality}/{extension}.json
		for _, entry := range entries {
			if entry.IsDir() {
				personalityName := entry.Name()
				personalityExtDir := filepath.Join(extensionsDir, personalityName)

				extEntries, err := os.ReadDir(personalityExtDir)
				if err != nil {
					ptf.logger.Warn("Failed to read personality extension dir", "dir", personalityExtDir, "error", err)
					continue
				}

				if ptf.extensions[personalityName] == nil {
					ptf.extensions[personalityName] = make(map[string]*PersonalityExtension)
				}

				for _, extEntry := range extEntries {
					if strings.HasSuffix(extEntry.Name(), ".json") {
						extFile := filepath.Join(personalityExtDir, extEntry.Name())
						extension, err := ptf.loadExtensionFromFile(extFile)
						if err != nil {
							ptf.logger.Warn("Failed to load extension", "file", extFile, "error", err)
							continue
						}

						extensionName := strings.TrimSuffix(extEntry.Name(), ".json")
						ptf.extensions[personalityName][extensionName] = extension
						ptf.logger.Info("Loaded extension", "personality", personalityName, "extension", extensionName)
					}
				}
			}
		}
	} else {
		// Handle flat structure: extensions/{extension}.json
		// Apply these extensions to all loaded personalities
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".json") {
				extFile := filepath.Join(extensionsDir, entry.Name())
				extension, err := ptf.loadExtensionFromFile(extFile)
				if err != nil {
					ptf.logger.Warn("Failed to load extension", "file", extFile, "error", err)
					continue
				}

				extensionName := strings.TrimSuffix(entry.Name(), ".json")

				// Apply this extension to all personalities
				for personalityName := range ptf.basePersonalities {
					if ptf.extensions[personalityName] == nil {
						ptf.extensions[personalityName] = make(map[string]*PersonalityExtension)
					}
					ptf.extensions[personalityName][extensionName] = extension
					ptf.logger.Info("Loaded extension", "personality", personalityName, "extension", extensionName)
				}
			}
		}
	}

	return nil
}

// LoadScenarios loads test scenarios from the test data directory.
func (ptf *PersonalityTestFramework) LoadScenarios() error {
	scenariosDir := filepath.Join(ptf.testDataPath, "scenarios")

	// Try to load from JSON files first
	entries, err := os.ReadDir(scenariosDir)
	if err != nil {
		// If no scenarios directory exists, generate scenarios from code
		ptf.logger.Info("No scenarios directory found, generating scenarios from code")
		return ptf.generateCodeBasedScenarios()
	}

	hasJsonFiles := false
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			scenarioFile := filepath.Join(scenariosDir, entry.Name())
			scenario, err := ptf.loadScenarioFromFile(scenarioFile)
			if err != nil {
				ptf.logger.Warn("Failed to load scenario", "file", scenarioFile, "error", err)
				continue
			}

			// Create Thread from ThreadData if it's not already set
			if scenario.Thread == nil && (scenario.ThreadData.Title != "" || scenario.ThreadData.Content != "") {
				scenario.Thread = ptf.createThreadFromData(scenario.ThreadData)
			}

			ptf.scenarios = append(ptf.scenarios, *scenario)
			ptf.logger.Info("Loaded scenario", "name", scenario.Name)
			hasJsonFiles = true
		}
	}

	// If no JSON files found, generate from code
	if !hasJsonFiles {
		ptf.logger.Info("No JSON scenario files found, generating scenarios from code")
		return ptf.generateCodeBasedScenarios()
	}

	return nil
}

// LoadGenericScenarios loads generic test scenarios from the test data directory.
func (ptf *PersonalityTestFramework) LoadGenericScenarios() error {
	scenariosDir := filepath.Join(ptf.testDataPath, "generic_scenarios")

	if _, err := os.Stat(scenariosDir); os.IsNotExist(err) {
		ptf.logger.Info("No generic scenarios directory found, skipping generic scenario loading")
		return nil
	}

	entries, err := os.ReadDir(scenariosDir)
	if err != nil {
		return fmt.Errorf("failed to read generic scenarios directory: %w", err)
	}

	// Create evaluation handler registry
	registry := NewEvaluationHandlerRegistry()
	registry.Register(NewThreadEvaluationHandler(ptf))
	registry.Register(NewChatMessageEvaluationHandler(ptf))
	registry.Register(NewEmailEvaluationHandler(ptf))
	registry.Register(NewSocialPostEvaluationHandler(ptf))

	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			scenarioFile := filepath.Join(scenariosDir, entry.Name())
			scenario, err := ptf.loadGenericScenarioFromFile(scenarioFile)
			if err != nil {
				ptf.logger.Warn("Failed to load generic scenario", "file", scenarioFile, "error", err)
				continue
			}

			// Assign the appropriate evaluation handler based on scenario type
			if handler, exists := registry.GetHandler(scenario.Type); exists {
				scenario.EvaluationHandler = handler
				ptf.logger.Debug("Assigned evaluation handler", "scenario", scenario.Name, "type", scenario.Type)
			} else {
				ptf.logger.Warn("No evaluation handler found for scenario type", "scenario", scenario.Name, "type", scenario.Type)
				continue
			}

			ptf.genericScenarios = append(ptf.genericScenarios, *scenario)
			ptf.logger.Info("Loaded generic scenario", "name", scenario.Name, "type", scenario.Type)
		}
	}

	return nil
}

// GetPersonalities returns the loaded personalities.
func (ptf *PersonalityTestFramework) GetPersonalities() map[string]*ReferencePersonality {
	return ptf.personalities
}

// GetScenarios returns the loaded scenarios.
func (ptf *PersonalityTestFramework) GetScenarios() []ThreadTestScenario {
	return ptf.scenarios
}

// GetGenericScenarios returns the loaded generic scenarios.
func (ptf *PersonalityTestFramework) GetGenericScenarios() []GenericTestScenario {
	return ptf.genericScenarios
}

// RunPersonalityTests runs a comprehensive suite of personality tests with personality-specific expectations.
func (ptf *PersonalityTestFramework) RunPersonalityTests(ctx context.Context, memoryStorage interface{}, holonRepo interface{}) ([]TestResult, error) {
	var results []TestResult

	ptf.logger.Debug("Starting RunPersonalityTests")

	// Collect all personality names from both maps for compatibility
	allPersonalities := make(map[string]bool)

	// Add from basePersonalities (preferred)
	for name := range ptf.basePersonalities {
		allPersonalities[name] = true
	}

	// Add from personalities (fallback for legacy compatibility)
	for name := range ptf.personalities {
		allPersonalities[name] = true
	}

	ptf.logger.Debug("Collected personalities", "count", len(allPersonalities))

	// Test all personalities
	personalityCount := 0
	for personalityName := range allPersonalities {
		personalityCount++
		ptf.logger.Debug("Processing personality", "name", personalityName, "index", personalityCount, "total", len(allPersonalities))

		scenarioCount := 0
		for _, scenario := range ptf.scenarios {
			scenarioCount++
			ptf.logger.Debug("Processing scenario", "personality", personalityName, "scenario", scenario.Name, "scenario_index", scenarioCount, "total_scenarios", len(ptf.scenarios))

			// Check for context cancellation
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}

			// Test base personality
			baseExpectation := scenario.GetExpectedOutcomeForPersonality(personalityName, []string{})
			if baseExpectation != nil {
				ptf.logger.Debug("Running base personality test", "personality", personalityName, "scenario", scenario.Name)
				result, err := ptf.runTestForPersonalityExtensionCombo(ctx, scenario, personalityName, []string{}, *baseExpectation, memoryStorage, holonRepo)
				if err == nil {
					results = append(results, *result)
					ptf.logger.Debug("Base personality test completed", "personality", personalityName, "scenario", scenario.Name, "score", result.Score)
				} else {
					ptf.logger.Warn("Base personality test failed", "personality", personalityName, "scenario", scenario.Name, "error", err)
				}
			}

			// Only test extensions if they are actually loaded for this personality
			if extensions, exists := ptf.extensions[personalityName]; exists && len(extensions) > 0 {
				ptf.logger.Debug("Testing extensions", "personality", personalityName, "extension_count", len(extensions))

				// Test single extensions
				extensionIndex := 0
				for extensionName := range extensions {
					extensionIndex++
					ptf.logger.Debug("Testing single extension", "personality", personalityName, "extension", extensionName, "extension_index", extensionIndex)

					// Check for context cancellation
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					default:
					}

					expectation := scenario.GetExpectedOutcomeForPersonality(personalityName, []string{extensionName})
					if expectation != nil {
						result, err := ptf.runTestForPersonalityExtensionCombo(ctx, scenario, personalityName, []string{extensionName}, *expectation, memoryStorage, holonRepo)
						if err == nil {
							results = append(results, *result)
							ptf.logger.Debug("Single extension test completed", "personality", personalityName, "extension", extensionName, "score", result.Score)
						} else {
							ptf.logger.Warn("Single extension test failed", "personality", personalityName, "extension", extensionName, "error", err)
						}
					}
				}

				// Test specific multi-extension combinations that have expectations AND are available
				multiExtensionIndex := 0
				for _, expectation := range scenario.PersonalityExpectations {
					if expectation.PersonalityName == personalityName && len(expectation.ExtensionNames) > 1 {
						multiExtensionIndex++
						ptf.logger.Debug("Testing multi-extension combination", "personality", personalityName, "extensions", expectation.ExtensionNames, "multi_index", multiExtensionIndex)

						// Check for context cancellation
						select {
						case <-ctx.Done():
							return nil, ctx.Err()
						default:
						}

						// Check if all required extensions are actually loaded
						allExtensionsAvailable := true
						for _, extName := range expectation.ExtensionNames {
							if _, extensionExists := extensions[extName]; !extensionExists {
								allExtensionsAvailable = false
								ptf.logger.Debug("Extension not available", "personality", personalityName, "missing_extension", extName)
								break
							}
						}

						// Only run test if all extensions are available
						if allExtensionsAvailable {
							result, err := ptf.runTestForPersonalityExtensionCombo(ctx, scenario, personalityName, expectation.ExtensionNames, expectation, memoryStorage, holonRepo)
							if err == nil {
								results = append(results, *result)
								ptf.logger.Debug("Multi-extension test completed", "personality", personalityName, "extensions", expectation.ExtensionNames, "score", result.Score)
							} else {
								ptf.logger.Warn("Multi-extension test failed", "personality", personalityName, "extensions", expectation.ExtensionNames, "error", err)
							}
						} else {
							ptf.logger.Debug("Skipping multi-extension test - not all extensions available", "personality", personalityName, "extensions", expectation.ExtensionNames)
						}
					}
				}
			} else {
				ptf.logger.Debug("No extensions found for personality", "personality", personalityName)
			}
		}
	}

	ptf.logger.Debug("RunPersonalityTests completed", "total_results", len(results))
	return results, nil
}

// RunFlexiblePersonalityTests runs tests using the flexible generic scenario system.
func (ptf *PersonalityTestFramework) RunFlexiblePersonalityTests(ctx context.Context, memoryStorage interface{}, holonRepo interface{}) ([]TestResult, error) {
	var results []TestResult

	// Test base personalities against generic scenarios
	for personalityName := range ptf.basePersonalities {
		for _, scenario := range ptf.genericScenarios {
			// Test base personality
			baseExpectation := scenario.GetExpectedOutcomeForPersonality(personalityName, []string{})
			if baseExpectation != nil {
				result, err := ptf.runGenericTestForPersonality(ctx, scenario, personalityName, []string{}, *baseExpectation, memoryStorage, holonRepo)
				if err == nil {
					results = append(results, *result)
				}
			}

			// Test single extensions
			if extensions, exists := ptf.extensions[personalityName]; exists {
				for extensionName := range extensions {
					expectation := scenario.GetExpectedOutcomeForPersonality(personalityName, []string{extensionName})
					if expectation != nil {
						result, err := ptf.runGenericTestForPersonality(ctx, scenario, personalityName, []string{extensionName}, *expectation, memoryStorage, holonRepo)
						if err == nil {
							results = append(results, *result)
						}
					}
				}
			}
		}
	}

	return results, nil
}

// runTestForPersonalityExtensionCombo runs a test for a specific personality-extension combination.
func (ptf *PersonalityTestFramework) runTestForPersonalityExtensionCombo(ctx context.Context, scenario ThreadTestScenario, personalityName string, extensionNames []string, expectedOutcome PersonalityExpectedOutcome, memoryStorage interface{}, holonRepo interface{}) (*TestResult, error) {
	// Create the test personality by combining base + extensions
	testPersonality, err := ptf.createTestPersonality(personalityName, extensionNames)
	if err != nil {
		return nil, fmt.Errorf("failed to create test personality: %w", err)
	}

	// Run the test
	result, err := ptf.runSingleTest(ctx, scenario, testPersonality, expectedOutcome, memoryStorage, holonRepo)
	if err != nil {
		ptf.logger.Warn("Test failed",
			"scenario", scenario.Name,
			"personality", personalityName,
			"extension", extensionNames,
			"error", err)
		return nil, err
	}

	return result, nil
}

// runGenericTestForPersonality runs a generic test for a specific personality-extension combination.
func (ptf *PersonalityTestFramework) runGenericTestForPersonality(ctx context.Context, scenario GenericTestScenario, personalityName string, extensionNames []string, expectedOutcome PersonalityExpectedOutcome, memoryStorage interface{}, holonRepo interface{}) (*TestResult, error) {
	// Create the test personality by combining base + extensions
	testPersonality, err := ptf.createTestPersonality(personalityName, extensionNames)
	if err != nil {
		return nil, fmt.Errorf("failed to create test personality: %w", err)
	}

	// Create test environment
	env, err := ptf.setupTestEnvironment(ctx, testPersonality, memoryStorage, holonRepo)
	if err != nil {
		return nil, fmt.Errorf("failed to setup test environment: %w", err)
	}

	// Run the generic scenario evaluation
	genericResult, err := scenario.Evaluate(ctx, testPersonality, env)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate generic scenario: %w", err)
	}

	// Convert generic result to thread evaluation result for compatibility
	actualResult := &ThreadEvaluationResult{
		ShouldShow: genericResult.ShouldShow,
		Reason:     genericResult.Reason,
		Confidence: genericResult.Confidence,
		NewState:   genericResult.NewState,
	}

	// Calculate score based on how well actual matches expected
	score := ptf.calculateBasicScore(expectedOutcome, actualResult)

	return &TestResult{
		PersonalityName: testPersonality.Name,
		ScenarioName:    scenario.Name,
		Success:         score >= 0.7, // Consider 70% or higher as success
		Score:           score,
		ActualResult:    actualResult,
		ExpectedResult:  expectedOutcome.GetExpectedThreadEvaluation(),
		MemoriesUsed:    env.MemoryTracker.GetAccessedMemories(),
		Reasoning:       actualResult.Reason,
		Timestamp:       time.Now(),
	}, nil
}

// createTestPersonality creates a test personality by combining base personality with extensions.
func (ptf *PersonalityTestFramework) createTestPersonality(personalityName string, extensionNames []string) (*ReferencePersonality, error) {
	// Try to find the personality in basePersonalities first (preferred)
	basePersonality, exists := ptf.basePersonalities[personalityName]
	if exists {
		// Start with base personality
		extendedPersonality := &ExtendedPersonality{
			Base:       basePersonality,
			Extensions: make([]*PersonalityExtension, 0),
			TestID:     generateTestID(),
			CreatedAt:  time.Now(),
		}

		// Add extensions
		if len(extensionNames) > 0 {
			personalityExtensions, extExists := ptf.extensions[personalityName]
			if !extExists {
				return nil, fmt.Errorf("no extensions found for personality: %s", personalityName)
			}

			for _, extensionName := range extensionNames {
				extension, extFound := personalityExtensions[extensionName]
				if !extFound {
					return nil, fmt.Errorf("extension not found: %s for personality %s", extensionName, personalityName)
				}
				extendedPersonality.Extensions = append(extendedPersonality.Extensions, extension)
			}
		}

		return extendedPersonality.ToReferencePersonality(), nil
	}

	// Fallback: look in personalities map (legacy compatibility)
	refPersonality, exists := ptf.personalities[personalityName]
	if exists {
		// For legacy personalities, just return as-is (can't add extensions)
		if len(extensionNames) > 0 {
			ptf.logger.Warn("Cannot add extensions to legacy personality", "personality", personalityName, "extensions", extensionNames)
		}
		return refPersonality, nil
	}

	return nil, fmt.Errorf("personality not found: %s", personalityName)
}

// CreatePersonalityVariant creates a personality variant programmatically.
func (ptf *PersonalityTestFramework) CreatePersonalityVariant(baseName, variantName string, modifications func(*PersonalityExtension) *PersonalityExtension) (*ExtendedPersonality, error) {
	basePersonality, exists := ptf.basePersonalities[baseName]
	if !exists {
		return nil, fmt.Errorf("base personality '%s' not found", baseName)
	}

	// Create a basic extension
	extension := &PersonalityExtension{
		TestName:          variantName,
		Description:       fmt.Sprintf("Programmatic variant of %s", baseName),
		AdditionalFacts:   []MemoryFact{},
		AdditionalPlans:   []PersonalityPlan{},
		ExpectedBehaviors: []ExpectedBehavior{},
		Tags:              []string{"programmatic", "variant"},
	}

	// Apply modifications
	if modifications != nil {
		extension = modifications(extension)
	}

	return &ExtendedPersonality{
		Base:       basePersonality,
		Extensions: []*PersonalityExtension{extension},
		TestID:     generateTestID(),
		CreatedAt:  time.Now(),
	}, nil
}

// CreateExtendedPersonality creates an extended personality with multiple extensions.
func (ptf *PersonalityTestFramework) CreateExtendedPersonality(baseName string, extensionNames ...string) (*ExtendedPersonality, error) {
	basePersonality, exists := ptf.basePersonalities[baseName]
	if !exists {
		return nil, fmt.Errorf("base personality '%s' not found", baseName)
	}

	var extensions []*PersonalityExtension
	for _, extName := range extensionNames {
		if personalityExtensions, exists := ptf.extensions[baseName]; exists {
			if extension, exists := personalityExtensions[extName]; exists {
				extensions = append(extensions, extension)
			} else {
				return nil, fmt.Errorf("extension '%s' not found for personality '%s'", extName, baseName)
			}
		} else {
			return nil, fmt.Errorf("no extensions found for personality '%s'", baseName)
		}
	}

	return &ExtendedPersonality{
		Base:       basePersonality,
		Extensions: extensions,
		TestID:     generateTestID(),
		CreatedAt:  time.Now(),
	}, nil
}

// runSingleTest runs a single test scenario for a personality.
func (ptf *PersonalityTestFramework) runSingleTest(ctx context.Context, scenario ThreadTestScenario, personality *ReferencePersonality, expectedOutcome PersonalityExpectedOutcome, memoryStorage interface{}, holonRepo interface{}) (*TestResult, error) {
	// Create test environment
	env, err := ptf.setupTestEnvironment(ctx, personality, memoryStorage, holonRepo)
	if err != nil {
		return nil, fmt.Errorf("failed to setup test environment: %w", err)
	}

	var actualResult *ThreadEvaluationResult

	if ptf.useMockEvaluation {
		// Mock evaluation mode - use rule-based evaluation for testing framework logic
		actualResult = ptf.performMockEvaluation(scenario, personality, expectedOutcome)
		ptf.logger.Debug("Using mock evaluation",
			"personality", personality.Name,
			"scenario", scenario.Name,
			"result", actualResult.ShouldShow)
	} else {
		// Real evaluation mode - use actual thread processor/AI evaluation
		realResult, err := ptf.performRealEvaluation(ctx, scenario, personality, env)
		if err != nil {
			return nil, fmt.Errorf("failed to perform real evaluation: %w", err)
		}
		actualResult = realResult
		ptf.logger.Debug("Using real evaluation",
			"personality", personality.Name,
			"scenario", scenario.Name,
			"result", actualResult.ShouldShow,
			"confidence", actualResult.Confidence)
	}

	// Calculate score based on how well actual matches expected
	score := ptf.calculateBasicScore(expectedOutcome, actualResult)

	return &TestResult{
		PersonalityName: personality.Name,
		ScenarioName:    scenario.Name,
		Success:         score >= 0.7, // Consider 70% or higher as success
		Score:           score,
		ActualResult:    actualResult,
		ExpectedResult:  expectedOutcome.GetExpectedThreadEvaluation(),
		MemoriesUsed:    env.MemoryTracker.GetAccessedMemories(),
		Reasoning:       actualResult.Reason,
		Timestamp:       time.Now(),
	}, nil
}

// performMockEvaluation provides rule-based evaluation for testing the framework logic.
func (ptf *PersonalityTestFramework) performMockEvaluation(scenario ThreadTestScenario, personality *ReferencePersonality, expectedOutcome PersonalityExpectedOutcome) *ThreadEvaluationResult {
	// Implement rule-based logic that evaluates based on content analysis
	threadContent := scenario.ThreadData.Content
	threadTitle := scenario.ThreadData.Title
	combinedText := strings.ToLower(threadTitle + " " + threadContent)

	shouldShow := true
	confidence := 0.7
	reason := fmt.Sprintf("Rule-based evaluation for personality %s", personality.Name)

	// Extract base personality and extensions from the personality name
	baseName, extensions := ptf.parsePersonalityName(personality.Name)

	// Apply personality-specific rules with extension bonuses
	switch baseName {
	case "tech_entrepreneur":
		shouldShow, confidence, reason = ptf.evaluateTechEntrepreneurInterest(combinedText, scenario)
		// Apply extension bonuses for tech entrepreneur
		confidence = ptf.applyExtensionBonus(confidence, extensions, scenario)

	case "creative_artist":
		shouldShow, confidence, reason = ptf.evaluateCreativeArtistInterest(combinedText, scenario)
		// Apply extension bonuses for creative artist
		confidence = ptf.applyExtensionBonus(confidence, extensions, scenario)

	default:
		// Generic evaluation based on content quality and length
		shouldShow, confidence, reason = ptf.evaluateGenericInterest(combinedText, scenario)
	}

	// Update reason to include extension information
	if len(extensions) > 0 {
		reason = fmt.Sprintf("%s (with %d extensions: %v)", reason, len(extensions), extensions)
	}

	newState := "visible"
	if !shouldShow {
		newState = "hidden"
	}

	return &ThreadEvaluationResult{
		ShouldShow: shouldShow,
		Reason:     reason,
		Confidence: confidence,
		NewState:   newState,
	}
}

// evaluateTechEntrepreneurInterest evaluates content from a tech entrepreneur's perspective.
func (ptf *PersonalityTestFramework) evaluateTechEntrepreneurInterest(text string, scenario ThreadTestScenario) (bool, float64, string) {
	// High interest keywords for tech entrepreneurs
	highInterestKeywords := []string{"ai", "startup", "funding", "technology", "innovation", "venture", "series", "investment", "automation", "machine learning", "artificial intelligence"}

	// Low interest keywords
	lowInterestKeywords := []string{"celebrity", "gossip", "entertainment", "fashion", "sports"}

	// Negative keywords that should be filtered out
	negativeKeywords := []string{"scandal", "drama", "reality tv"}

	highMatches := countKeywordMatches(text, highInterestKeywords)
	lowMatches := countKeywordMatches(text, lowInterestKeywords)
	negativeMatches := countKeywordMatches(text, negativeKeywords)

	// Strong negative signals
	if negativeMatches > 0 {
		return false, 0.9, "Content contains negative keywords that tech entrepreneurs typically avoid"
	}

	// Strong positive signals
	if highMatches >= 2 {
		return true, 0.95, fmt.Sprintf("High relevance content with %d tech-related keywords", highMatches)
	}

	if highMatches == 1 {
		return true, 0.8, "Content contains relevant technology keywords"
	}

	// Low interest content
	if lowMatches > 0 {
		return false, 0.85, "Content appears to be entertainment/celebrity focused, not relevant to tech entrepreneurs"
	}

	// Check domain context if available
	if domain, ok := scenario.Context["domain"].(string); ok {
		switch domain {
		case "artificial_intelligence", "venture_capital", "technical_education":
			return true, 0.9, fmt.Sprintf("Content is in highly relevant domain: %s", domain)
		case "entertainment_gossip":
			return false, 0.9, "Entertainment gossip is not relevant to tech entrepreneurs"
		}
	}

	// Default: moderate interest if content is substantial
	if len(strings.Fields(text)) > 10 {
		return true, 0.6, "Content appears substantial and potentially relevant"
	}

	return false, 0.7, "Content too brief or not clearly relevant to tech entrepreneurship"
}

// evaluateCreativeArtistInterest evaluates content from a creative artist's perspective.
func (ptf *PersonalityTestFramework) evaluateCreativeArtistInterest(text string, scenario ThreadTestScenario) (bool, float64, string) {
	// High interest keywords for creative artists
	highInterestKeywords := []string{"art", "creative", "design", "visual", "artistic", "painting", "illustration", "digital art", "procreate", "adobe", "photoshop", "creativity", "aesthetic"}

	// Moderate interest keywords
	moderateKeywords := []string{"ai", "tool", "technology", "innovation", "platform"}

	// Low interest keywords
	lowInterestKeywords := []string{"funding", "venture", "investment", "series", "startup", "business"}

	// Negative keywords
	negativeKeywords := []string{"celebrity", "gossip", "scandal", "drama"}

	highMatches := countKeywordMatches(text, highInterestKeywords)
	moderateMatches := countKeywordMatches(text, moderateKeywords)
	lowMatches := countKeywordMatches(text, lowInterestKeywords)
	negativeMatches := countKeywordMatches(text, negativeKeywords)

	// Strong negative signals
	if negativeMatches > 0 {
		return false, 0.8, "Content contains topics that creative artists typically avoid"
	}

	// Strong positive signals
	if highMatches >= 2 {
		return true, 0.95, fmt.Sprintf("High relevance content with %d creative-related keywords", highMatches)
	}

	if highMatches == 1 {
		return true, 0.85, "Content contains relevant creative/artistic keywords"
	}

	// Moderate interest in tech if it relates to creative tools
	if moderateMatches > 0 && highMatches > 0 {
		return true, 0.75, "Content about technology tools that could be relevant to creative work"
	}

	// Low interest in pure business content
	if lowMatches > 0 && highMatches == 0 {
		return false, 0.7, "Pure business/funding content is less relevant to creative artists"
	}

	// Check domain context
	if domain, ok := scenario.Context["domain"].(string); ok {
		switch domain {
		case "creative_tools":
			return true, 0.9, "Content is about creative tools, highly relevant"
		case "artificial_intelligence":
			return true, 0.65, "AI content has moderate relevance for creative applications"
		case "entertainment_gossip":
			return false, 0.8, "Entertainment gossip is not relevant to creative work"
		case "venture_capital":
			return false, 0.7, "Venture capital content has limited relevance to creative artists"
		}
	}

	// Default evaluation based on content quality
	if len(strings.Fields(text)) > 10 {
		return true, 0.5, "Content appears substantial, may have some creative relevance"
	}

	return false, 0.6, "Content too brief or not clearly relevant to creative arts"
}

// evaluateGenericInterest provides fallback evaluation for unknown personalities.
func (ptf *PersonalityTestFramework) evaluateGenericInterest(text string, scenario ThreadTestScenario) (bool, float64, string) {
	// Basic content quality evaluation
	wordCount := len(strings.Fields(text))

	// Too short content
	if wordCount < 5 {
		return false, 0.9, "Content too short to be meaningful"
	}

	// Spam indicators
	spamKeywords := []string{"click here", "limited time", "act now", "free money", "guaranteed"}
	spamMatches := countKeywordMatches(text, spamKeywords)

	if spamMatches > 0 {
		return false, 0.95, "Content appears to be spam or promotional"
	}

	// Quality indicators
	qualityKeywords := []string{"analysis", "research", "study", "insight", "comprehensive", "detailed"}
	qualityMatches := countKeywordMatches(text, qualityKeywords)

	if qualityMatches > 0 {
		return true, 0.8, "Content appears to be high-quality and informative"
	}

	// Default: accept substantial content
	if wordCount > 15 {
		return true, 0.6, "Content appears substantial and potentially interesting"
	}

	return true, 0.5, "Content meets basic quality threshold"
}

// performRealEvaluation performs actual thread evaluation using AI services or thread processors.
func (ptf *PersonalityTestFramework) performRealEvaluation(ctx context.Context, scenario ThreadTestScenario, personality *ReferencePersonality, env *TestEnvironment) (*ThreadEvaluationResult, error) {
	// If AI service is available, use it for evaluation
	if ptf.aiService != nil {
		return ptf.performAIEvaluation(ctx, scenario, personality, env)
	}

	// TODO: If thread processor is available, use it
	// This would integrate with the actual thread processing pipeline
	// if env.ThreadProcessor != nil {
	//     return env.ThreadProcessor.EvaluateThread(ctx, scenario.Thread, personality)
	// }

	return nil, fmt.Errorf("no evaluation method available - neither AI service nor thread processor configured")
}

// performAIEvaluation uses the AI service to evaluate thread relevance.
func (ptf *PersonalityTestFramework) performAIEvaluation(ctx context.Context, scenario ThreadTestScenario, personality *ReferencePersonality, env *TestEnvironment) (*ThreadEvaluationResult, error) {
	// Construct prompt for AI evaluation
	prompt := ptf.buildEvaluationPrompt(scenario, personality)

	ptf.logger.Debug("Performing AI evaluation",
		"personality", personality.Name,
		"scenario", scenario.Name,
		"prompt_length", len(prompt))

	openaiMessages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(prompt),
	}
	response, err := ptf.aiService.Completions(ctx, openaiMessages, nil, "gpt-4")
	if err != nil {
		return nil, fmt.Errorf("AI evaluation failed: %w", err)
	}

	// Parse AI response to extract evaluation result
	result, err := ptf.parseAIEvaluationResponse(response.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse AI evaluation response: %w", err)
	}

	ptf.logger.Debug("AI evaluation completed",
		"personality", personality.Name,
		"scenario", scenario.Name,
		"should_show", result.ShouldShow,
		"confidence", result.Confidence)

	return result, nil
}

// buildEvaluationPrompt constructs a prompt for AI-based thread evaluation.
func (ptf *PersonalityTestFramework) buildEvaluationPrompt(scenario ThreadTestScenario, personality *ReferencePersonality) string {
	return fmt.Sprintf(`You are evaluating whether a person with the following personality profile would be interested in a specific thread.

PERSONALITY PROFILE:
Name: %s
Description: %s
Interests: %v
Core Traits: %v

THREAD TO EVALUATE:
Title: %s
Content: %s
Author: %s

Please evaluate whether this person would want to see this thread and respond in the following JSON format:
{
  "should_show": true/false,
  "confidence": 0.0-1.0,
  "reason": "explanation of your reasoning",
  "new_state": "visible" or "hidden"
}

Consider the person's interests, personality traits, and how the content aligns with their likely preferences.`,
		personality.Name,
		personality.Description,
		personality.Profile.Interests,
		personality.Profile.CoreTraits,
		scenario.ThreadData.Title,
		scenario.ThreadData.Content,
		scenario.ThreadData.AuthorName)
}

// parseAIEvaluationResponse parses the AI service response into a ThreadEvaluationResult.
func (ptf *PersonalityTestFramework) parseAIEvaluationResponse(response string) (*ThreadEvaluationResult, error) {
	// Try to extract JSON from the response
	var result struct {
		ShouldShow bool    `json:"should_show"`
		Confidence float64 `json:"confidence"`
		Reason     string  `json:"reason"`
		NewState   string  `json:"new_state"`
	}

	// Find JSON in the response (AI might include additional text)
	jsonStart := strings.Index(response, "{")
	jsonEnd := strings.LastIndex(response, "}") + 1

	if jsonStart == -1 || jsonEnd <= jsonStart {
		return nil, fmt.Errorf("no valid JSON found in AI response")
	}

	jsonStr := response[jsonStart:jsonEnd]

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Validate and set defaults
	if result.Confidence < 0 {
		result.Confidence = 0
	}
	if result.Confidence > 1 {
		result.Confidence = 1
	}

	if result.NewState == "" {
		if result.ShouldShow {
			result.NewState = "visible"
		} else {
			result.NewState = "hidden"
		}
	}

	if result.Reason == "" {
		result.Reason = "AI evaluation completed"
	}

	return &ThreadEvaluationResult{
		ShouldShow: result.ShouldShow,
		Reason:     result.Reason,
		Confidence: result.Confidence,
		NewState:   result.NewState,
	}, nil
}

// countKeywordMatches counts how many keywords from the list appear in the text.
func countKeywordMatches(text string, keywords []string) int {
	count := 0
	text = strings.ToLower(text)

	for _, keyword := range keywords {
		if strings.Contains(text, strings.ToLower(keyword)) {
			count++
		}
	}

	return count
}

// generateCodeBasedScenarios generates scenarios using the code-based scenario system.
func (ptf *PersonalityTestFramework) generateCodeBasedScenarios() error {
	generator := NewScenarioGenerator()

	// Generate standard scenarios
	scenarios, err := generator.GenerateStandardScenarios(ptf)
	if err != nil {
		return fmt.Errorf("failed to generate standard scenarios: %w", err)
	}

	ptf.scenarios = append(ptf.scenarios, scenarios...)

	ptf.logger.Info("Generated scenarios from code", "count", len(scenarios))
	return nil
}

// GenerateReport creates a comprehensive test report.
func (ptf *PersonalityTestFramework) GenerateReport(results []TestResult) PersonalityTestResults {
	totalTests := len(results)
	passedTests := 0
	var totalScore float64
	highestScore := 0.0
	lowestScore := 1.0

	testMap := make(map[string]*TestResult)

	for i, result := range results {
		if result.Success {
			passedTests++
		}

		totalScore += result.Score

		if result.Score > highestScore {
			highestScore = result.Score
		}
		if result.Score < lowestScore {
			lowestScore = result.Score
		}

		key := fmt.Sprintf("%s_%s", result.PersonalityName, result.ScenarioName)
		testMap[key] = &results[i]
	}

	averageScore := 0.0
	if totalTests > 0 {
		averageScore = totalScore / float64(totalTests)
	}

	return PersonalityTestResults{
		TestID:    generateTestID(),
		Timestamp: time.Now(),
		Tests:     testMap,
		Summary: TestSummary{
			TotalTests:   totalTests,
			PassedTests:  passedTests,
			FailedTests:  totalTests - passedTests,
			AverageScore: averageScore,
			HighestScore: highestScore,
			LowestScore:  lowestScore,
		},
		Duration: 0, // Would be calculated in real implementation
	}
}

// PrintSummary prints a summary of test results.
func (ptf *PersonalityTestFramework) PrintSummary(report PersonalityTestResults) {
	ptf.logger.Info("Personality Test Results Summary",
		"test_id", report.TestID,
		"total_tests", report.Summary.TotalTests,
		"passed_tests", report.Summary.PassedTests,
		"failed_tests", report.Summary.FailedTests,
		"average_score", fmt.Sprintf("%.3f", report.Summary.AverageScore),
		"highest_score", fmt.Sprintf("%.3f", report.Summary.HighestScore),
		"lowest_score", fmt.Sprintf("%.3f", report.Summary.LowestScore))
}

// SaveReport saves the test report to a file.
func (ptf *PersonalityTestFramework) SaveReport(report PersonalityTestResults, filename string) error {
	// Ensure the directory exists
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create reports directory: %w", err)
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	if err := os.WriteFile(filename, data, 0o644); err != nil {
		return fmt.Errorf("failed to write report file: %w", err)
	}

	return nil
}

// Helper functions for loading data.
func (ptf *PersonalityTestFramework) loadPersonalityFromFile(filename string) (*ReferencePersonality, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var personality ReferencePersonality
	if err := json.Unmarshal(data, &personality); err != nil {
		return nil, err
	}

	return &personality, nil
}

func (ptf *PersonalityTestFramework) loadBasePersonalityFromFile(filename string) (*BasePersonality, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var personality BasePersonality
	if err := json.Unmarshal(data, &personality); err != nil {
		return nil, err
	}

	return &personality, nil
}

func (ptf *PersonalityTestFramework) loadExtensionFromFile(filename string) (*PersonalityExtension, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var extension PersonalityExtension
	if err := json.Unmarshal(data, &extension); err != nil {
		return nil, err
	}

	return &extension, nil
}

func (ptf *PersonalityTestFramework) loadScenarioFromFile(filename string) (*ThreadTestScenario, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var scenario ThreadTestScenario
	if err := json.Unmarshal(data, &scenario); err != nil {
		return nil, err
	}

	return &scenario, nil
}

// loadGenericScenarioFromFile loads a generic scenario from a JSON file.
func (ptf *PersonalityTestFramework) loadGenericScenarioFromFile(filename string) (*GenericTestScenario, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var scenario GenericTestScenario
	if err := json.Unmarshal(data, &scenario); err != nil {
		return nil, err
	}

	return &scenario, nil
}

// createThreadFromData converts ThreadData to a model.Thread.
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

// generateTestID creates a unique test ID.
func generateTestID() string {
	return fmt.Sprintf("test-%d", time.Now().UnixNano())
}

// setupTestEnvironment creates a test environment with personality data loaded into memory.
func (ptf *PersonalityTestFramework) setupTestEnvironment(ctx context.Context, personality *ReferencePersonality, memoryStorage interface{}, repository interface{}) (*TestEnvironment, error) {
	// Create memory tracker
	tracker := NewMemoryTracker()

	// For mock testing, create a minimal processor
	// This allows us to test the framework logic without requiring a real database
	ptf.logger.Debug("Using mock repository, creating simplified test environment", "personality", personality.Name)

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

// calculateBasicScore provides a basic scoring mechanism when LLM judge is not available.
func (ptf *PersonalityTestFramework) calculateBasicScore(expectedOutcome PersonalityExpectedOutcome, actualResult *ThreadEvaluationResult) float64 {
	score := 0.0

	// Check if ShouldShow matches
	if expectedOutcome.ShouldShow == actualResult.ShouldShow {
		score += 0.4
	}

	// Check if confidence is within reasonable range
	confidenceDiff := expectedOutcome.Confidence - actualResult.Confidence
	if confidenceDiff < 0 {
		confidenceDiff = -confidenceDiff
	}
	if confidenceDiff <= 0.2 {
		score += 0.3
	}

	// Check if state matches
	if expectedOutcome.ExpectedState == actualResult.NewState {
		score += 0.3
	}

	return score
}

// parsePersonalityName extracts the base personality name and extensions from a combined name.
func (ptf *PersonalityTestFramework) parsePersonalityName(fullName string) (string, []string) {
	// Handle names like "tech_entrepreneur_ai_research_focused_startup_ecosystem_focused"

	// Common base personality patterns
	basePatterns := []string{
		"tech_entrepreneur",
		"creative_artist",
	}

	for _, pattern := range basePatterns {
		if strings.HasPrefix(fullName, pattern) {
			// Extract extensions by removing the base pattern
			remainder := strings.TrimPrefix(fullName, pattern)
			remainder = strings.TrimPrefix(remainder, "_")

			if remainder == "" {
				return pattern, []string{} // No extensions
			}

			// Split remaining parts as extensions
			extensionParts := strings.Split(remainder, "_")

			// Group extension parts back together (e.g., "ai_research_focused")
			extensions := ptf.reconstructExtensionNames(extensionParts)
			return pattern, extensions
		}
	}

	// Fallback: treat the whole name as base with no extensions
	return fullName, []string{}
}

// reconstructExtensionNames groups extension parts back into meaningful extension names.
func (ptf *PersonalityTestFramework) reconstructExtensionNames(parts []string) []string {
	if len(parts) == 0 {
		return []string{}
	}

	// Known extension patterns
	knownExtensions := []string{
		"ai_research_focused",
		"startup_ecosystem_focused",
		"creative_tools_focused",
		"ai_creative_applications",
	}

	var extensions []string
	remaining := strings.Join(parts, "_")

	// Try to match known extension patterns
	for _, ext := range knownExtensions {
		if strings.Contains(remaining, ext) {
			extensions = append(extensions, ext)
			remaining = strings.ReplaceAll(remaining, ext, "")
			remaining = strings.Trim(remaining, "_")
		}
	}

	// If we couldn't match known patterns, treat each part as an extension
	if len(extensions) == 0 && len(parts) > 0 {
		// Fallback: group parts in reasonable chunks
		for i := 0; i < len(parts); i += 3 {
			end := i + 3
			if end > len(parts) {
				end = len(parts)
			}
			extensionName := strings.Join(parts[i:end], "_")
			extensions = append(extensions, extensionName)
		}
	}

	return extensions
}

// applyExtensionBonus applies confidence bonuses based on the number and relevance of extensions.
func (ptf *PersonalityTestFramework) applyExtensionBonus(baseConfidence float64, extensions []string, scenario ThreadTestScenario) float64 {
	if len(extensions) == 0 {
		return baseConfidence
	}

	combinedText := strings.ToLower(scenario.ThreadData.Title + " " + scenario.ThreadData.Content)

	// Calculate relevance bonus based on extensions and scenario content
	extensionBonus := 0.0

	for _, extension := range extensions {
		switch extension {
		case "ai_research_focused":
			if strings.Contains(combinedText, "ai") || strings.Contains(combinedText, "artificial intelligence") ||
				strings.Contains(combinedText, "machine learning") || strings.Contains(combinedText, "constitutional ai") {
				extensionBonus += 0.1
			}
		case "startup_ecosystem_focused":
			if strings.Contains(combinedText, "funding") || strings.Contains(combinedText, "startup") ||
				strings.Contains(combinedText, "series") || strings.Contains(combinedText, "investment") ||
				strings.Contains(combinedText, "valuation") {
				extensionBonus += 0.1
			}
		case "creative_tools_focused":
			if strings.Contains(combinedText, "creative") || strings.Contains(combinedText, "design") ||
				strings.Contains(combinedText, "art") || strings.Contains(combinedText, "tool") {
				extensionBonus += 0.1
			}
		case "ai_creative_applications":
			if (strings.Contains(combinedText, "ai") || strings.Contains(combinedText, "artificial intelligence")) &&
				(strings.Contains(combinedText, "creative") || strings.Contains(combinedText, "art") || strings.Contains(combinedText, "design")) {
				extensionBonus += 0.15 // Higher bonus for AI+creative combination
			}
		}
	}

	// Multiple extension synergy bonus
	if len(extensions) >= 2 {
		// Check for specific high-value combinations
		hasAIResearch := false
		hasStartupEcosystem := false

		for _, ext := range extensions {
			if strings.Contains(ext, "ai_research") {
				hasAIResearch = true
			}
			if strings.Contains(ext, "startup_ecosystem") {
				hasStartupEcosystem = true
			}
		}

		// AI + Startup combination gets extra synergy bonus for relevant scenarios
		if hasAIResearch && hasStartupEcosystem {
			if strings.Contains(combinedText, "ai") && (strings.Contains(combinedText, "funding") || strings.Contains(combinedText, "startup")) {
				extensionBonus += 0.2 // Strong synergy bonus
			}
		}

		// General multi-extension bonus
		extensionBonus += float64(len(extensions)-1) * 0.05
	}

	// Apply the bonus but cap at 0.98 to be realistic
	newConfidence := baseConfidence + extensionBonus
	if newConfidence > 0.98 {
		newConfidence = 0.98
	}

	return newConfidence
}
