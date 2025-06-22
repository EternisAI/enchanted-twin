package personalities

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// TestReport represents a comprehensive test report
type TestReport struct {
	GeneratedAt        time.Time                    `json:"generated_at"`
	TotalTests         int                          `json:"total_tests"`
	PassedTests        int                          `json:"passed_tests"`
	FailedTests        int                          `json:"failed_tests"`
	OverallScore       float64                      `json:"overall_score"`
	PersonalityResults map[string]PersonalityReport `json:"personality_results"`
	ScenarioResults    map[string]ScenarioReport    `json:"scenario_results"`
	DetailedResults    []TestResult                 `json:"detailed_results"`
}

// PersonalityReport contains aggregated results for a specific personality
type PersonalityReport struct {
	Name          string           `json:"name"`
	TotalTests    int              `json:"total_tests"`
	PassedTests   int              `json:"passed_tests"`
	AverageScore  float64          `json:"average_score"`
	BestScenario  string           `json:"best_scenario"`
	WorstScenario string           `json:"worst_scenario"`
	MemoryUsage   MemoryUsageStats `json:"memory_usage"`
}

// ScenarioReport contains aggregated results for a specific scenario
type ScenarioReport struct {
	Name             string  `json:"name"`
	TotalTests       int     `json:"total_tests"`
	PassedTests      int     `json:"passed_tests"`
	AverageScore     float64 `json:"average_score"`
	BestPersonality  string  `json:"best_personality"`
	WorstPersonality string  `json:"worst_personality"`
}

// MemoryUsageStats tracks memory access patterns
type MemoryUsageStats struct {
	TotalMemoriesAccessed  int            `json:"total_memories_accessed"`
	UniqueMemoriesAccessed int            `json:"unique_memories_accessed"`
	AverageMemoriesPerTest float64        `json:"average_memories_per_test"`
	MostAccessedMemories   []string       `json:"most_accessed_memories"`
	MemoryAccessPatterns   map[string]int `json:"memory_access_patterns"`
}

// GenerateReport creates a comprehensive test report from results
func (ptf *PersonalityTestFramework) GenerateReport(results []TestResult) *TestReport {
	report := &TestReport{
		GeneratedAt:        time.Now(),
		TotalTests:         len(results),
		PersonalityResults: make(map[string]PersonalityReport),
		ScenarioResults:    make(map[string]ScenarioReport),
		DetailedResults:    results,
	}

	// Calculate overall statistics
	totalScore := 0.0
	passedTests := 0

	// Group results by personality and scenario
	personalityGroups := make(map[string][]TestResult)
	scenarioGroups := make(map[string][]TestResult)

	for _, result := range results {
		if result.Success {
			passedTests++
		}
		totalScore += result.Score

		personalityGroups[result.PersonalityName] = append(personalityGroups[result.PersonalityName], result)
		scenarioGroups[result.ScenarioName] = append(scenarioGroups[result.ScenarioName], result)
	}

	report.PassedTests = passedTests
	report.FailedTests = len(results) - passedTests
	if len(results) > 0 {
		report.OverallScore = totalScore / float64(len(results))
	}

	// Generate personality reports
	for name, results := range personalityGroups {
		report.PersonalityResults[name] = ptf.generatePersonalityReport(name, results)
	}

	// Generate scenario reports
	for name, results := range scenarioGroups {
		report.ScenarioResults[name] = ptf.generateScenarioReport(name, results)
	}

	return report
}

// generatePersonalityReport creates a report for a specific personality
func (ptf *PersonalityTestFramework) generatePersonalityReport(name string, results []TestResult) PersonalityReport {
	totalScore := 0.0
	passedTests := 0
	bestScore := 0.0
	worstScore := 1.0
	bestScenario := ""
	worstScenario := ""

	memoryAccess := make(map[string]int)
	totalMemoriesAccessed := 0

	for _, result := range results {
		if result.Success {
			passedTests++
		}
		totalScore += result.Score

		if result.Score > bestScore {
			bestScore = result.Score
			bestScenario = result.ScenarioName
		}
		if result.Score < worstScore {
			worstScore = result.Score
			worstScenario = result.ScenarioName
		}

		// Track memory usage
		for _, memoryID := range result.MemoriesUsed {
			memoryAccess[memoryID]++
			totalMemoriesAccessed++
		}
	}

	// Calculate memory statistics
	uniqueMemories := len(memoryAccess)
	avgMemoriesPerTest := 0.0
	if len(results) > 0 {
		avgMemoriesPerTest = float64(totalMemoriesAccessed) / float64(len(results))
	}

	// Find most accessed memories
	type memoryCount struct {
		id    string
		count int
	}
	var memoryCounts []memoryCount
	for id, count := range memoryAccess {
		memoryCounts = append(memoryCounts, memoryCount{id, count})
	}
	sort.Slice(memoryCounts, func(i, j int) bool {
		return memoryCounts[i].count > memoryCounts[j].count
	})

	mostAccessed := make([]string, 0)
	for i, mc := range memoryCounts {
		if i >= 5 { // Top 5 most accessed
			break
		}
		mostAccessed = append(mostAccessed, mc.id)
	}

	averageScore := 0.0
	if len(results) > 0 {
		averageScore = totalScore / float64(len(results))
	}

	return PersonalityReport{
		Name:          name,
		TotalTests:    len(results),
		PassedTests:   passedTests,
		AverageScore:  averageScore,
		BestScenario:  bestScenario,
		WorstScenario: worstScenario,
		MemoryUsage: MemoryUsageStats{
			TotalMemoriesAccessed:  totalMemoriesAccessed,
			UniqueMemoriesAccessed: uniqueMemories,
			AverageMemoriesPerTest: avgMemoriesPerTest,
			MostAccessedMemories:   mostAccessed,
			MemoryAccessPatterns:   memoryAccess,
		},
	}
}

