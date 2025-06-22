package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/charmbracelet/log"
	"github.com/joho/godotenv"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory/storage"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/testing/personalities"
)

func main() {
	var (
		testDataPath = flag.String("data", "testdata", "Path to test data directory")
		outputPath   = flag.String("output", "personality_test_report.json", "Path for output report")
		verbose      = flag.Bool("verbose", false, "Enable verbose logging")
		listOnly     = flag.Bool("list", false, "List available personalities and scenarios without running tests")
	)
	flag.Parse()

	// Setup logging
	logger := log.New(os.Stdout)
	if *verbose {
		logger.SetLevel(log.DebugLevel)
	} else {
		logger.SetLevel(log.InfoLevel)
	}

	// Load environment variables
	_ = godotenv.Load()

	// Check for API key
	completionsKey := os.Getenv("COMPLETIONS_API_KEY")
	if completionsKey == "" {
		logger.Fatal("COMPLETIONS_API_KEY environment variable is required")
	}

	completionsURL := os.Getenv("COMPLETIONS_API_URL")
	if completionsURL == "" {
		completionsURL = "https://api.openai.com/v1"
	}

	logger.Info("Personality Testing Framework", "version", "1.0.0")

	// Initialize AI service
	aiService := ai.NewOpenAIService(logger, completionsKey, completionsURL)

	// Initialize personality test framework
	framework := personalities.NewPersonalityTestFramework(logger, aiService, *testDataPath)

	// Load personalities and scenarios
	logger.Info("Loading test data", "path", *testDataPath)

	err := framework.LoadPersonalities()
	if err != nil {
		logger.Fatal("Failed to load personalities", "error", err)
	}

	err = framework.LoadScenarios()
	if err != nil {
		logger.Fatal("Failed to load scenarios", "error", err)
	}

	personalities := framework.GetPersonalities()
	scenarios := framework.GetScenarios()

	logger.Info("Loaded test data",
		"personalities", len(personalities),
		"scenarios", len(scenarios))

	// List mode - just show what's available
	if *listOnly {
		fmt.Printf("\n=== Available Personalities ===\n")
		for name, personality := range personalities {
			fmt.Printf("- %s: %s\n", name, personality.Description)
			fmt.Printf("  Occupation: %s\n", personality.Profile.Occupation)
			fmt.Printf("  Interests: %v\n", personality.Profile.Interests)
			fmt.Printf("  Memory Facts: %d\n", len(personality.MemoryFacts))
		}

		fmt.Printf("\n=== Available Scenarios ===\n")
		for _, scenario := range scenarios {
			fmt.Printf("- %s: %s\n", scenario.Name, scenario.Description)
			fmt.Printf("  Expected: %s (confidence: %.2f)\n",
				scenario.Expected.ExpectedState, scenario.Expected.Confidence)
		}

		fmt.Printf("\nTotal test combinations: %d personalities × %d scenarios = %d tests\n",
			len(personalities), len(scenarios), len(personalities)*len(scenarios))
		return
	}

	// Validate we have test data
	if len(personalities) == 0 {
		logger.Fatal("No personalities found", "path", *testDataPath)
	}
	if len(scenarios) == 0 {
		logger.Fatal("No scenarios found", "path", *testDataPath)
	}

	// Create mock storage and repository for demonstration
	// In a real implementation, you'd connect to actual services
	mockStorage := &MockMemoryStorage{
		facts: make(map[string][]string),
	}

	// Create a mock holon repository that implements the same interface as holon.Repository
	mockRepo := &MockHolonRepository{}

	logger.Info("Starting personality tests",
		"total_combinations", len(personalities)*len(scenarios))

	// Run the personality test matrix - pass the mock repository directly
	ctx := context.Background()
	results, err := framework.RunPersonalityTests(ctx, mockStorage, mockRepo)
	if err != nil {
		logger.Fatal("Failed to run personality tests", "error", err)
	}

	logger.Info("Tests completed", "total_results", len(results))

	// Generate comprehensive report
	report := framework.GenerateReport(results)

	// Print summary to console
	framework.PrintSummary(report)

	// Save detailed report to file
	err = framework.SaveReport(report, *outputPath)
	if err != nil {
		logger.Fatal("Failed to save report", "error", err)
	}

	logger.Info("Report saved", "path", *outputPath)

	// Print final statistics
	fmt.Printf("\n=== Final Results ===\n")
	fmt.Printf("Overall Success Rate: %.1f%% (%d/%d tests passed)\n",
		float64(report.PassedTests)/float64(report.TotalTests)*100,
		report.PassedTests, report.TotalTests)
	fmt.Printf("Average Score: %.3f\n", report.OverallScore)

	if report.PassedTests < report.TotalTests {
		fmt.Printf("\nSome tests failed. Review the report for detailed analysis:\n")
		fmt.Printf("  %s\n", *outputPath)
		os.Exit(1)
	}

	fmt.Printf("\nAll tests passed! 🎉\n")
}

// MockMemoryStorage provides a simple mock implementation for the CLI tool
type MockMemoryStorage struct {
	facts map[string][]string
}

func (m *MockMemoryStorage) Store(ctx context.Context, documents []memory.Document, progressCallback memory.ProgressCallback) error {
	// Mock implementation - just store document IDs
	for i, doc := range documents {
		if progressCallback != nil {
			progressCallback(i+1, len(documents))
		}

		// Extract content from document
		var content string
		switch d := doc.(type) {
		case *memory.TextDocument:
			content = d.FieldContent
		default:
			content = "unknown document type"
		}

		// Store in mock facts
		category := "general"
		if m.facts[category] == nil {
			m.facts[category] = make([]string, 0)
		}
		m.facts[category] = append(m.facts[category], content)
	}
	return nil
}

func (m *MockMemoryStorage) Query(ctx context.Context, queryText string, filter *memory.Filter) (memory.QueryResult, error) {
	// Mock implementation - return personality-relevant facts
	result := memory.QueryResult{
		Facts: []memory.MemoryFact{}, // Fixed: removed pointer
	}

	// Simulate different responses based on query content
	if queryText != "" {
		// Add some mock personality facts
		mockFacts := []string{
			"User has strong interest in technology and innovation",
			"User prefers data-driven decision making",
			"User values creative expression and artistic quality",
			"User actively follows industry trends and developments",
		}

		for i, fact := range mockFacts {
			result.Facts = append(result.Facts, memory.MemoryFact{
				ID:      fmt.Sprintf("mock-fact-%d", i),
				Content: fact,
			})
		}
	}

	return result, nil
}

// GetDocumentReferences implements the missing method from evolvingmemory.MemoryStorage interface
func (m *MockMemoryStorage) GetDocumentReferences(ctx context.Context, memoryID string) ([]*storage.DocumentReference, error) {
	// Mock implementation - return empty references
	return []*storage.DocumentReference{}, nil
}
