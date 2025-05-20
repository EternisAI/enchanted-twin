package prompts

import (
	"strings"
	"testing"
	"time"
)

func TestBuildScheduledTaskSystemPrompt(t *testing.T) {
	userName := "Bob"
	bio := "A diligent productivity enthusiast."
	chatID := "chat-5678"
	emails := []string{"bob@example.com", "bob@work.com"}
	timeStr := time.Date(2024, 6, 2, 15, 30, 0, 0, time.UTC).Format(time.RFC3339)
	prevResult := "Last result was successful."

	prompt, err := BuildScheduledTaskSystemPrompt(ScheduledTaskSystemPrompt{
		UserName:       &userName,
		Bio:            &bio,
		ChatID:         &chatID,
		CurrentTime:    timeStr,
		EmailAccounts:  emails,
		PreviousResult: &prevResult,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("Generated prompt:\n%s", prompt)

	if !strings.Contains(prompt, userName) {
		t.Errorf("expected prompt to contain user name")
	}
	if !strings.Contains(prompt, bio) {
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
	if !strings.Contains(prompt, prevResult) {
		t.Errorf("expected prompt to contain previous result")
	}
}
