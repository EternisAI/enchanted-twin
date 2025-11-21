package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashBasedMessageSkippingConsistency(t *testing.T) {
	t.Run("SkippedMessagesUseCorrectAnonymizationRules", func(t *testing.T) {
		db, tempDB := setupTempDatabase(t)
		defer func() { _ = os.Remove(tempDB) }()
		defer func() { _ = db.Close() }()

		conversationID := "test-hash-consistency"
		logger := log.New(nil)

		// === FIRST BATCH: Process initial messages ===
		anonymizer1 := NewPersistentAnonymizer(db, logger)
		defer anonymizer1.Shutdown()

		messages1 := []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("Hello John, how are you?"),
			openai.AssistantMessage("Hi! I'm doing well at OpenAI."),
		}

		anonymizedMessages1, dict1, newRules1, err := anonymizer1.AnonymizeMessages(
			context.Background(), conversationID, messages1, nil, make(chan struct{}))
		require.NoError(t, err)

		// Verify first batch anonymization
		assert.Contains(t, dict1, "PERSON_001")
		assert.Contains(t, dict1, "COMPANY_001")
		assert.Equal(t, "John", dict1["PERSON_001"])
		assert.Equal(t, "OpenAI", dict1["COMPANY_001"])

		// Extract anonymized content for comparison
		msg1Content := extractMessageContent(anonymizedMessages1[0])
		msg2Content := extractMessageContent(anonymizedMessages1[1])

		t.Logf("First batch - Message 1: %s", msg1Content)
		t.Logf("First batch - Message 2: %s", msg2Content)
		t.Logf("First batch - Dictionary: %v", dict1)
		t.Logf("First batch - New rules: %v", newRules1)

		anonymizer1.Shutdown()

		// === SECOND BATCH: Re-process same messages + new message ===
		anonymizer2 := NewPersistentAnonymizer(db, logger)
		defer anonymizer2.Shutdown()

		// Same messages + one new message
		messages2 := []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("Hello John, how are you?"),                        // Same as message 1 (should be skipped by hash)
			openai.AssistantMessage("Hi! I'm doing well at OpenAI."),              // Same as message 2 (should be skipped by hash)
			openai.UserMessage("John, please introduce me to Alice from OpenAI."), // New message
		}

		anonymizedMessages2, dict2, newRules2, err := anonymizer2.AnonymizeMessages(
			context.Background(), conversationID, messages2, nil, make(chan struct{}))
		require.NoError(t, err)

		// Extract anonymized content for comparison
		msg1Content2 := extractMessageContent(anonymizedMessages2[0])
		msg2Content2 := extractMessageContent(anonymizedMessages2[1])
		msg3Content2 := extractMessageContent(anonymizedMessages2[2])

		t.Logf("Second batch - Message 1: %s", msg1Content2)
		t.Logf("Second batch - Message 2: %s", msg2Content2)
		t.Logf("Second batch - Message 3: %s", msg3Content2)
		t.Logf("Second batch - Dictionary: %v", dict2)
		t.Logf("Second batch - New rules: %v", newRules2)

		// === CRITICAL TEST: Verify hash-skipped messages have same anonymization ===
		assert.Equal(t, msg1Content, msg1Content2, "First message should have identical anonymization across batches")
		assert.Equal(t, msg2Content, msg2Content2, "Second message should have identical anonymization across batches")

		// Verify dictionary consistency
		assert.Equal(t, dict1["PERSON_001"], dict2["PERSON_001"], "John should have same token")
		assert.Equal(t, dict1["COMPANY_001"], dict2["COMPANY_001"], "OpenAI should have same token")

		// Verify new entity was processed
		assert.Contains(t, dict2, "PERSON_003")
		assert.Equal(t, "Alice", dict2["PERSON_003"])

		// Verify only new rules were returned in second batch
		assert.NotContains(t, newRules2, "PERSON_001", "John was not newly anonymized in second batch")
		assert.NotContains(t, newRules2, "COMPANY_001", "OpenAI was not newly anonymized in second batch")
		assert.Contains(t, newRules2, "PERSON_003", "Alice was newly anonymized in second batch")

		// Verify third message contains correct anonymization
		assert.Contains(t, msg3Content2, "PERSON_001", "Third message should use John's existing token")
		assert.Contains(t, msg3Content2, "PERSON_003", "Third message should use Alice's new token")
		assert.Contains(t, msg3Content2, "COMPANY_001", "Third message should use OpenAI's existing token")
	})

	t.Run("MultipleHashSkippingCycles", func(t *testing.T) {
		db, tempDB := setupTempDatabase(t)
		defer func() { _ = os.Remove(tempDB) }()
		defer func() { _ = db.Close() }()

		conversationID := "test-multi-hash-consistency"
		logger := log.New(nil)

		// Define a set of messages that will be reused
		baseMessages := []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("Hello John from OpenAI"),
			openai.AssistantMessage("Hi Alice from Microsoft"),
		}

		var previousAnonymizedMessages []openai.ChatCompletionMessageParamUnion
		var previousDict map[string]string

		// Run 3 cycles, each time adding one new message but keeping the base messages
		for cycle := 0; cycle < 3; cycle++ {
			t.Logf("=== CYCLE %d ===", cycle+1)

			anonymizer := NewPersistentAnonymizer(db, logger)

			// Build message batch: base messages + new message for this cycle
			messages := make([]openai.ChatCompletionMessageParamUnion, len(baseMessages))
			copy(messages, baseMessages)
			messages = append(messages, openai.UserMessage(fmt.Sprintf("Cycle %d with Bob", cycle+1)))

			anonymizedMessages, dict, newRules, err := anonymizer.AnonymizeMessages(
				context.Background(), conversationID, messages, nil, make(chan struct{}))
			require.NoError(t, err)

			t.Logf("Cycle %d - Dictionary: %v", cycle+1, dict)
			t.Logf("Cycle %d - New rules: %v", cycle+1, newRules)

			if cycle > 0 {
				// Verify base messages have identical anonymization across cycles
				for i := 0; i < len(baseMessages); i++ {
					prevContent := extractMessageContent(previousAnonymizedMessages[i])
					currContent := extractMessageContent(anonymizedMessages[i])
					assert.Equal(t, prevContent, currContent,
						"Base message %d should have identical anonymization in cycle %d", i, cycle+1)
				}

				// Verify dictionary consistency for previous entities
				for token, entity := range previousDict {
					assert.Equal(t, entity, dict[token],
						"Token %s should map to same entity across cycles", token)
				}
			}

			previousAnonymizedMessages = anonymizedMessages
			previousDict = dict
			anonymizer.Shutdown()
		}
	})
}

func extractMessageContent(message openai.ChatCompletionMessageParamUnion) string {
	// Helper function to extract content from anonymized message using JSON marshal/unmarshal
	messageBytes, err := json.Marshal(message)
	if err != nil {
		return ""
	}

	var messageMap map[string]interface{}
	if err := json.Unmarshal(messageBytes, &messageMap); err != nil {
		return ""
	}

	if content, exists := messageMap["content"]; exists {
		if contentStr, ok := content.(string); ok {
			return contentStr
		}
	}
	return ""
}