// generateScenarioReport creates a report for a specific scenario
func (ptf *PersonalityTestFramework) generateScenarioReport(name string, results []TestResult) ScenarioReport {
	totalScore := 0.0
	passedTests := 0
	bestScore := 0.0
	worstScore := 1.0
	bestPersonality := ""
	worstPersonality := ""

	for _, result := range results {
		if result.Success {
			passedTests++
		}
		totalScore += result.Score

		if result.Score > bestScore {
			bestScore = result.Score
			bestPersonality = result.PersonalityName
		}
		if result.Score < worstScore {
			worstScore = result.Score
			worstPersonality = result.PersonalityName
		}
	}

	averageScore := 0.0
	if len(results) > 0 {
		averageScore = totalScore / float64(len(results))
	}

	return ScenarioReport{
		Name:             name,
		TotalTests:       len(results),
		PassedTests:      passedTests,
		AverageScore:     averageScore,
		BestPersonality:  bestPersonality,
		WorstPersonality: worstPersonality,
	}
}

// SaveReport saves the test report to a JSON file
func (ptf *PersonalityTestFramework) SaveReport(report *TestReport, outputPath string) error {
	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Marshal report to JSON
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	// Write to file
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write report file: %w", err)
	}

	ptf.logger.Info("Test report saved", "path", outputPath)
	return nil
}

// PrintSummary prints a human-readable summary of the test results
func (ptf *PersonalityTestFramework) PrintSummary(report *TestReport) {
	fmt.Printf("\n=== Personality Testing Report ===\n")
	fmt.Printf("Generated: %s\n", report.GeneratedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Total Tests: %d\n", report.TotalTests)
	fmt.Printf("Passed: %d (%.1f%%)\n", report.PassedTests, float64(report.PassedTests)/float64(report.TotalTests)*100)
	fmt.Printf("Failed: %d (%.1f%%)\n", report.FailedTests, float64(report.FailedTests)/float64(report.TotalTests)*100)
	fmt.Printf("Overall Score: %.3f\n", report.OverallScore)

	fmt.Printf("\n=== Personality Performance ===\n")
	for name, result := range report.PersonalityResults {
		fmt.Printf("%s: %.3f avg score, %d/%d passed\n",
			name, result.AverageScore, result.PassedTests, result.TotalTests)
		fmt.Printf("  Best: %s, Worst: %s\n", result.BestScenario, result.WorstScenario)
		fmt.Printf("  Memory: %.1f avg/test, %d unique accessed\n",
			result.MemoryUsage.AverageMemoriesPerTest, result.MemoryUsage.UniqueMemoriesAccessed)
	}

	fmt.Printf("\n=== Scenario Difficulty ===\n")
	for name, result := range report.ScenarioResults {
		fmt.Printf("%s: %.3f avg score, %d/%d passed\n",
			name, result.AverageScore, result.PassedTests, result.TotalTests)
		fmt.Printf("  Best: %s, Worst: %s\n", result.BestPersonality, result.WorstPersonality)
	}

	// Show detailed failures
	fmt.Printf("\n=== Failed Tests ===\n")
	for _, result := range report.DetailedResults {
		if !result.Success {
			fmt.Printf("%s x %s: %.3f score\n",
				result.PersonalityName, result.ScenarioName, result.Score)
			fmt.Printf("  Expected: %t, Got: %t\n",
				result.ExpectedResult.ShouldShow, result.ActualResult.ShouldShow)
			fmt.Printf("  Reasoning: %s\n", result.Reasoning)
		}
	}
}
