package integration

import (
	"fmt"
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

		time.Sleep(5 * time.Second)

		env.logger.Info("🟡 Querying memories")
		result, err := env.memory.Query(env.ctx, fmt.Sprintf("What do you we know about user from %s source?", source), &filter)
		require.NoError(t, err)
		assert.NotEmpty(t, result.Facts, "should find memories from %s source", source)

		for _, fact := range result.Facts {
			env.logger.Info(source, "fact", "id", fact.ID, "content", fact.Content, "source", fact.Source)
		}
	})
}
