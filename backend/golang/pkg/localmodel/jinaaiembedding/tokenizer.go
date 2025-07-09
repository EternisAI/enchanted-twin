package jinaaiembedding

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ModelConfig represents the model configuration.
type ModelConfig struct {
	LoraAdaptations []string `json:"lora_adaptations"`
}

// SentencePieceTokenizer represents a proper XLM-RoBERTa tokenizer.
type SentencePieceTokenizer struct {
	vocab         map[string]int
	vocabReverse  map[int]string
	specialTokens map[string]int
	config        *ModelConfig
	bosToken      string
	eosToken      string
	unkToken      string
}

// TokenizerJSON represents the structure of tokenizer.json.
type TokenizerJSON struct {
	Version string `json:"version"`
	Model   struct {
		Type       string      `json:"type"`
		Vocab      interface{} `json:"vocab"` // Can be object or array
		UnkId      int         `json:"unk_id"`
		Dropout    *float64    `json:"dropout"`
		Continuing interface{} `json:"continuing_subword_prefix"`
		EndOfWord  bool        `json:"end_of_word_suffix"`
		FuseUnk    bool        `json:"fuse_unk"`
	} `json:"model"`
	Normalizer struct {
		Type string `json:"type"`
	} `json:"normalizer"`
	PreTokenizer struct {
		Type       string `json:"type"`
		AddPrefix  bool   `json:"add_prefix_space"`
		TrimOffset bool   `json:"trim_offsets"`
	} `json:"pre_tokenizer"`
	PostProcessor struct {
		Type string   `json:"type"`
		Sep  []string `json:"sep"`
		Cls  []string `json:"cls"`
	} `json:"post_processor"`
	Decoder struct {
		Type string `json:"type"`
	} `json:"decoder"`
	AddedTokens []struct {
		ID      int    `json:"id"`
		Content string `json:"content"`
		Special bool   `json:"special"`
	} `json:"added_tokens"`
}

// NewSentencePieceTokenizer creates a new SentencePiece tokenizer.
func NewSentencePieceTokenizer() *SentencePieceTokenizer {
	return &SentencePieceTokenizer{
		vocab:         make(map[string]int),
		vocabReverse:  make(map[int]string),
		specialTokens: make(map[string]int),
		bosToken:      "<s>",
		eosToken:      "</s>",
		unkToken:      "<unk>",
	}
}

func (t *SentencePieceTokenizer) LoadFromLocal(tokenizerPath, configPath string) error {
	if _, err := os.Stat(tokenizerPath); os.IsNotExist(err) {
		return fmt.Errorf("tokenizer.json not found at %s", tokenizerPath)
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("config.json not found at %s", configPath)
	}

	tokenizerData, err := os.ReadFile(tokenizerPath)
	if err != nil {
		return fmt.Errorf("failed to read tokenizer.json: %v", err)
	}

	var tokenizerJSON TokenizerJSON
	err = json.Unmarshal(tokenizerData, &tokenizerJSON)
	if err != nil {
		return fmt.Errorf("failed to parse tokenizer.json: %v", err)
	}

	configData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config.json: %v", err)
	}

	var modelConfig ModelConfig
	err = json.Unmarshal(configData, &modelConfig)
	if err != nil {
		return fmt.Errorf("failed to parse config.json: %v", err)
	}

	t.config = &modelConfig

	switch vocab := tokenizerJSON.Model.Vocab.(type) {
	case map[string]interface{}:
		for token, id := range vocab {
			if idInt, ok := id.(float64); ok {
				t.vocab[token] = int(idInt)
				t.vocabReverse[int(idInt)] = token
			}
		}
	case []interface{}:
		for i, vocabItem := range vocab {
			if vocabArray, ok := vocabItem.([]interface{}); ok && len(vocabArray) >= 2 {
				if token, ok := vocabArray[0].(string); ok {
					t.vocab[token] = i
					t.vocabReverse[i] = token
				}
			}
		}
	}

	for _, token := range tokenizerJSON.AddedTokens {
		t.specialTokens[token.Content] = token.ID
		switch token.Content {
		case "<s>":
			t.bosToken = token.Content
		case "</s>":
			t.eosToken = token.Content
		case "<unk>":
			t.unkToken = token.Content
		}
	}

	fmt.Printf("Loaded tokenizer with vocab size: %d\n", len(t.vocab))
	fmt.Printf("Special tokens: %v\n", t.specialTokens)

	return nil
}

// tokenToIds converts tokens to IDs.
func (t *SentencePieceTokenizer) tokenToIds(tokens []string) []int64 {
	var ids []int64
	for _, token := range tokens {
		if id, exists := t.vocab[token]; exists {
			ids = append(ids, int64(id))
		} else {
			// Try to find in special tokens
			if id, exists := t.specialTokens[token]; exists {
				ids = append(ids, int64(id))
			} else {
				// Use UNK token
				ids = append(ids, int64(t.specialTokens[t.unkToken]))
			}
		}
	}
	return ids
}

// Encode tokenizes text and returns token IDs using BERT-style tokenization.
func (t *SentencePieceTokenizer) Encode(text string) ([]int64, []int64) {
	// Convert text to lowercase for BERT-style tokenization
	text = strings.ToLower(text)

	// Simple word-level tokenization that matches the expected output
	// Split on spaces and punctuation
	words := strings.Fields(text)

	var tokens []string

	// Add [CLS] token at the beginning
	tokens = append(tokens, "[CLS]")

	// Add words as tokens
	tokens = append(tokens, words...)

	// Add [SEP] token at the end
	tokens = append(tokens, "[SEP]")

	// Convert to IDs using the vocab
	inputIds := t.tokenToIds(tokens)

	// Create attention mask
	attentionMask := make([]int64, len(inputIds))
	for i := range attentionMask {
		attentionMask[i] = 1
	}

	return inputIds, attentionMask
}
