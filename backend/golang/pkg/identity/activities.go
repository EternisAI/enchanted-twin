package identity

import (
	"context"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"
	"go.temporal.io/sdk/worker"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/prompts"
)

func NewIdentityActivities(logger *log.Logger, memory memory.Storage, ai *ai.Service, completionsModel string) *identityActivities {
	return &identityActivities{
		logger:           logger,
		memory:           memory,
		ai:               ai,
		completionsModel: completionsModel,
	}
}

type identityActivities struct {
	logger           *log.Logger
	memory           memory.Storage
	ai               *ai.Service
	completionsModel string
}

func (a *identityActivities) GenerateUserProfileActivity(ctx context.Context) (string, error) {
	personalityPrompts := []string{
		"My personality",
		"What do I want to do lately",
		"What are my job and hobbies",
		"Who are my friends and main relationships",
		"I am interested in",
		"I am uncomfortable with",
	}
	memoryDocuments := []string{}
	limit := 30
	for _, prompt := range personalityPrompts {
		filter := memory.Filter{
			Limit: &limit,
		}
		docs, err := a.memory.Query(ctx, prompt, &filter)
		if err != nil {
			return "", err
		}
		for _, fact := range docs.Facts {
			memoryDocuments = append(memoryDocuments, fact.GenerateContentForLLM())
		}
	}

	a.logger.Info("Memory documents", "memory_documents", len(memoryDocuments))

	systemPrompt, err := prompts.BuildIdentityPersonalitySystemPrompt()
	if err != nil {
		return "", err
	}

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
		openai.UserMessage(strings.Join(memoryDocuments, "\n")),
	}

	result, err := a.ai.Completions(ctx, messages, []openai.ChatCompletionToolParam{}, a.completionsModel, ai.Background)
	if err != nil {
		return "", err
	}

	cleanedContent := ai.StripThinkingTags(result.Message.Content)

	return cleanedContent, nil
}

func (a *identityActivities) RegisterWorkflowsAndActivities(worker worker.Worker) {
	worker.RegisterWorkflow(DerivePersonalityWorkflow)
	worker.RegisterActivity(a.GenerateUserProfileActivity)
}
