package ai

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

	for _, r := range pattern {
		if _, exists := current.children[r]; !exists {
			current.children[r] = &TrieNode{
				children: make(map[rune]*TrieNode),
			}
		}
		current = current.children[r]
	}

	current.isEndOfWord = true
	current.replacement = replacement
	current.original = pattern
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
			// Found a match, replace it
			result = append(result, []rune(match.replacement)...)
			rules[match.replacement] = match.original
			i += matchLen
		} else {
			// No match, keep original character
			result = append(result, runes[i])
			i++
		}
	}

	return string(result), rules
}

// findLongestMatch finds the longest pattern match starting at the given position.
func (t *ReplacementTrie) findLongestMatch(runes []rune, startPos int) (*TrieNode, int) {
	current := t.root
	var longestMatch *TrieNode
	longestMatchLen := 0

	for i := startPos; i < len(runes); i++ {
		r := runes[i]
		if child, exists := current.children[r]; exists {
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
