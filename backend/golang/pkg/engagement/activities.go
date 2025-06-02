// Owner: slimane@eternis.ai

package engagement

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/prompts"
)

const (
	SimilarityThreshold = 0.15
	FriendMetadataType  = "friend"
)

func (s *FriendService) StoreSentMessage(ctx context.Context, message string, activityType string) error {
	if s.memoryService == nil {
		s.logger.Warn("Memory service not available, skipping message storage")
		return nil
	}

	now := time.Now()
	doc := memory.TextDocument{
		FieldSource:    "friend",
		FieldContent:   message,
		FieldTimestamp: &now,
		FieldTags:      []string{"sent_message", activityType},
		FieldMetadata: map[string]string{
			"type":          FriendMetadataType,
			"activity_type": activityType,
			"sent_at":       now.Format(time.RFC3339),
		},
	}

	err := s.memoryService.Store(ctx, memory.TextDocumentsToDocuments([]memory.TextDocument{doc}), func(processed, total int) {
		// Progress callback - no action needed
	})
	if err != nil {
		s.logger.Error("Failed to store sent message", "error", err, "message", message)
		return fmt.Errorf("failed to store sent message: %w", err)
	}

	s.logger.Info("Stored sent message in memory", "activity_type", activityType, "message_length", len(message))
	return nil
}

func (s *FriendService) CheckForSimilarFriendMessages(ctx context.Context, message string) (bool, error) {
	if s.memoryService == nil {
		s.logger.Warn("Memory service not available, skipping similarity check")
		return false, nil
	}

	s.logger.Info("Checking for similarity with previous friend messages", "message", message)

	// Create a filtered query to only search friend messages
	result, err := s.memoryService.QueryWithDistance(ctx, message, map[string]string{
		"type": FriendMetadataType,
	})
	if err != nil {
		s.logger.Error("Failed to query for similar friend messages", "error", err)
		return false, fmt.Errorf("failed to query for similar friend messages: %w", err)
	}

	s.logger.Debug("Query with distance result (friend messages only)", "total_documents", len(result.Documents))

	for _, docWithDistance := range result.Documents {
		s.logger.Debug("Checking friend document",
			"distance", docWithDistance.Distance,
			"threshold", SimilarityThreshold,
			"activity_type", docWithDistance.Document.FieldMetadata["activity_type"],
			"content_preview", docWithDistance.Document.FieldContent[:min(50, len(docWithDistance.Document.FieldContent))])

		if docWithDistance.Distance < SimilarityThreshold {
			s.logger.Info("Found similar friend message, skipping send",
				"distance", docWithDistance.Distance,
				"threshold", SimilarityThreshold,
				"similar_message", docWithDistance.Document.FieldContent[:min(100, len(docWithDistance.Document.FieldContent))])
			return true, nil
		}
	}

	s.logger.Info("No similar friend messages found, safe to send",
		"total_friend_documents_checked", len(result.Documents))
	return false, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *FriendService) FetchMemory(ctx context.Context) (string, error) {
	if s.memoryService == nil {
		return "", nil
	}

	result, err := s.memoryService.Query(ctx, "what do you know about me")
	if err != nil {
		s.logger.Error("Failed to fetch memories", "error", err)
		return "", err
	}

	if len(result.Documents) == 0 {
		return "No memories found", nil
	}

	randomIndex := rand.Intn(len(result.Documents))
	return result.Documents[randomIndex].FieldContent, nil
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
	return result.Documents[randomIndex].FieldContent, nil
}

func (s *FriendService) FetchIdentity(ctx context.Context) (string, error) {
	personality, err := s.identityService.GetPersonality(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get personality: %w", err)
	}
	return personality, nil
}

type GeneratePokeMessageInput struct {
	Identity string `json:"identity"`
	Memories string `json:"memories"`
}

