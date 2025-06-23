package personalities

import (
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
