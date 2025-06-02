package contacts

import (
	"context"

	"github.com/EternisAI/enchanted-twin/graph/model"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/openai/openai-go"
)

type Storage interface {
	AddContact(ctx context.Context, contact model.Contact) (string, error)
	UpdateInsight(ctx context.Context, contactID string, insight string) error
	GetContact(ctx context.Context, contactID string) (*model.Contact, error)
	ListContacts(ctx context.Context) ([]*model.Contact, error)
}

type Service struct {
	storage            Storage
	memoryStorage      memory.Storage
	completionsService *ai.Service
	model              string
}

func NewService(storage Storage, memoryStorage memory.Storage, completionsService *ai.Service, model string) *Service {
	return &Service{
		storage:            storage,
		memoryStorage:      memoryStorage,
		completionsService: completionsService,
		model:              model,
	}
}

func (s *Service) GenerateInsight(ctx context.Context, contactName string) (string, error) {
	filter := memory.Filter{
		ContactName: &contactName,
	}
	queryResult, err := s.memoryStorage.Query(ctx, "conversations with "+contactName, &filter)
	if err != nil {
		return "", err
	}

	var conversationContent string
	for _, doc := range queryResult.Documents {
		conversationContent += doc.Content() + "\n"
	}

	if conversationContent == "" {
		return "", nil
	}

	systemPrompt := `You are an AI assistant that generates insights about people based on conversation history. 
Analyze the provided conversations and generate a concise, helpful insight about this person including:
- Their interests and preferences
- Communication style
- Key topics they discuss
- Any notable patterns or characteristics
Keep the insight professional and factual.`

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
		openai.UserMessage(conversationContent),
	}

	response, err := s.completionsService.Completions(ctx, messages, nil, s.model)
	if err != nil {
		return "", err
	}

	return response.Content, nil
}

func (s *Service) AddContact(ctx context.Context, contactName string, tags []string) (*model.Contact, error) {
	contact := model.Contact{
		ContactName: contactName,
		Tags:        tags,
	}

	insight, err := s.GenerateInsight(ctx, contactName)
	if err != nil {
		insight = "Failed to generate insight: " + err.Error()
	}
	contact.Insight = &insight

	contactID, err := s.storage.AddContact(ctx, contact)
	if err != nil {
		return nil, err
	}

	contact.ID = contactID
	return &contact, nil
}

func (s *Service) UpdateContactInsight(ctx context.Context, contactID string) error {
	contact, err := s.storage.GetContact(ctx, contactID)
	if err != nil {
		return err
	}

	insight, err := s.GenerateInsight(ctx, contact.ContactName)
	if err != nil {
		return err
	}

	return s.storage.UpdateInsight(ctx, contactID, insight)
}
