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
		CanSearchWeb:  true,
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

	// Test IsVoice branch
	promptVoice, err := BuildTwinChatSystemPrompt(TwinChatSystemPrompt{
		UserName:      &userName,
		Bio:           &bio,
		ChatID:        &chatID,
		CurrentTime:   timeStr,
		EmailAccounts: emails,
		IsVoice:       true,
		CanSearchWeb:  true,
	})
	if err != nil {
		t.Fatalf("unexpected error (IsVoice): %v", err)
	}
	if !strings.Contains(promptVoice, "You keep responses tight, usually under three sentences, cuz impact beats length every time.") {
		t.Errorf("expected IsVoice prompt to contain voice-specific instructions")
	}

	// Test web search capability
	promptWithWebSearch, err := BuildTwinChatSystemPrompt(TwinChatSystemPrompt{
		UserName:     &userName,
		CurrentTime:  timeStr,
		CanSearchWeb: true,
	})
	if err != nil {
		t.Fatalf("unexpected error (CanSearchWeb=true): %v", err)
	}
	if !strings.Contains(promptWithWebSearch, "When you are asked to search the web or about news or very recent events, you should use the `perplexity_ask` tool if it exists.") {
		t.Errorf("expected prompt with web search to contain perplexity_ask instruction")
	}

	promptWithoutWebSearch, err := BuildTwinChatSystemPrompt(TwinChatSystemPrompt{
		UserName:     &userName,
		CurrentTime:  timeStr,
		CanSearchWeb: false,
	})
	if err != nil {
		t.Fatalf("unexpected error (CanSearchWeb=false): %v", err)
	}
	if !strings.Contains(promptWithoutWebSearch, "You cannot search the web at this time. If the user asks you to search for information online or latest news, tell them they need to connect a search tool in their settings to enable web search functionality.") {
		t.Errorf("expected prompt without web search to contain settings instruction")
	}
}
