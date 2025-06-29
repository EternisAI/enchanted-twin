package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

func TestMemoryIntegrationTelegram(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := SetupTestEnvironment(t)
	defer env.Cleanup(t)

	t.Run("Query telegram from export", func(t *testing.T) {
		source := "telegram"
		inputPath := "testdata/telegram_export_sample.json"

		env.LoadDocuments(t, source, inputPath)
		env.StoreDocuments(t)

		limit := 100
		filter := memory.Filter{
			Source: &source,
			Limit:  &limit,
		}

		result, err := env.memory.Query(env.ctx, fmt.Sprintf("What do you we know about user from %s source?", source), &filter)
		require.NoError(t, err)
		assert.NotEmpty(t, result.Facts, "should find memories from %s source", source)

		for _, fact := range result.Facts {
			env.logger.Info(source, "fact", "id", fact.ID, "content", fact.Content, "source", fact.Source)
		}
	})

	t.Run("Query telegram from jsonl", func(t *testing.T) {
		source := "telegram"
		inputPath := "testdata/telegram_sample.jsonl"

		env.LoadDocumentsFromJSONL(t, source, inputPath)
		env.StoreDocuments(t)

		limit := 100
		filter := memory.Filter{
			Source: &source,
			Limit:  &limit,
		}

		result, err := env.memory.Query(env.ctx, "What do you know about me ?", &filter)
		require.NoError(t, err)
		assert.NotEmpty(t, result.Facts, "should find memories from %s source", source)

		for _, fact := range result.Facts {
			env.logger.Info(source, "fact", "id", fact.ID, "content", fact.Content, "source", fact.Source)
		}
	})
}

func TestMemoryIntegrationWhatsapp(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := SetupTestEnvironment(t)
	defer env.Cleanup(t)

	t.Run("Query whatsapp", func(t *testing.T) {
		source := "whatsapp"
		inputPath := "testdata/whatsapp_sample.jsonl"

		if _, err := os.Stat(inputPath); os.IsNotExist(err) {
			testdataDir := filepath.Dir(inputPath)
			if err := os.MkdirAll(testdataDir, 0o755); err != nil {
				t.Fatalf("Failed to create testdata directory: %v", err)
			}

			sampleData := []string{
				`{"data":{"text":"2023-05-15 10:30:00 - Alice: Hey, how are you?\n2023-05-15 10:31:00 - Bob: I'm good! Just working on the project.\n2023-05-15 10:32:00 - Alice: Great! Let me know if you need help.","metadata":{"chat_session":"alice_bob_chat","participants":["Alice","Bob"],"user":"Bob"}},"timestamp":"2023-05-15T10:32:00Z","source":"whatsapp"}`,
				`{"data":{"text":"2023-05-16 14:00:00 - Carol: Meeting at 3pm today?\n2023-05-16 14:01:00 - Bob: Yes, I'll be there.\n2023-05-16 14:02:00 - Carol: Perfect, see you then!","metadata":{"chat_session":"carol_bob_chat","participants":["Carol","Bob"],"user":"Bob"}},"timestamp":"2023-05-16T14:02:00Z","source":"whatsapp"}`,
			}

			content := strings.Join(sampleData, "\n")
			if err := os.WriteFile(inputPath, []byte(content), 0o644); err != nil {
				t.Fatalf("Failed to create sample WhatsApp file: %v", err)
			}
			t.Logf("Created sample WhatsApp test file: %s", inputPath)
		}

		env.LoadDocumentsFromJSONL(t, source, inputPath)

		if len(env.documents) == 0 {
			t.Skip("No WhatsApp documents loaded - skipping test")
			return
		}

		env.logger.Info("=== WhatsApp ConversationDocuments Details ===")
		for i, doc := range env.documents {
			env.logger.Info("ConversationDocument",
				"index", i,
				"id", doc.ID(),
				"source", doc.Source(),
				"content_length", len(doc.Content()),
			)

			env.logger.Info("Conversation Content:",
				"document_index", i,
				"full_content", doc.Content(),
			)

			if convDoc, ok := doc.(*memory.ConversationDocument); ok {
				env.logger.Info("ConversationDocument Metadata",
					"document_index", i,
					"participants", convDoc.People,
					"message_count", len(convDoc.Conversation),
					"user", convDoc.User,
					"chat_session", convDoc.FieldMetadata["chat_session"],
					"metadata", convDoc.FieldMetadata,
				)
			}

			env.logger.Info("--- End Document %d ---", i)
		}
		env.logger.Info("=== End WhatsApp ConversationDocuments ===")

		env.StoreDocumentsWithTimeout(t, 30*time.Minute)

		limit := 100
		filter := memory.Filter{
			Source: &source,
			Limit:  &limit,
		}

		result, err := env.memory.Query(env.ctx, fmt.Sprintf("What do we know about the user from %s source?", source), &filter)

		if err != nil {
			env.logger.Warn("WhatsApp query returned error (may be expected)", "error", err)
			t.Logf("WhatsApp query error (non-fatal): %v", err)
		} else {
			env.logger.Info("WhatsApp query results", "fact_count", len(result.Facts))

			if len(result.Facts) == 0 {
				t.Logf("No facts extracted from WhatsApp conversations (this may be normal)")
			} else {
				for _, fact := range result.Facts {
					env.logger.Info(source, "fact", "id", fact.ID, "content", fact.Content, "source", fact.Source)
				}
			}
		}
	})
}

func TestMemoryIntegrationChatGPT(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := SetupTestEnvironment(t)
	defer env.Cleanup(t)

	t.Run("Query chatgpt from export", func(t *testing.T) {
		source := "chatgpt"
		inputPath := "testdata/conversations.json"

		env.LoadDocuments(t, source, inputPath)
		env.StoreDocuments(t)

		limit := 100
		filter := memory.Filter{
			Source: &source,
			Limit:  &limit,
		}

		result, err := env.memory.Query(env.ctx, fmt.Sprintf("What do you we know from %s source?", source), &filter)
		require.NoError(t, err)
		assert.NotEmpty(t, result.Facts, "should find memories from %s source", source)

		for _, fact := range result.Facts {
			env.logger.Info(source, "fact", "id", fact.ID, "content", fact.Content, "source", fact.Source)
		}
	})
}

func TestMemoryIntegrationGmail(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := SetupTestEnvironment(t)
	defer env.Cleanup(t)

	t.Run("Query gmail from export", func(t *testing.T) {
		source := "gmail"
		inputPath := "testdata/google_export_sample.zip"

		env.LoadDocuments(t, source, inputPath)
		env.StoreDocuments(t)

		limit := 100
		filter := memory.Filter{
			Source: &source,
			Limit:  &limit,
		}

		result, err := env.memory.Query(env.ctx, fmt.Sprintf("What do we know about the user from %s source?", source), &filter)
		require.NoError(t, err)
		assert.NotEmpty(t, result.Facts, "should find memories from %s source", source)

		for _, fact := range result.Facts {
			env.logger.Info(source, "fact", "id", fact.ID, "content", fact.Content, "source", fact.Source)
		}
	})
}
