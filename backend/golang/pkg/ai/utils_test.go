package ai

import (
	"testing"
)

func TestStripThinkingTags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "removes think tags",
			input:    "<think>This is my reasoning</think>\n\nHere is the final answer.",
			expected: "Here is the final answer.",
		},
		{
			name:     "removes thinking tags",
			input:    "<thinking>Let me analyze this</thinking>\n\nThe conclusion is correct.",
			expected: "The conclusion is correct.",
		},
		{
			name:     "removes reasoning tags",
			input:    "<reasoning>Step by step analysis</reasoning>\n\nFinal result here.",
			expected: "Final result here.",
		},
		{
			name:     "handles multiple tags",
			input:    "<think>First thought</think>\n<reasoning>Analysis</reasoning>\n\nClean output.",
			expected: "Clean output.",
		},
		{
			name:     "case insensitive",
			input:    "<THINK>Uppercase</THINK>\n<ThInK>Mixed case</ThInK>\n\nResult.",
			expected: "Result.",
		},
		{
			name:     "multiline tags",
			input:    "<think>\nMultiple\nlines\nof reasoning\n</think>\n\nAnswer here.",
			expected: "Answer here.",
		},
		{
			name:     "no tags present",
			input:    "Just regular content without any thinking tags.",
			expected: "Just regular content without any thinking tags.",
		},
		{
			name:     "cleans up excessive newlines",
			input:    "<think>reasoning</think>\n\n\n\nResult with many newlines.",
			expected: "Result with many newlines.",
		},
		{
			name: "complex example with personality profile",
			input: `<think>
I need to analyze the user's data and create a profile. Let me organize this information systematically.
</think>

### **Profile Summary for PrimaryUser**

The user is a tech-focused professional with global interests.`,
			expected: `### **Profile Summary for PrimaryUser**

The user is a tech-focused professional with global interests.`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripThinkingTags(tt.input)
			if result != tt.expected {
				t.Errorf("StripThinkingTags() = %q, want %q", result, tt.expected)
			}
		})
	}
}
