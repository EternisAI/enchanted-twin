package timeparser

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/charmbracelet/log"
)

func TestTimeParserTool_Execute(t *testing.T) {
	logger := log.New(os.Stderr)
	tool := NewTimeParserTool(logger)

	tests := []struct {
		name        string
		input       map[string]any
		wantError   bool
		checkResult func(types.ToolResult) bool
	}{
		{
			name: "simple time expression",
			input: map[string]any{
				"text": "Meet me at 3pm today",
			},
			wantError: false,
			checkResult: func(result types.ToolResult) bool {
				content := result.Content()
				return strings.Contains(content, "UTC") && strings.Contains(content, "15:00:00")
			},
		},
		{
			name: "tomorrow with time",
			input: map[string]any{
				"text": "The meeting is tomorrow at 9:30 AM",
			},
			wantError: false,
			checkResult: func(result types.ToolResult) bool {
				content := result.Content()
				return strings.Contains(content, "UTC") && strings.Contains(content, "09:30:00")
			},
		},
		{
			name: "next week",
			input: map[string]any{
				"text": "Let's schedule this for next Monday",
			},
			wantError: false,
			checkResult: func(result types.ToolResult) bool {
				content := result.Content()
				return strings.Contains(content, "UTC") && strings.Contains(content, "next Monday")
			},
		},
		{
			name: "specific date and time",
			input: map[string]any{
				"text": "The deadline is January 20th at 5pm",
			},
			wantError: false,
			checkResult: func(result types.ToolResult) bool {
				content := result.Content()
				return strings.Contains(content, "UTC") && strings.Contains(content, "17:00:00")
			},
		},
		{
			name: "time with timezone",
			input: map[string]any{
				"text":     "Conference call at 2pm",
				"timezone": "America/New_York",
			},
			wantError: false,
			checkResult: func(result types.ToolResult) bool {
				content := result.Content()
				return strings.Contains(content, "UTC") && strings.Contains(content, "America/New_York")
			},
		},
		{
			name: "relative time - in hours",
			input: map[string]any{
				"text": "Let's meet in 2 hours",
			},
			wantError: false,
			checkResult: func(result types.ToolResult) bool {
				content := result.Content()
				return strings.Contains(content, "UTC") && strings.Contains(content, "in 2 hours")
			},
		},
		{
			name: "relative time - ago",
			input: map[string]any{
				"text": "This happened 3 hours ago",
			},
			wantError: false,
			checkResult: func(result types.ToolResult) bool {
				content := result.Content()
				return strings.Contains(content, "UTC") && strings.Contains(content, "3 hours ago")
			},
		},
		{
			name: "multiple time expressions",
			input: map[string]any{
				"text": "Meeting at 9am tomorrow, lunch at 12pm, and dinner at 7pm",
			},
			wantError: false,
			checkResult: func(result types.ToolResult) bool {
				content := result.Content()
				utcCount := strings.Count(content, "UTC")
				return utcCount >= 2
			},
		},
		{
			name: "24-hour format",
			input: map[string]any{
				"text": "The server restarts at 14:30 daily",
			},
			wantError: false,
			checkResult: func(result types.ToolResult) bool {
				content := result.Content()
				return strings.Contains(content, "UTC")
			},
		},
		{
			name: "date with slashes",
			input: map[string]any{
				"text": "Project due date is 1/20/2024",
			},
			wantError: false,
			checkResult: func(result types.ToolResult) bool {
				content := result.Content()
				return strings.Contains(content, "UTC") && strings.Contains(content, "2024")
			},
		},
		{
			name: "no time expressions",
			input: map[string]any{
				"text": "This text has no time expressions in it",
			},
			wantError: false,
			checkResult: func(result types.ToolResult) bool {
				content := result.Content()
				return content == "This text has no time expressions in it"
			},
		},
		{
			name: "missing text parameter",
			input: map[string]any{
				"not_text": "value",
			},
			wantError: true,
			checkResult: func(result types.ToolResult) bool {
				return false
			},
		},
		{
			name: "invalid text type",
			input: map[string]any{
				"text": 123,
			},
			wantError: true,
			checkResult: func(result types.ToolResult) bool {
				return false
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), tt.input)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Errorf("Expected non-nil result")
				return
			}

			if !tt.checkResult(result) {
				t.Errorf("Result check failed. Got: %s", result.Content())
			}
		})
	}
}

