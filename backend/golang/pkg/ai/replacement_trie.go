package ai

import "strings"

// TrieNode represents a node in the replacement trie.
type TrieNode struct {
	children    map[rune]*TrieNode
	isEndOfWord bool
	replacement string
	original    string
}

// ReplacementTrie implements efficient string replacement with longest match first.
type ReplacementTrie struct {
	root *TrieNode
}

// NewReplacementTrie creates a new trie for string replacements.
func NewReplacementTrie() *ReplacementTrie {
	return &ReplacementTrie{
		root: &TrieNode{
			children: make(map[rune]*TrieNode),
		},
	}
}

// Insert adds a pattern and its replacement to the trie.
func (t *ReplacementTrie) Insert(pattern, replacement string) {
	// Validate input pattern - empty patterns can cause matching issues
	if pattern == "" {
		return // Early return for empty patterns
	}

	// Validate trie root
	if t.root == nil {
		return // Early return if trie is not properly initialized
	}

	current := t.root

	// Convert pattern to lowercase for case-insensitive matching
	lowerPattern := strings.ToLower(pattern)

	for _, r := range lowerPattern {
		if _, exists := current.children[r]; !exists {
			current.children[r] = &TrieNode{
				children: make(map[rune]*TrieNode),
			}
		}
		current = current.children[r]
	}

	current.isEndOfWord = true
	current.replacement = replacement
	current.original = pattern // Store original case for reference
}

// ReplaceAll performs all replacements in the text with longest match first behavior.
func (t *ReplacementTrie) ReplaceAll(text string) (string, map[string]string) {
	if t.root == nil {
		return text, make(map[string]string)
	}

	runes := []rune(text)
	result := make([]rune, 0, len(runes))
	rules := make(map[string]string)
	i := 0

	for i < len(runes) {
		// Try to find longest match starting at position i
		match, matchLen := t.findLongestMatch(runes, i)

		if match != nil {
			// Found a match, apply case-preserving replacement
			originalText := string(runes[i : i+matchLen])
			casePreservedReplacement := t.applyCasePreservation(originalText, match.replacement)
			result = append(result, []rune(casePreservedReplacement)...)
			rules[casePreservedReplacement] = match.original
			i += matchLen
		} else {
			// No match, keep original character
			result = append(result, runes[i])
			i++
		}
	}

	return string(result), rules
}

// applyCasePreservation applies case pattern from source to target.
func (t *ReplacementTrie) applyCasePreservation(source, target string) string {
	if len(source) == 0 || len(target) == 0 {
		return target
	}

	sourceRunes := []rune(source)
	targetRunes := []rune(target)

	// Check if source is all uppercase
	allUpper := true
	for _, r := range sourceRunes {
		if r >= 'a' && r <= 'z' {
			allUpper = false
			break
		}
	}
	if allUpper {
		return strings.ToUpper(target)
	}

	// Check if source is all lowercase
	allLower := true
	for _, r := range sourceRunes {
		if r >= 'A' && r <= 'Z' {
			allLower = false
			break
		}
	}
	if allLower {
		return strings.ToLower(target)
	}

	// Mixed case - capitalize first letter only
	if len(targetRunes) == 0 {
		return target
	}

	result := make([]rune, len(targetRunes))
	result[0] = []rune(strings.ToUpper(string(targetRunes[0])))[0]
	for i := 1; i < len(targetRunes); i++ {
		result[i] = []rune(strings.ToLower(string(targetRunes[i])))[0]
	}

	return string(result)
}

// findLongestMatch finds the longest pattern match starting at the given position.
func (t *ReplacementTrie) findLongestMatch(runes []rune, startPos int) (*TrieNode, int) {
	current := t.root
	var longestMatch *TrieNode
	longestMatchLen := 0

	for i := startPos; i < len(runes); i++ {
		r := runes[i]
		// Convert character to lowercase for case-insensitive matching
		lowerR := []rune(strings.ToLower(string(r)))[0]
		if child, exists := current.children[lowerR]; exists {
			current = child
			if current.isEndOfWord {
				longestMatch = current
				longestMatchLen = i - startPos + 1
			}
		} else {
			break
		}
	}

	return longestMatch, longestMatchLen
}

// Size returns the number of patterns in the trie.
func (t *ReplacementTrie) Size() int {
	return t.countNodes(t.root)
}

func (t *ReplacementTrie) countNodes(node *TrieNode) int {
	if node == nil {
		return 0
	}

	count := 0
	if node.isEndOfWord {
		count = 1
	}

	for _, child := range node.children {
		count += t.countNodes(child)
	}

	return count
}
