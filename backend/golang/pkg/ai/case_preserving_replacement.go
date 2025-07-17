package ai

import (
	"sort"
	"strings"
	"unicode"
)

// sortKeysByLengthDesc sorts map keys by descending length.
func sortKeysByLengthDesc(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return len(keys[i]) > len(keys[j])
	})
	return keys
}

// CasePreservingReplacer handles case-preserving string replacement.
type CasePreservingReplacer struct {
	rules map[string]string
}

// NewCasePreservingReplacer creates a new case-preserving replacer.
func NewCasePreservingReplacer(rules map[string]string) *CasePreservingReplacer {
	return &CasePreservingReplacer{
		rules: rules,
	}
}

// Replace performs case-preserving replacement on the input text.
func (r *CasePreservingReplacer) Replace(text string) string {
	result := text

	// Sort rules by length (longest first) to avoid partial matches
	sortedRules := sortKeysByLengthDesc(r.rules)

	// Apply replacements with case preservation
	for _, original := range sortedRules {
		replacement := r.rules[original]
		result = r.replaceWithDynamicCasePreservation(result, original, replacement)
	}

	return result
}

// replaceWithDynamicCasePreservation performs case-preserving replacement by analyzing the input text.
func (r *CasePreservingReplacer) replaceWithDynamicCasePreservation(text, original, replacement string) string {
	result := text
	originalLower := strings.ToLower(original)
	replacementLower := strings.ToLower(replacement)

	// Find all occurrences of the original word (case-insensitive) from the beginning
	searchStart := 0
	for {
		// Find next occurrence (case-insensitive)
		lowerText := strings.ToLower(result)
		idx := strings.Index(lowerText[searchStart:], originalLower)
		if idx == -1 {
			break
		}

		// Adjust index to absolute position
		idx += searchStart

		// Check if this is a word boundary (simplified)
		if !isWordBoundaryAt(result, idx, len(originalLower)) {
			// Not a word boundary, skip this occurrence and continue searching
			searchStart = idx + 1
			continue
		}

		// Extract the actual case pattern from the text
		foundText := result[idx : idx+len(originalLower)]

		// Apply the case pattern to the replacement
		casePreservedReplacement := r.applyCasePattern(foundText, replacementLower)

		// Replace in the result
		result = result[:idx] + casePreservedReplacement + result[idx+len(originalLower):]

		// Update search start position to continue after this replacement
		searchStart = idx + len(casePreservedReplacement)
	}

	return result
}

// isWordBoundaryAt checks if the position represents a word boundary.
func isWordBoundaryAt(text string, idx, length int) bool {
	// Check character before
	if idx > 0 {
		prevChar := text[idx-1]
		if (prevChar >= 'a' && prevChar <= 'z') || (prevChar >= 'A' && prevChar <= 'Z') {
			return false
		}
	}

	// Check character after
	if idx+length < len(text) {
		nextChar := text[idx+length]
		if (nextChar >= 'a' && nextChar <= 'z') || (nextChar >= 'A' && nextChar <= 'Z') {
			return false
		}
	}

	return true
}

// applyCasePattern applies the case pattern from source to target based on the rules:
// 1. If source word is all caps → result will be all caps
// 2. If source word is all lower → result will be all lower
// 3. In other cases → capitalize only first letter in result word.
func (r *CasePreservingReplacer) applyCasePattern(source, target string) string {
	if len(source) == 0 || len(target) == 0 {
		return target
	}

	// Handle compound words (contain spaces) by processing each word separately
	if strings.Contains(source, " ") && strings.Contains(target, " ") {
		sourceWords := strings.Fields(source)
		targetWords := strings.Fields(target)

		// If word counts don't match, fall back to simple rules
		if len(sourceWords) != len(targetWords) {
			return r.applyCasePatternToSingleWord(source, target)
		}

		result := make([]string, len(targetWords))
		for i := 0; i < len(sourceWords) && i < len(targetWords); i++ {
			result[i] = r.applyCasePatternToSingleWord(sourceWords[i], targetWords[i])
		}

		return strings.Join(result, " ")
	}

	// Handle single words
	return r.applyCasePatternToSingleWord(source, target)
}

// applyCasePatternToSingleWord applies case rules to a single word.
func (r *CasePreservingReplacer) applyCasePatternToSingleWord(source, target string) string {
	if len(source) == 0 || len(target) == 0 {
		return target
	}

	// Check if source is all uppercase
	if r.isAllUppercase(source) {
		return strings.ToUpper(target)
	}

	// Check if source is all lowercase
	if r.isAllLowercase(source) {
		return strings.ToLower(target)
	}

	// In other cases, capitalize only first letter
	if len(target) == 0 {
		return target
	}

	return strings.ToUpper(target[:1]) + strings.ToLower(target[1:])
}

// isAllUppercase checks if all letters in the string are uppercase.
func (r *CasePreservingReplacer) isAllUppercase(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) && !unicode.IsUpper(r) {
			return false
		}
	}
	return true
}

// isAllLowercase checks if all letters in the string are lowercase.
func (r *CasePreservingReplacer) isAllLowercase(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) && !unicode.IsLower(r) {
			return false
		}
	}
	return true
}

// ApplyReplacements applies case-preserving replacements to text.
func ApplyReplacements(text string, rules map[string]string) string {
	if len(rules) == 0 {
		return text
	}

	replacer := NewCasePreservingReplacer(rules)
	return replacer.Replace(text)
}

// ApplyDeAnonymization applies de-anonymization by simply replacing tokens with original values.
func ApplyDeAnonymization(text string, rules map[string]string) string {
	if len(rules) == 0 {
		return text
	}

	result := text

	// Sort rules by token length (longest first) to avoid partial matches
	sortedRules := sortKeysByLengthDesc(rules)

	// Apply simple string replacement (restore original case)
	for _, token := range sortedRules {
		original := rules[token]
		result = strings.ReplaceAll(result, token, original)
	}

	return result
}

// ApplyAnonymization applies anonymization replacements while preserving token case.
func ApplyAnonymization(text string, rules map[string]string) string {
	if len(rules) == 0 {
		return text
	}

	result := text

	// Sort rules by length (longest first) to avoid partial matches
	sortedRules := sortKeysByLengthDesc(rules)

	// Apply replacements (case-insensitive matching but keep token case unchanged)
	for _, original := range sortedRules {
		token := rules[original]
		result = replaceAllCaseInsensitive(result, original, token)
	}

	return result
}

// replaceAllCaseInsensitive performs case-insensitive replacement.
func replaceAllCaseInsensitive(text, original, replacement string) string {
	result := text
	originalLower := strings.ToLower(original)

	// Find all occurrences of the original word (case-insensitive)
	searchStart := 0
	for {
		// Find next occurrence (case-insensitive)
		lowerText := strings.ToLower(result)
		idx := strings.Index(lowerText[searchStart:], originalLower)
		if idx == -1 {
			break
		}

		// Adjust index to absolute position
		idx += searchStart

		// Check if this is a word boundary
		if !isWordBoundaryAt(result, idx, len(originalLower)) {
			// Not a word boundary, skip this occurrence and continue searching
			searchStart = idx + 1
			continue
		}

		// Replace in the result
		result = result[:idx] + replacement + result[idx+len(originalLower):]

		// Update search start position to continue after this replacement
		searchStart = idx + len(replacement)
	}

	return result
}
