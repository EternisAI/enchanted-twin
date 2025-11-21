package ai

import (
	"context"
	"os"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDictionaryMergePrecedence(t *testing.T) {
	t.Run("ExistingDictTakesPrecedenceOverConversationDict", func(t *testing.T) {
		db, tempDB := setupTempDatabase(t)
		defer func() { _ = os.Remove(tempDB) }()
		defer func() { _ = db.Close() }()

		logger := log.New(nil)
		anonymizer := NewPersistentAnonymizer(db, logger)
		defer anonymizer.Shutdown()

		conversationID := "test-merge-conversation"

		// First, save a conversation dictionary: {test1 -> a, test2 -> b}
		conversationDict := map[string]string{
			"test1": "a",
			"test2": "b",
		}
		err := anonymizer.SaveConversationDict(conversationID, conversationDict)
		require.NoError(t, err)

		// Now call with existing dictionary: {test1 -> c, test3 -> d}
		// Expected result: {test1 -> c, test2 -> b, test3 -> d}
		// (test1 -> c should override test1 -> a from conversation dict)
		existingDict := map[string]string{
			"test1": "c", // This should override the conversation dict value
			"test3": "d", // This should be added
		}

		// Use a simple message that won't trigger any anonymization
		messages := []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("Hello world"),
		}

		_, resultDict, _, err := anonymizer.AnonymizeMessages(
			context.Background(), conversationID, messages, existingDict, make(chan struct{}))
		require.NoError(t, err)

		// Verify the merge precedence
		expectedDict := map[string]string{
			"test1": "c", // From existingDict (takes precedence)
			"test2": "b", // From conversationDict
			"test3": "d", // From existingDict
		}

		assert.Equal(t, expectedDict["test1"], resultDict["test1"], "test1 should have value from existingDict")
		assert.Equal(t, expectedDict["test2"], resultDict["test2"], "test2 should have value from conversationDict")
		assert.Equal(t, expectedDict["test3"], resultDict["test3"], "test3 should have value from existingDict")

		t.Logf("Conversation dict: %v", conversationDict)
		t.Logf("Existing dict: %v", existingDict)
		t.Logf("Result dict: %v", resultDict)
	})

	t.Run("MemoryOnlyModeAlsoRespectsPrecedence", func(t *testing.T) {
		db, tempDB := setupTempDatabase(t)
		defer func() { _ = os.Remove(tempDB) }()
		defer func() { _ = db.Close() }()

		logger := log.New(nil)
		anonymizer := NewPersistentAnonymizer(db, logger)
		defer anonymizer.Shutdown()

		// Test memory-only mode with dictionary merge
		existingDict := map[string]string{
			"test1": "c",
			"test3": "d",
		}

		messages := []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("Hello world"),
		}

		// Memory-only mode (empty conversationID)
		_, resultDict, _, err := anonymizer.AnonymizeMessages(
			context.Background(), "", messages, existingDict, make(chan struct{}))
		require.NoError(t, err)

		// In memory-only mode, only existingDict should be used
		expectedDict := map[string]string{
			"test1": "c",
			"test3": "d",
		}

		assert.Equal(t, expectedDict["test1"], resultDict["test1"], "test1 should have value from existingDict")
		assert.Equal(t, expectedDict["test3"], resultDict["test3"], "test3 should have value from existingDict")
		assert.NotContains(t, resultDict, "test2", "test2 should not be present in memory-only mode")

		t.Logf("Existing dict: %v", existingDict)
		t.Logf("Result dict: %v", resultDict)
	})
}
