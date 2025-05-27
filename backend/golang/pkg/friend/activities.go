// Owner: slimane@eternis.ai

package friend

import (
	"context"
	"fmt"
	"math/rand"
	"strings"

	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/prompts"
)

func (s *FriendService) FetchMemory(ctx context.Context) (string, error) {
	if s.memoryService == nil {
		return "", nil
	}

	result, err := s.memoryService.Query(ctx, "user memories personal experiences")
	if err != nil {
		s.logger.Error("Failed to fetch memories", "error", err)
		return "", err
	}

	if len(result.Documents) == 0 {
		return "No memories found", nil
	}

	var memories []string
	for _, doc := range result.Documents {
		memories = append(memories, doc.Content)
	}

	return strings.Join(memories, "\n"), nil
}

func (s *FriendService) FetchRandomMemory(ctx context.Context) (string, error) {
	if s.memoryService == nil {
		return "", nil
	}

	result, err := s.memoryService.Query(ctx, "user memories personal experiences activities")
	if err != nil {
		s.logger.Error("Failed to fetch random memory", "error", err)
		return "", err
	}

	if len(result.Documents) == 0 {
		return "No memories found", nil
	}

	randomIndex := rand.Intn(len(result.Documents))
	return result.Documents[randomIndex].Content, nil
}

func (s *FriendService) GeneratePokeMessage(ctx context.Context) (string, error) {
	personality, err := s.identityService.GetPersonality(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get personality: %w", err)
	}

	memories, err := s.FetchMemory(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to fetch memories: %w", err)
	}

	prompt, err := prompts.BuildPokeMessagePrompt(prompts.PokeMessagePrompt{
		Identity: personality,
		Memories: memories,
	})
	if err != nil {
		return "", fmt.Errorf("failed to build prompt: %w", err)
	}

	messages := []ai.Message{
		{
			Role:    ai.MessageRoleSystem,
			Content: prompt,
		},
	}

	response, err := s.aiService.CompletionsWithMessages(ctx, messages, nil, "gpt-4o-mini")
	if err != nil {
		return "", fmt.Errorf("failed to generate poke message: %w", err)
	}

	return response.Content, nil
}

func (s *FriendService) SendPokeMessage(ctx context.Context, message string) error {
	if s.twinchatService == nil {
		return fmt.Errorf("twinchat service not available")
	}

	_, err := s.twinchatService.SendAssistantMessage(ctx, "", message)
	if err != nil {
		return fmt.Errorf("failed to send poke message: %w", err)
	}

	s.logger.Info("Poke message sent successfully")
	return nil
}

func (s *FriendService) GenerateMemoryPicture(ctx context.Context) (string, error) {
	personality, err := s.identityService.GetPersonality(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get personality: %w", err)
	}

	randomMemory, err := s.FetchRandomMemory(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to fetch random memory: %w", err)
	}

	prompt, err := prompts.BuildMemoryPicturePrompt(prompts.MemoryPicturePrompt{
		Memory:   randomMemory,
		Identity: personality,
	})
	if err != nil {
		return "", fmt.Errorf("failed to build picture prompt: %w", err)
	}

	messages := []ai.Message{
		{
			Role:    ai.MessageRoleUser,
			Content: prompt,
		},
	}

	response, err := s.aiService.CompletionsWithMessages(ctx, messages, nil, "gpt-4o-mini")
	if err != nil {
		return "", fmt.Errorf("failed to generate picture description: %w", err)
	}

	return response.Content, nil
}

func (s *FriendService) SendMemoryPicture(ctx context.Context, chatID, pictureDescription string) error {
	if s.twinchatService == nil {
		return fmt.Errorf("twinchat service not available")
	}

	message := fmt.Sprintf("I was thinking about this memory and created a picture for you: %s", pictureDescription)

	_, err := s.twinchatService.SendAssistantMessage(ctx, chatID, message)
	if err != nil {
		return fmt.Errorf("failed to send memory picture: %w", err)
	}

	s.logger.Info("Memory picture sent successfully", "chat_id", chatID)
	return nil
}