func TestTimeParserTool_ParseAndReplaceTimeValues(t *testing.T) {
	logger := log.New(os.Stderr)
	tool := NewTimeParserTool(logger)

	estLocation, _ := time.LoadLocation("America/New_York")
	utcLocation, _ := time.LoadLocation("UTC")

	tests := []struct {
		name          string
		text          string
		timezone      *time.Location
		shouldContain []string
	}{
		{
			name:          "EST timezone conversion",
			text:          "Meeting at 3pm today",
			timezone:      estLocation,
			shouldContain: []string{"UTC", "America/New_York"},
		},
		{
			name:          "UTC timezone",
			text:          "Call scheduled for tomorrow 10am",
			timezone:      utcLocation,
			shouldContain: []string{"UTC", "tomorrow 10am"},
		},
		{
			name:          "Complex sentence with multiple times",
			text:          "Start at 9am, break at 12pm, resume at 1pm, and finish by 5pm",
			timezone:      time.Local,
			shouldContain: []string{"UTC"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.parseAndReplaceTimeValues(tt.text, tt.timezone)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			for _, shouldContain := range tt.shouldContain {
				if !strings.Contains(result, shouldContain) {
					t.Errorf("Result should contain '%s'. Got: %s", shouldContain, result)
				}
			}
		})
	}
}

func TestTimeParserTool_ExtendTimeContext(t *testing.T) {
	logger := log.New(os.Stderr)
	tool := NewTimeParserTool(logger)

	tests := []struct {
		name     string
		text     string
		start    int
		end      int
		expected [2]int
	}{
		{
			name:     "extend with 'at'",
			text:     "Meet me at 3pm",
			start:    11,
			end:      14,
			expected: [2]int{8, 14},
		},
		{
			name:     "extend with 'next'",
			text:     "See you next Monday",
			start:    13,
			end:      19,
			expected: [2]int{8, 19},
		},
		{
			name:     "no extension needed",
			text:     "Time is 14:30 exactly",
			start:    8,
			end:      13,
			expected: [2]int{8, 13},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tool.extendTimeContext(tt.text, tt.start, tt.end)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v for text '%s'", tt.expected, result, tt.text)
			}
		})
	}
}

func TestTimeParserTool_Definition(t *testing.T) {
	logger := log.New(os.Stderr)
	tool := NewTimeParserTool(logger)

	def := tool.Definition()

	if def.Function.Name != "time_parser_tool" {
		t.Errorf("Expected function name 'time_parser_tool', got '%s'", def.Function.Name)
	}

	desc := def.Function.Description.Value
	if desc == "" {
		t.Errorf("Expected non-empty description")
	}

	params := def.Function.Parameters
	if params["type"] != "object" {
		t.Errorf("Expected parameters type to be 'object'")
	}

	properties, ok := params["properties"].(map[string]any)
	if !ok {
		t.Errorf("Expected properties to be a map")
		return
	}

	if _, exists := properties["text"]; !exists {
		t.Errorf("Expected 'text' property to exist")
	}

	if _, exists := properties["timezone"]; !exists {
		t.Errorf("Expected 'timezone' property to exist")
	}
}

// Benchmark tests
func BenchmarkTimeParserTool_Execute(b *testing.B) {
	logger := log.New(os.Stderr)
	tool := NewTimeParserTool(logger)

	input := map[string]any{
		"text": "Meeting tomorrow at 2pm, lunch next Monday at 12:30, and conference call in 3 hours",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := tool.Execute(context.Background(), input)
		if err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}

func BenchmarkTimeParserTool_ParseComplexText(b *testing.B) {
	logger := log.New(os.Stderr)
	tool := NewTimeParserTool(logger)

	complexText := `The project kickoff is scheduled for next Monday at 9:00 AM EST. 
	We'll have a status update every Tuesday at 2pm, and the milestone review is 
	set for January 30th at 3:30 PM. Please join the daily standup tomorrow morning at 10am, 
	and don't forget the retrospective meeting in 2 weeks at 4pm on Friday.`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := tool.parseAndReplaceTimeValues(complexText, time.Local)
		if err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}
