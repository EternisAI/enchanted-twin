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

func (a *identityActivities) GeneratePersonalityActivity(ctx context.Context) (string, error) {
	personalityPrompts := []string{
		"My personality",
		"What makes me happy",
		"What makes me angry",
		"I am interested in",
		"Things that excite me",
		"I am uncomfortable with",
	}
	memoryDocuments := []string{}
	for _, prompt := range personalityPrompts {
		docs, err := a.memory.Query(ctx, prompt)
		if err != nil {
			return "", err
		}
		for _, doc := range docs.Documents {
			memoryDocuments = append(memoryDocuments, doc.Content())
		}
	}

	systemPrompt, err := prompts.BuildIdentityPersonalitySystemPrompt()
	if err != nil {
		return "", err
	}

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
		openai.UserMessage(strings.Join(memoryDocuments, "\n")),
	}

	response, err := a.ai.Completions(ctx, messages, []openai.ChatCompletionToolParam{}, a.completionsModel)
	if err != nil {
		return "", err
	}

	return response.Content, nil
}

func (a *identityActivities) RegisterWorkflowsAndActivities(worker worker.Worker) {
	worker.RegisterWorkflow(DerivePersonalityWorkflow)
	worker.RegisterActivity(a.GeneratePersonalityActivity)
}
