package ai

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

func TestReplacementTrie_LongestMatchFirst(t *testing.T) {
	trie := NewReplacementTrie()

	// Insert patterns with overlapping prefixes
	trie.Insert("Ivan", "ANON_2")
	trie.Insert("Ivan Ivanov", "ANON_1")
	trie.Insert("John", "PERSON_001")
	trie.Insert("John Smith", "PERSON_002")
	trie.Insert("OpenAI", "COMPANY_001")
	trie.Insert("San Francisco", "LOCATION_006")

	testCases := []struct {
		name     string
		input    string
		expected string
		rules    map[string]string
	}{
		{
			name:     "Longest match first - Ivan Ivanov",
			input:    "Hello Ivan Ivanov, this is from Ivan",
			expected: "Hello ANON_1, this is from ANON_2",
			rules: map[string]string{
				"ANON_1": "Ivan Ivanov",
				"ANON_2": "Ivan",
			},
		},
		{
			name:     "Longest match first - John Smith",
			input:    "Meet John Smith and John",
			expected: "Meet PERSON_002 and PERSON_001",
			rules: map[string]string{
				"PERSON_001": "John",
				"PERSON_002": "John Smith",
			},
		},
		{
			name:     "Multiple overlapping matches",
			input:    "Ivan Ivanov and Ivan work at OpenAI in San Francisco",
			expected: "ANON_1 and ANON_2 work at COMPANY_001 in LOCATION_006",
			rules: map[string]string{
				"ANON_1":       "Ivan Ivanov",
				"ANON_2":       "Ivan",
				"COMPANY_001":  "OpenAI",
				"LOCATION_006": "San Francisco",
			},
		},
		{
			name:     "No matches",
			input:    "Hello World",
			expected: "Hello World",
			rules:    map[string]string{},
		},
		{
			name:     "Partial matches only",
			input:    "Iv and Jo work together",
			expected: "Iv and Jo work together",
			rules:    map[string]string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, rules := trie.ReplaceAll(tc.input)

			if result != tc.expected {
				t.Errorf("Expected: %q, got: %q", tc.expected, result)
			}

			// Check rules
			if len(rules) != len(tc.rules) {
				t.Errorf("Expected %d rules, got %d", len(tc.rules), len(rules))
			}

			for token, expectedOriginal := range tc.rules {
				if actual, exists := rules[token]; !exists {
					t.Errorf("Missing rule for token '%s'", token)
				} else if actual != expectedOriginal {
					t.Errorf("Wrong rule for '%s': expected '%s', got '%s'", token, expectedOriginal, actual)
				}
			}

			t.Logf("Input: %s", tc.input)
			t.Logf("Output: %s", result)
			t.Logf("Rules: %v", rules)
		})
	}
}

func TestReplacementTrie_Size(t *testing.T) {
	trie := NewReplacementTrie()

	if size := trie.Size(); size != 0 {
		t.Errorf("Expected size 0 for empty trie, got %d", size)
	}

	trie.Insert("Ivan", "ANON_2")
	trie.Insert("Ivan Ivanov", "ANON_1")
	trie.Insert("John", "PERSON_001")

	if size := trie.Size(); size != 3 {
		t.Errorf("Expected size 3, got %d", size)
	}
}

func TestReplacementTrie_EmptyInput(t *testing.T) {
	trie := NewReplacementTrie()
	trie.Insert("test", "TEST")

	result, rules := trie.ReplaceAll("")

	if result != "" {
		t.Errorf("Expected empty result, got %q", result)
	}

	if len(rules) != 0 {
		t.Errorf("Expected no rules, got %v", rules)
	}
}