func (s *FriendService) GeneratePokeMessage(ctx context.Context, input GeneratePokeMessageInput) (string, error) {
	prompt, err := prompts.BuildPokeMessagePrompt(prompts.PokeMessagePrompt{
		Identity: input.Identity,
		Memories: input.Memories,
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
	isSimilar, err := s.CheckForSimilarFriendMessages(ctx, message)
	if err != nil {
		s.logger.Error("Failed to check for similar messages", "error", err)
	}

	if isSimilar {
		s.logger.Info("Skipping poke message due to similarity with previous messages")
		return nil
	}

	if s.twinchatService == nil {
		return fmt.Errorf("twinchat service not available")
	}

	_, err = s.twinchatService.SendAssistantMessage(ctx, "", message)
	if err != nil {
		return fmt.Errorf("failed to send poke message: %w", err)
	}

	err = s.StoreSentMessage(ctx, message, "poke_message")
	if err != nil {
		s.logger.Error("Failed to store sent poke message", "error", err)
	}

	s.logger.Info("Poke message sent successfully")
	return nil
}

type GenerateMemoryPictureInput struct {
	Identity     string `json:"identity"`
	RandomMemory string `json:"random_memory"`
}

func (s *FriendService) GenerateMemoryPicture(ctx context.Context, input GenerateMemoryPictureInput) (string, error) {
	prompt, err := prompts.BuildMemoryPicturePrompt(prompts.MemoryPicturePrompt{
		Memory:   input.RandomMemory,
		Identity: input.Identity,
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
	messageOptions := []string{
		"This memory made me think of something visual, so I made this:",
		"Your memory sparked this image in my head:",
		"I couldn't stop thinking about this memory, so I drew it:",
		"This memory was too good not to visualize:",
		"Had to turn this memory into something you could see:",
	}

	messageIndex := len(input.PictureDescription) % len(messageOptions)
	message := messageOptions[messageIndex]

	isSimilar, err := s.CheckForSimilarFriendMessages(ctx, message)
	if err != nil {
		s.logger.Error("Failed to check for similar messages", "error", err)
	}

	if isSimilar {
		s.logger.Info("Skipping memory picture due to similarity with previous messages")
		return nil
	}

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

	s.logger.Info("Sending memory picture with image", "image_urls", imageURLs)

	_, err = s.toolRegistry.Execute(ctx, "send_to_chat", map[string]any{
		"message":    message,
		"chat_id":    input.ChatID,
		"image_urls": []any{imageURLs[0]},
	})
	if err != nil {
		s.logger.Error("Failed to send message with image via send_to_chat tool", "error", err)
		_, err := s.twinchatService.SendAssistantMessage(ctx, input.ChatID, message)
		if err != nil {
			return fmt.Errorf("failed to send fallback memory picture message: %w", err)
		}
		s.logger.Info("Memory picture message sent (without image)", "chat_id", input.ChatID)
	} else {
		s.logger.Info("Memory picture sent successfully with image", "chat_id", input.ChatID, "image_count", len(imageURLs))
	}

	err = s.StoreSentMessage(ctx, message, "memory_picture")
	if err != nil {
		s.logger.Error("Failed to store sent memory picture message", "error", err)
	}

	return nil
}

type TrackUserResponseInput struct {
	ChatID       string    `json:"chat_id"`
	ActivityType string    `json:"activity_type"`
	Timestamp    time.Time `json:"timestamp"`
}

type GenerateRandomWaitInput struct {
	MinSeconds int `json:"min_seconds"`
	MaxSeconds int `json:"max_seconds"`
}

type GenerateRandomWaitOutput struct {
	WaitDurationSeconds int `json:"wait_duration_seconds"`
}

type SelectRandomActivityInput struct {
	AvailableActivities []string       `json:"available_activities"`
	ActivityWeights     map[string]int `json:"activity_weights,omitempty"`
}

type SelectRandomActivityOutput struct {
	SelectedActivity string `json:"selected_activity"`
}

func (s *FriendService) TrackUserResponse(ctx context.Context, input TrackUserResponseInput) error {
	s.logger.Info("Tracking user response",
		"chat_id", input.ChatID,
		"activity_type", input.ActivityType,
		"timestamp", input.Timestamp)

	if s.store != nil {
		err := s.store.StoreFriendActivity(ctx, input.ChatID, input.ActivityType, input.Timestamp)
		if err != nil {
			s.logger.Error("Failed to store friend activity", "error", err)
			return fmt.Errorf("failed to store friend activity: %w", err)
		}
		s.logger.Info("Friend activity stored successfully", "chat_id", input.ChatID)
	} else {
		s.logger.Warn("Store not available, skipping friend activity storage")
	}

	return nil
}

func (s *FriendService) GenerateRandomWait(ctx context.Context, input GenerateRandomWaitInput) (GenerateRandomWaitOutput, error) {
	waitDuration := rand.Intn(input.MaxSeconds-input.MinSeconds+1) + input.MinSeconds
	s.logger.Info("Generated random wait duration", "duration_seconds", waitDuration)

	return GenerateRandomWaitOutput{
		WaitDurationSeconds: waitDuration,
	}, nil
}

func (s *FriendService) SelectRandomActivity(ctx context.Context, input SelectRandomActivityInput) (SelectRandomActivityOutput, error) {
	if len(input.AvailableActivities) == 0 {
		return SelectRandomActivityOutput{}, fmt.Errorf("no activities available for selection")
	}

	// If no weights provided, use equal weights
	if len(input.ActivityWeights) == 0 {
		selectedIndex := rand.Intn(len(input.AvailableActivities))
		selectedActivity := input.AvailableActivities[selectedIndex]

		s.logger.Info("Selected random activity (equal weights)", "activity", selectedActivity, "from_options", input.AvailableActivities)
		return SelectRandomActivityOutput{
			SelectedActivity: selectedActivity,
		}, nil
	}

	// Build weighted pool
	var weightedPool []string
	for _, activity := range input.AvailableActivities {
		weight := input.ActivityWeights[activity]
		if weight <= 0 {
			weight = 1 // Default weight if not specified or invalid
		}

		// Add activity to pool 'weight' number of times
		for i := 0; i < weight; i++ {
			weightedPool = append(weightedPool, activity)
		}
	}

	if len(weightedPool) == 0 {
		return SelectRandomActivityOutput{}, fmt.Errorf("weighted pool is empty")
	}

	selectedIndex := rand.Intn(len(weightedPool))
	selectedActivity := weightedPool[selectedIndex]

	s.logger.Info("Selected weighted random activity",
		"activity", selectedActivity,
		"weights", input.ActivityWeights,
		"pool_size", len(weightedPool),
		"from_options", input.AvailableActivities)

	return SelectRandomActivityOutput{
		SelectedActivity: selectedActivity,
	}, nil
}

type SendQuestionInput struct {
	ChatID string `json:"chat_id"`
}

func (s *FriendService) GetRandomQuestion(ctx context.Context) (string, error) {
	if len(QuestionTable) == 0 {
		return "", fmt.Errorf("no questions available in question table")
	}

	randomIndex := rand.Intn(len(QuestionTable))
	question := QuestionTable[randomIndex]

	s.logger.Info("Selected random question", "question", question, "index", randomIndex)
	return question, nil
}

func (s *FriendService) SendQuestion(ctx context.Context, input SendQuestionInput) error {
	question, err := s.GetRandomQuestion(ctx)
	if err != nil {
		return fmt.Errorf("failed to get random question: %w", err)
	}

	isSimilar, err := s.CheckForSimilarFriendMessages(ctx, question)
	if err != nil {
		s.logger.Error("Failed to check for similar messages", "error", err)
	}

	if isSimilar {
		s.logger.Info("Skipping question due to similarity with previous messages")
		return nil
	}

	if s.twinchatService == nil {
		return fmt.Errorf("twinchat service not available")
	}

	_, err = s.twinchatService.SendAssistantMessage(ctx, input.ChatID, question)
	if err != nil {
		return fmt.Errorf("failed to send question: %w", err)
	}

	if s.memoryService != nil {
		metaData := map[string]string{
			"type":          FriendMetadataType,
			"activity_type": "question",
		}
		doc := memory.TextDocument{
			FieldSource:   "friend",
			FieldContent:  question,
			FieldMetadata: metaData,
			FieldTags:     []string{"friend", "question"},
		}
		docs := []memory.TextDocument{doc}
		if errStore := s.memoryService.Store(ctx, memory.TextDocumentsToDocuments(docs), func(processed, total int) {
		}); errStore != nil {
			s.logger.Error("Failed to store question in memory", "error", errStore)
		}
	}

	s.logger.Info("Successfully sent question", "question", question)
	return nil
}
