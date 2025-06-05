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
		"What do I want to do lately",
		"What are my job and hobbies",
		"Who are my friends and main relationships",
		"I am interested in",
		"I am uncomfortable with",
	}
	memoryDocuments := []string{}
	for _, prompt := range personalityPrompts {
		docs, err := a.memory.Query(ctx, prompt, nil)
		if err != nil {
			return "", err
		}
		for _, doc := range docs.Documents {
			memoryDocuments = append(memoryDocuments, doc.Content())
		}
	}

	a.logger.Info("Memory documents", "memory_documents", memoryDocuments)

	systemPrompt, err := prompts.BuildIdentityPersonalitySystemPrompt()
	if err != nil {
		return "", err
	}

	a.logger.Info("System prompt", "system_prompt", systemPrompt)

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
		openai.UserMessage(strings.Join(memoryDocuments, "\n")),
	}

	response, err := a.ai.Completions(ctx, messages, []openai.ChatCompletionToolParam{}, a.completionsModel)
	if err != nil {
		return "", err
	}

	a.logger.Info("Response_content", "response_content", response.Content)

	return response.Content, nil
}

func (a *identityActivities) RegisterWorkflowsAndActivities(worker worker.Worker) {
	worker.RegisterWorkflow(DerivePersonalityWorkflow)
	worker.RegisterActivity(a.GeneratePersonalityActivity)
}
