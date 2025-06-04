package holon

import (
	"context"
	"time"

	"github.com/EternisAI/enchanted-twin/graph/model"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
)

type Service struct {
	// Add dependencies here later (database, external APIs, etc.)
}

func NewService() *Service {
	return &Service{}
}

func (s *Service) GetHolons(ctx context.Context, userID string) ([]string, error) {
	// TODO: Implement actual holon retrieval based on user membership
	// For now, return mock data
	holons := []string{
		"ai-research-holon",
		"blockchain-dev-holon",
		"creative-writers-holon",
		"quantum-computing-holon",
	}
	return holons, nil
}

func (s *Service) GetThreads(ctx context.Context, network *string) ([]*model.Thread, error) {
	// TODO: Implement actual thread retrieval from holon network
	// For now, return mock data
	threads := []*model.Thread{
		{
			ID:      "thread-1",
			Title:   "Welcome to the Holon Network",
			Content: "This is an introduction thread to help you get started with holonic collaboration.",
			ImageURLs: []string{
				"https://example.com/holon-welcome.jpg",
			},
			Author: &model.Author{
				Identity: "holon-system",
				Alias:    helpers.Ptr("Holon Network"),
			},
			CreatedAt: time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
			ExpiresAt: helpers.Ptr(time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339)),
			Messages: []*model.ThreadMessage{
				{
					ID: "msg-welcome-1",
					Author: &model.Author{
						Identity: "user-alice",
						Alias:    helpers.Ptr("Alice Johnson"),
					},
					Content:     "Excited to be part of this holonic experiment!",
					CreatedAt:   time.Now().Add(-12 * time.Hour).Format(time.RFC3339),
					IsDelivered: helpers.Ptr(true),
					Actions:     []string{"like", "reply", "share"},
				},
			},
			Actions: []string{"like", "reply", "share", "bookmark"},
			Views:   156,
		},
		{
			ID:        "thread-2",
			Title:     "Collaborative AI Research Discussion",
			Content:   "Let's discuss the latest developments in collaborative AI and how we can apply them to our holon.",
			ImageURLs: []string{},
			Author: &model.Author{
				Identity: "user-researcher",
				Alias:    helpers.Ptr("Dr. Research"),
			},
			CreatedAt: time.Now().Add(-6 * time.Hour).Format(time.RFC3339),
			ExpiresAt: nil,
			Messages: []*model.ThreadMessage{
				{
					ID: "msg-research-1",
					Author: &model.Author{
						Identity: "user-bob",
						Alias:    helpers.Ptr("Bob Smith"),
					},
					Content:     "I've been working on distributed learning algorithms that could benefit our collective intelligence.",
					CreatedAt:   time.Now().Add(-3 * time.Hour).Format(time.RFC3339),
					IsDelivered: helpers.Ptr(true),
					Actions:     []string{"like", "reply"},
				},
			},
			Actions: []string{"like", "reply", "bookmark", "collaborate"},
			Views:   89,
		},
	}

	return threads, nil
}

func (s *Service) GetThread(ctx context.Context, network *string, threadID string) (*model.Thread, error) {
	// TODO: Implement actual thread retrieval by ID
	// For now, return mock data based on thread ID
	thread := &model.Thread{
		ID:      threadID,
		Title:   "Dynamic Thread: " + threadID,
		Content: "This thread demonstrates dynamic content generation based on the requested thread ID.",
		ImageURLs: []string{
			"https://example.com/thread-" + threadID + ".jpg",
		},
		Author: &model.Author{
			Identity: "user-creator",
			Alias:    helpers.Ptr("Thread Creator"),
		},
		CreatedAt: time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
		ExpiresAt: helpers.Ptr(time.Now().Add(7 * 24 * time.Hour).Format(time.RFC3339)),
		Messages: []*model.ThreadMessage{
			{
				ID: "msg-" + threadID + "-1",
				Author: &model.Author{
					Identity: "user-commenter",
					Alias:    helpers.Ptr("Community Member"),
				},
				Content:     "This is a great thread! Looking forward to more discussions.",
				CreatedAt:   time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
				IsDelivered: helpers.Ptr(true),
				Actions:     []string{"like", "reply"},
			},
		},
		Actions: []string{"like", "reply", "share", "bookmark", "collaborate"},
		Views:   234,
	}

	return thread, nil
}

func (s *Service) JoinHolon(ctx context.Context, network *string, userID string) (bool, error) {
	// TODO: Implement actual holon joining logic
	// This would involve:
	// 1. Validating the holon network exists
	// 2. Checking user permissions/requirements
	// 3. Adding user to holon membership
	// 4. Triggering any onboarding workflows

	return true, nil
}

func (s *Service) SendToHolon(ctx context.Context, threadPreviewID string, network *string) (*model.Thread, error) {
	// TODO: Implement actual thread sending to holon

	publishedThread := &model.Thread{
		ID:        "published-" + threadPreviewID,
		Title:     "Published Thread",
		Content:   "This thread has been successfully published to the holon network.",
		ImageURLs: []string{},
		Author: &model.Author{
			Identity: "current-user",
			Alias:    helpers.Ptr("You"),
		},
		CreatedAt: time.Now().Format(time.RFC3339),
		ExpiresAt: nil,
		Messages:  []*model.ThreadMessage{},
		Actions:   []string{"like", "reply", "share"},
		Views:     1, // Just published
	}

	return publishedThread, nil
}

func extractTitleFromContent(content string) string {
	// Simple implementation - take first line or first 50 characters
	if len(content) == 0 {
		return "Untitled Thread"
	}

	// Find first newline
	for i, char := range content {
		if char == '\n' {
			if i > 0 {
				return content[:i]
			}
			break
		}
	}

	// If no newline found, take first 50 characters
	if len(content) > 50 {
		return content[:47] + "..."
	}

	return content
}