func TestReplacementTrie_UnicodeSupport(t *testing.T) {
	trie := NewReplacementTrie()
	trie.Insert("café", "PLACE_001")
	trie.Insert("naïve", "WORD_001")
	trie.Insert("北京", "CITY_001")

	result, rules := trie.ReplaceAll("I went to café in 北京 with a naïve friend")
	expected := "I went to PLACE_001 in CITY_001 with a WORD_001 friend"

	if result != expected {
		t.Errorf("Expected: %q, got: %q", expected, result)
	}

	expectedRules := map[string]string{
		"PLACE_001": "café",
		"CITY_001":  "北京",
		"WORD_001":  "naïve",
	}

	for token, expectedOriginal := range expectedRules {
		if actual, exists := rules[token]; !exists {
			t.Errorf("Missing rule for token '%s'", token)
		} else if actual != expectedOriginal {
			t.Errorf("Wrong rule for '%s': expected '%s', got '%s'", token, expectedOriginal, actual)
		}
	}
}

// Benchmark comparison with current sorting approach.
func BenchmarkSortingApproach_ReplaceAll(b *testing.B) {
	patterns := map[string]string{
		"Ivan":          "ANON_2",
		"Ivan Ivanov":   "ANON_1",
		"John":          "PERSON_001",
		"John Smith":    "PERSON_002",
		"OpenAI":        "COMPANY_001",
		"Microsoft":     "COMPANY_002",
		"Google":        "COMPANY_003",
		"San Francisco": "LOCATION_006",
		"New York":      "LOCATION_001",
	}

	text := "Hello Ivan Ivanov, this is from Ivan who works at OpenAI in San Francisco with John Smith and John from Microsoft in New York"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate current sorting approach
		result := text

		// Create sorted slice by length (longest first)
		type replacement struct {
			original string
			token    string
		}

		var sortedReplacements []replacement
		for original, token := range patterns {
			sortedReplacements = append(sortedReplacements, replacement{original: original, token: token})
		}

		// Sort by length descending using Go's built-in sort
		sort.Slice(sortedReplacements, func(i, j int) bool {
			return len(sortedReplacements[i].original) > len(sortedReplacements[j].original)
		})

		// Apply replacements
		for _, repl := range sortedReplacements {
			result = strings.ReplaceAll(result, repl.original, repl.token)
		}

		_ = result
	}
}

func BenchmarkReplacementTrie_ReplaceAll(b *testing.B) {
	trie := NewReplacementTrie()

	// Insert patterns
	patterns := map[string]string{
		"Ivan":          "ANON_2",
		"Ivan Ivanov":   "ANON_1",
		"John":          "PERSON_001",
		"John Smith":    "PERSON_002",
		"OpenAI":        "COMPANY_001",
		"Microsoft":     "COMPANY_002",
		"Google":        "COMPANY_003",
		"San Francisco": "LOCATION_006",
		"New York":      "LOCATION_001",
	}

	for pattern, replacement := range patterns {
		trie.Insert(pattern, replacement)
	}

	text := "Hello Ivan Ivanov, this is from Ivan who works at OpenAI in San Francisco with John Smith and John from Microsoft in New York"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = trie.ReplaceAll(text)
	}
}

// Benchmark with many patterns to show scalability.
func BenchmarkSortingApproach_ManyPatterns(b *testing.B) {
	patterns := make(map[string]string)

	// Add 100 patterns
	for i := 0; i < 100; i++ {
		patterns[fmt.Sprintf("pattern_%d", i)] = fmt.Sprintf("TOKEN_%d", i)
		patterns[fmt.Sprintf("long_pattern_%d_with_more_text", i)] = fmt.Sprintf("LONG_TOKEN_%d", i)
	}

	text := "This is a test with pattern_5 and long_pattern_10_with_more_text and pattern_25 and some other pattern_50 text"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := text

		// Create sorted slice by length (longest first)
		type replacement struct {
			original string
			token    string
		}

		var sortedReplacements []replacement
		for original, token := range patterns {
			sortedReplacements = append(sortedReplacements, replacement{original: original, token: token})
		}

		// Sort by length descending using Go's built-in sort - O(n log n)
		sort.Slice(sortedReplacements, func(i, j int) bool {
			return len(sortedReplacements[i].original) > len(sortedReplacements[j].original)
		})

		// Apply replacements
		for _, repl := range sortedReplacements {
			result = strings.ReplaceAll(result, repl.original, repl.token)
		}

		_ = result
	}
}

