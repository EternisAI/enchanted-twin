package prompts

import (
	"strings"
	"testing"
	"time"
)

func TestBuildTwinChatSystemPrompt(t *testing.T) {
	userName := "Alice"
	bio := "A curious AI enthusiast."
	chatID := "chat-1234"
	emails := []string{"alice@example.com", "alice@work.com"}
	timeStr := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)

	prompt, err := BuildTwinChatSystemPrompt(TwinChatSystemPrompt{
		UserName:      &userName,
		Bio:           &bio,
		ChatID:        &chatID,
		CurrentTime:   timeStr,
		EmailAccounts: emails,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("Generated prompt:\n%s", prompt)

	if !strings.Contains(prompt, "Alice") {
		t.Errorf("expected prompt to contain user name")
	}
	if !strings.Contains(prompt, "A curious AI enthusiast.") {
		t.Errorf("expected prompt to contain bio")
	}
	if !strings.Contains(prompt, chatID) {
		t.Errorf("expected prompt to contain chat ID")
	}
	for _, email := range emails {
		if !strings.Contains(prompt, email) {
			t.Errorf("expected prompt to contain email: %s", email)
		}
	}
	if !strings.Contains(prompt, timeStr) {
		t.Errorf("expected prompt to contain current time")
	}
}
