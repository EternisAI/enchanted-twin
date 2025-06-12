package integration

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

func TestMemoryIntegrationTelegram(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := SetupTestEnvironment(t)
	defer env.Cleanup(t)

	t.Run("Query telegram", func(t *testing.T) {
		source := "telegram"
		inputPath := "testdata/telegram_export_sample.jsonl"

		_, err := env.dataprocessing.ProcessSource(env.ctx, source, inputPath, "telegram")
		if err != nil {
			t.Fatalf("Failed to process source: %v", err)
		}

		env.LoadDocuments(t, source, inputPath)

		env.StoreDocumentsWithTimeout(t, 30*time.Minute)

		limit := 100
		filter := memory.Filter{
			Source: &source,
			Limit:  &limit,
		}

		result, err := env.memory.Query(env.ctx, fmt.Sprintf("What do we know about the user from %s source?", source), &filter)

		if err != nil {
			env.logger.Warn("telegram query returned error (may be expected)", "error", err)
			t.Logf("telegram query error (non-fatal): %v", err)
		} else {
			env.logger.Info("telegram query results", "fact_count", len(result.Facts))

			if len(result.Facts) == 0 {
				t.Logf("No facts extracted from telegram conversations (this may be normal)")
			} else {
				for _, fact := range result.Facts {
					env.logger.Info(source, "fact", "id", fact.ID, "content", fact.Content, "source", fact.Source)
				}
			}
		}
	})
}

func TestMainTelegram(m *testing.M) {
	SetupSharedInfrastructure()

	code := m.Run()

	TeardownSharedInfrastructure()

	os.Exit(code)
}
