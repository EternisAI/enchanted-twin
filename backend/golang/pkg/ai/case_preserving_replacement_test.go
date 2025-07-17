package ai

import (
	"testing"
)

func TestCasePreservingReplacer_SimpleWords(t *testing.T) {
	rules := map[string]string{
		"andrey": "fedor",
		"alice":  "bob",
	}

	replacer := NewCasePreservingReplacer(rules)

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		// Simple word variations
		{"lowercase", "andrey", "fedor"},
		{"capitalized", "Andrey", "Fedor"},
		{"uppercase", "ANDREY", "FEDOR"},
		{"mixed case", "ANdrey", "Fedor"},
		{"different mixed case", "aNDREY", "Fedor"},

		// Context testing
		{"in sentence lowercase", "Hello andrey!", "Hello fedor!"},
		{"in sentence capitalized", "Hello Andrey!", "Hello Fedor!"},
		{"in sentence uppercase", "Hello ANDREY!", "Hello FEDOR!"},
		{"in sentence mixed", "Hello ANdrey!", "Hello Fedor!"},

		// Multiple replacements
		{"multiple simple", "andrey and alice", "fedor and bob"},
		{"multiple capitalized", "Andrey and Alice", "Fedor and Bob"},
		{"multiple uppercase", "ANDREY AND ALICE", "FEDOR AND BOB"},
		{"multiple mixed", "ANdrey and ALice", "Fedor and Bob"},

		// No replacement
		{"no match", "john", "john"},
		{"partial match", "andreya", "andreya"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := replacer.Replace(tc.input)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestCasePreservingReplacer_CompoundWords(t *testing.T) {
	rules := map[string]string{
		"jane bush":     "john doe",
		"alice johnson": "bob smith",
	}

	replacer := NewCasePreservingReplacer(rules)

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		// Compound word variations
		{"lowercase", "jane bush", "john doe"},
		{"title case", "Jane Bush", "John Doe"},
		{"uppercase", "JANE BUSH", "JOHN DOE"},
		{"mixed case", "JaNe BuSh", "John Doe"},
		{"different mixed", "JANE bush", "JOHN doe"},
		{"another mixed", "jane BUSH", "john DOE"},

		// Context testing
		{"in sentence lowercase", "Hello jane bush!", "Hello john doe!"},
		{"in sentence title", "Hello Jane Bush!", "Hello John Doe!"},
		{"in sentence uppercase", "Hello JANE BUSH!", "Hello JOHN DOE!"},
		{"in sentence mixed", "Hello JaNe BuSh!", "Hello John Doe!"},

		// Multiple compound words
		{"multiple lowercase", "jane bush and alice johnson", "john doe and bob smith"},
		{"multiple title", "Jane Bush and Alice Johnson", "John Doe and Bob Smith"},
		{"multiple uppercase", "JANE BUSH AND ALICE JOHNSON", "JOHN DOE AND BOB SMITH"},
		{"multiple mixed", "JaNe BuSh and ALice JOHnson", "John Doe and Bob Smith"},

		// No replacement
		{"no match", "mary jane", "mary jane"},
		{"partial match", "jane bushes", "jane bushes"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := replacer.Replace(tc.input)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestCasePreservingReplacer_MixedSimpleAndCompound(t *testing.T) {
	rules := map[string]string{
		"john smith": "person_001",
		"jane doe":   "person_002",
		"john":       "person_003",
		"jane":       "person_004",
		"openai":     "company_001",
	}

	replacer := NewCasePreservingReplacer(rules)

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		// Longest match first (compound before simple)
		{"compound over simple", "john smith works at openai", "person_001 works at company_001"},
		{"compound capitalized", "John Smith works at OpenAI", "Person_001 works at Company_001"},
		{"compound uppercase", "JOHN SMITH WORKS AT OPENAI", "PERSON_001 WORKS AT COMPANY_001"},
		{"compound mixed", "John SMITH works at OpenAI", "Person_001 works at Company_001"},

		// Individual names when compound not matched
		{"individual john", "john works here", "person_003 works here"},
		{"individual jane", "jane is here", "person_004 is here"},
		{"individual capitalized", "John and Jane", "Person_003 and Person_004"},
		{"individual uppercase", "JOHN AND JANE", "PERSON_003 AND PERSON_004"},

		// Complex sentences
		{"complex sentence", "john smith and jane doe work at openai", "person_001 and person_002 work at company_001"},
		{"complex capitalized", "John Smith and Jane Doe work at OpenAI", "Person_001 and Person_002 work at Company_001"},
		{"complex mixed", "JOHN SMITH and Jane DOE work at OpenAI", "PERSON_001 and Person_002 work at Company_001"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := replacer.Replace(tc.input)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestCasePreservingReplacer_EdgeCases(t *testing.T) {
	rules := map[string]string{
		"a":   "x",
		"ab":  "xy",
		"abc": "xyz",
	}

	replacer := NewCasePreservingReplacer(rules)

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		// Length sorting test
		{"longest first", "abc", "xyz"},
		{"middle length", "ab", "xy"},
		{"shortest", "a", "x"},

		// Case preservation with different lengths
		{"longest uppercase", "ABC", "XYZ"},
		{"middle uppercase", "AB", "XY"},
		{"shortest uppercase", "A", "X"},

		// Mixed case patterns - follow rule 3: capitalize only first letter
		{"mixed case abc", "AbC", "Xyz"},
		{"mixed case ab", "Ab", "Xy"},
		{"mixed case a", "A", "X"},

		// Empty and special cases
		{"empty string", "", ""},
		{"no rules match", "def", "def"},
		{"spaces", "a b c", "x b c"},
		{"punctuation", "a, ab, abc", "x, xy, xyz"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := replacer.Replace(tc.input)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestCasePreservingReplacer_MixedCasePatterns(t *testing.T) {
	rules := map[string]string{
		"test": "demo",
	}

	replacer := NewCasePreservingReplacer(rules)

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		// Various mixed case patterns - all should become "Demo" (capitalized first letter)
		{"tEsT", "tEsT", "Demo"},
		{"TeSt", "TeSt", "Demo"},
		{"tEST", "tEST", "Demo"},
		{"TesT", "TesT", "Demo"},
		{"tEst", "tEst", "Demo"},
		{"TESt", "TESt", "Demo"},
		{"teST", "teST", "Demo"},
		{"TEsT", "TEsT", "Demo"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := replacer.Replace(tc.input)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestApplyReplacements_IntegrationWithMockAnonymizer(t *testing.T) {
	rules := map[string]string{
		"john smith": "PERSON_001",
		"jane doe":   "PERSON_002",
		"openai":     "COMPANY_001",
	}

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple anonymization", "Hello john smith", "Hello PERSON_001"},
		{"capitalized", "Hello John Smith", "Hello PERSON_001"},
		{"uppercase", "Hello JOHN SMITH", "Hello PERSON_001"},
		{"mixed case", "Hello JoHn SmItH", "Hello PERSON_001"},
		{"multiple entities", "john smith works at openai", "PERSON_001 works at COMPANY_001"},
		{"mixed case multiple", "John SMITH works at OpenAI", "PERSON_001 works at COMPANY_001"},
		{"complex sentence", "john smith and jane doe both work at openai", "PERSON_001 and PERSON_002 both work at COMPANY_001"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ApplyAnonymization(tc.input, rules)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestCasePreservingReplacer_DeAnonymization(t *testing.T) {
	// Test reverse mapping for de-anonymization
	rules := map[string]string{
		"PERSON_001":  "john smith",
		"PERSON_002":  "jane doe",
		"COMPANY_001": "openai",
	}

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple deanonymization", "Hello PERSON_001", "Hello john smith"},
		{"multiple entities", "PERSON_001 works at COMPANY_001", "john smith works at openai"},
		{"complex sentence", "PERSON_001 and PERSON_002 both work at COMPANY_001", "john smith and jane doe both work at openai"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ApplyDeAnonymization(tc.input, rules)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestCasePreservingReplacer_EmptyAndNilRules(t *testing.T) {
	testCases := []struct {
		name     string
		rules    map[string]string
		input    string
		expected string
	}{
		{"empty rules", map[string]string{}, "hello world", "hello world"},
		{"nil rules", nil, "hello world", "hello world"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ApplyReplacements(tc.input, tc.rules)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestCasePreservingReplacer_RealWorldExamples(t *testing.T) {
	// Real-world anonymization examples
	rules := map[string]string{
		"andrey petrov": "fedor ivanov",
		"alice smith":   "bob johnson",
		"microsoft":     "techcorp",
		"san francisco": "big city",
	}

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		// Examples from the user's requirements
		{"andrey case", "andrey petrov", "fedor ivanov"},
		{"Andrey case", "Andrey Petrov", "Fedor Ivanov"},
		{"ANdrey case", "ANdrey Petrov", "Fedor Ivanov"},
		{"ANDREY case", "ANDREY PETROV", "FEDOR IVANOV"},

		{"alice case", "alice smith", "bob johnson"},
		{"Alice case", "Alice Smith", "Bob Johnson"},
		{"ALice case", "ALice Smith", "Bob Johnson"},
		{"ALICE case", "ALICE SMITH", "BOB JOHNSON"},

		{"microsoft case", "microsoft", "techcorp"},
		{"Microsoft case", "Microsoft", "Techcorp"},
		{"MICROSOFT case", "MICROSOFT", "TECHCORP"},
		{"MicroSoft case", "MicroSoft", "Techcorp"},

		{"san francisco case", "san francisco", "big city"},
		{"San Francisco case", "San Francisco", "Big City"},
		{"SAN FRANCISCO case", "SAN FRANCISCO", "BIG CITY"},
		{"SaN FrAnCiScO case", "SaN FrAnCiScO", "Big City"},

		// Complex real-world sentence
		{"complex sentence", "andrey petrov and alice smith work at microsoft in san francisco", "fedor ivanov and bob johnson work at techcorp in big city"},
		{"complex mixed case", "Andrey PETROV and Alice Smith work at Microsoft in San Francisco", "Fedor IVANOV and Bob Johnson work at Techcorp in Big City"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ApplyReplacements(tc.input, rules)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}
