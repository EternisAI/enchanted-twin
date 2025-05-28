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

type SendMemoryPictureInput struct {
	ChatID             string
	PictureDescription string
}

func (s *FriendService) SendMemoryPicture(ctx context.Context, input SendMemoryPictureInput) error {
	if s.twinchatService == nil {
		return fmt.Errorf("twinchat service not available")
	}

	if s.toolRegistry == nil {
		return fmt.Errorf("tool registry not available")
	}

	tool, exists := s.toolRegistry.Get("generate_image")
	if !exists {
		return fmt.Errorf("generate_image tool not found")
	}
	s.logger.Info("Found generate_image tool", "tool_type", fmt.Sprintf("%T", tool))

	toolResult, err := s.toolRegistry.Execute(ctx, "generate_image", map[string]any{
		"prompt": input.PictureDescription,
	})
	if err != nil {
		return fmt.Errorf("failed to generate image: %w", err)
	}

	imageURLs := toolResult.ImageURLs()
	s.logger.Info("Extracted image URLs", "imageURLs", imageURLs, "count", len(imageURLs))
	if len(imageURLs) == 0 {
		return fmt.Errorf("no image URLs returned from generate_image tool")
	}

	// Send message with the generated image
	s.logger.Info("Sending memory picture with image", "image_urls", imageURLs)

	message := fmt.Sprintf("I was thinking about this memory and created a picture for you: %s", input.PictureDescription)

	// Use the send_to_chat tool to send the message with image URLs
	_, err = s.toolRegistry.Execute(ctx, "send_to_chat", map[string]any{
		"message":    message,
		"chat_id":    input.ChatID,
		"image_urls": []any{imageURLs[0]},
	})
	if err != nil {
		s.logger.Error("Failed to send message with image via send_to_chat tool", "error", err)
		// Fallback to regular message without image
		_, err := s.twinchatService.SendAssistantMessage(ctx, input.ChatID, message)
		if err != nil {
			return fmt.Errorf("failed to send fallback memory picture message: %w", err)
		}
		s.logger.Info("Memory picture message sent (without image)", "chat_id", input.ChatID)
		return nil
	}

	s.logger.Info("Memory picture sent successfully with image", "chat_id", input.ChatID, "image_count", len(imageURLs))
	return nil
}