func BenchmarkReplacementTrie_ManyPatterns(b *testing.B) {
	trie := NewReplacementTrie()

	// Add 100 patterns
	for i := 0; i < 100; i++ {
		trie.Insert(fmt.Sprintf("pattern_%d", i), fmt.Sprintf("TOKEN_%d", i))
		trie.Insert(fmt.Sprintf("long_pattern_%d_with_more_text", i), fmt.Sprintf("LONG_TOKEN_%d", i))
	}

	text := "This is a test with pattern_5 and long_pattern_10_with_more_text and pattern_25 and some other pattern_50 text"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = trie.ReplaceAll(text)
	}
}

func TestReplacementTrie_WordBoundaries(t *testing.T) {
	trie := NewReplacementTrie()
	trie.Insert("2", "1")
	trie.Insert("John", "PERSON_001")
	trie.Insert("test", "WORD")

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Digit boundaries - should not replace digits within year",
			input:    "The year 2025 started",
			expected: "The year 2025 started",
		},
		{
			name:     "Word boundaries - should not replace part of word",
			input:    "Johnson visited John",
			expected: "Johnson visited PERSON_001",
		},
		{
			name:     "Punctuation boundaries - should replace standalone words",
			input:    "testing test, contest test.",
			expected: "testing WORD, contest WORD.",
		},
		{
			name:     "Start and end boundaries",
			input:    "test at start and end test",
			expected: "WORD at start and end WORD",
		},
		{
			name:     "Digit boundaries - standalone digit should replace",
			input:    "Number 2 in the list",
			expected: "Number 1 in the list",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, _ := trie.ReplaceAll(tc.input)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestMergeRules(t *testing.T) {
	testCases := []struct {
		name     string
		input    map[string]string
		expected map[string]string
	}{
		{
			name: "Basic case merging",
			input: map[string]string{
				"innokentii": "boris",
				"InnokenTii": "Boris",
			},
			expected: map[string]string{
				"innokentii": "boris",
			},
		},
		{
			name: "Multiple case variations",
			input: map[string]string{
				"john": "person1",
				"John": "Person1",
				"JOHN": "PERSON1",
				"JoHn": "PeRsOn1",
				"jane": "person2",
				"Jane": "Person2",
			},
			expected: map[string]string{
				"john": "person1",
				"jane": "person2",
			},
		},
		{
			name: "No duplicates - should remain unchanged",
			input: map[string]string{
				"alice": "person1",
				"bob":   "person2",
				"carol": "person3",
			},
			expected: map[string]string{
				"alice": "person1",
				"bob":   "person2",
				"carol": "person3",
			},
		},
		{
			name:     "Empty input",
			input:    map[string]string{},
			expected: map[string]string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := MergeRules(tc.input)

			if len(result) != len(tc.expected) {
				t.Errorf("Expected %d rules, got %d", len(tc.expected), len(result))
				return
			}

			for expectedKey, expectedValue := range tc.expected {
				if actualValue, exists := result[expectedKey]; !exists {
					t.Errorf("Expected key %q not found in result", expectedKey)
				} else if actualValue != expectedValue {
					t.Errorf("For key %q: expected %q, got %q", expectedKey, expectedValue, actualValue)
				}
			}
		})
	}
}

func TestNewReplacementTrieFromRules(t *testing.T) {
	rules := map[string]string{
		"innokentii": "boris",
		"InnokenTii": "Boris",
		"JOHN":       "PERSON1",
		"john":       "person1",
		"Jane":       "Person2",
	}

	trie := NewReplacementTrieFromRules(rules)

	// Should have 3 unique rules after merging (innokentii, john, jane)
	if trie.Size() != 3 {
		t.Errorf("Expected 3 rules after merging, got %d", trie.Size())
	}

	result, _ := trie.ReplaceAll("Hello innokentii and john and Jane")
	expected := "Hello boris and person1 and Person2"

	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}
