package personalities

import (
	"time"
)

// TestReport represents a comprehensive test report.
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

// PersonalityReport contains aggregated results for a specific personality.
type PersonalityReport struct {
	Name          string           `json:"name"`
	TotalTests    int              `json:"total_tests"`
	PassedTests   int              `json:"passed_tests"`
	AverageScore  float64          `json:"average_score"`
	BestScenario  string           `json:"best_scenario"`
	WorstScenario string           `json:"worst_scenario"`
	MemoryUsage   MemoryUsageStats `json:"memory_usage"`
}

// ScenarioReport contains aggregated results for a specific scenario.
type ScenarioReport struct {
	Name             string  `json:"name"`
	TotalTests       int     `json:"total_tests"`
	PassedTests      int     `json:"passed_tests"`
	AverageScore     float64 `json:"average_score"`
	BestPersonality  string  `json:"best_personality"`
	WorstPersonality string  `json:"worst_personality"`
}

// MemoryUsageStats tracks memory access patterns.
type MemoryUsageStats struct {
	TotalMemoriesAccessed  int            `json:"total_memories_accessed"`
	UniqueMemoriesAccessed int            `json:"unique_memories_accessed"`
	AverageMemoriesPerTest float64        `json:"average_memories_per_test"`
	MostAccessedMemories   []string       `json:"most_accessed_memories"`
	MemoryAccessPatterns   map[string]int `json:"memory_access_patterns"`
}
