package personalities

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// ThreadEvaluationHandler handles thread-based scenario evaluation.
type ThreadEvaluationHandler struct {
	framework *PersonalityTestFramework
}

// NewThreadEvaluationHandler creates a new thread evaluation handler.
func NewThreadEvaluationHandler(framework *PersonalityTestFramework) *ThreadEvaluationHandler {
	return &ThreadEvaluationHandler{
		framework: framework,
	}
}

// GetSupportedType returns the scenario type this handler supports.
func (the *ThreadEvaluationHandler) GetSupportedType() ScenarioType {
	return ScenarioTypeThread
}

// Evaluate evaluates a thread scenario.
func (the *ThreadEvaluationHandler) Evaluate(ctx context.Context, content ScenarioContent, personality *ReferencePersonality, env *TestEnvironment) (*GenericEvaluationResult, error) {
	// Cast content to ThreadContent
	threadContent, ok := content.(*ThreadContent)
	if !ok {
		return nil, fmt.Errorf("invalid content type for thread handler")
	}

	// Get the main text content
	mainText := threadContent.GetMainText()
	title := threadContent.GetDisplayTitle()

	// For thread evaluation, we'd typically use the thread processor
	// For now, return a mock result based on basic content analysis
	shouldShow := true
	confidence := 0.8
	reason := fmt.Sprintf("Thread evaluation for personality %s", personality.Name)

	// Basic filtering logic
	if len(mainText) < 20 {
		shouldShow = false
		confidence = 0.9
		reason = "Thread content too short to be meaningful"
	}

	// Check for tech-related content for tech entrepreneurs
	if personality.Name == "tech_entrepreneur" && (contains(mainText, "AI") || contains(title, "technology")) {
		shouldShow = true
		confidence = 0.95
		reason = "Tech entrepreneur is highly interested in technology content"
	}

	newState := "visible"
	if !shouldShow {
		newState = "hidden"
	}

	return &GenericEvaluationResult{
		ShouldShow: shouldShow,
		Reason:     reason,
		Confidence: confidence,
		NewState:   newState,
		Metadata: map[string]interface{}{
			"handler":        "thread",
			"title":          title,
			"content_length": len(mainText),
			"personality":    personality.Name,
		},
	}, nil
}

// ChatMessageEvaluationHandler handles chat message scenario evaluation.
type ChatMessageEvaluationHandler struct {
	framework *PersonalityTestFramework
}

// NewChatMessageEvaluationHandler creates a new chat message evaluation handler.
func NewChatMessageEvaluationHandler(framework *PersonalityTestFramework) *ChatMessageEvaluationHandler {
	return &ChatMessageEvaluationHandler{
		framework: framework,
	}
}

// GetSupportedType returns the scenario type this handler supports.
func (cmeh *ChatMessageEvaluationHandler) GetSupportedType() ScenarioType {
	return ScenarioTypeChatMessage
}

// Evaluate evaluates a chat message scenario.
func (cmeh *ChatMessageEvaluationHandler) Evaluate(ctx context.Context, content ScenarioContent, personality *ReferencePersonality, env *TestEnvironment) (*GenericEvaluationResult, error) {
	// Cast content to ChatMessageContent
	chatContent, ok := content.(*ChatMessageContent)
	if !ok {
		return nil, fmt.Errorf("invalid content type for chat message handler")
	}

	// Get the main text content
	messageText := chatContent.GetText()
	chatContext := chatContent.ChatContext
	if chatContext == "" {
		chatContext = "unknown"
	}

	// Simple evaluation logic based on content
	shouldShow := true
	confidence := 0.8
	reason := fmt.Sprintf("Chat message evaluation for personality %s", personality.Name)

	// Basic filtering logic - this would be more sophisticated in practice
	if len(messageText) < 10 {
		shouldShow = false
		confidence = 0.9
		reason = "Message too short to be meaningful"
	}

	// Check for AI-related content for tech entrepreneurs
	if personality.Name == "tech_entrepreneur" && contains(messageText, "AI") {
		shouldShow = true
		confidence = 0.95
		reason = "Tech entrepreneur is highly interested in AI content"
	}

	newState := "visible"
	if !shouldShow {
		newState = "hidden"
	}

	return &GenericEvaluationResult{
		ShouldShow: shouldShow,
		Reason:     reason,
		Confidence: confidence,
		NewState:   newState,
		Metadata: map[string]interface{}{
			"handler":        "chat_message",
			"content_length": len(messageText),
			"personality":    personality.Name,
			"chat_context":   chatContext,
			"message_type":   "chat",
		},
	}, nil
}

// EmailEvaluationHandler handles email scenario evaluation.
type EmailEvaluationHandler struct {
	framework *PersonalityTestFramework
}

// NewEmailEvaluationHandler creates a new email evaluation handler.
func NewEmailEvaluationHandler(framework *PersonalityTestFramework) *EmailEvaluationHandler {
	return &EmailEvaluationHandler{
		framework: framework,
	}
}

// GetSupportedType returns the scenario type this handler supports.
func (eeh *EmailEvaluationHandler) GetSupportedType() ScenarioType {
	return ScenarioTypeEmail
}

// Evaluate evaluates an email scenario.
func (eeh *EmailEvaluationHandler) Evaluate(ctx context.Context, content ScenarioContent, personality *ReferencePersonality, env *TestEnvironment) (*GenericEvaluationResult, error) {
	// Cast content to EmailContent
	emailContent, ok := content.(*EmailContent)
	if !ok {
		return nil, fmt.Errorf("invalid content type for email handler")
	}

	subject := emailContent.Subject
	body := emailContent.Body
	priority := emailContent.Priority
	if priority == "" {
		priority = "normal"
	}

	// Simple evaluation logic based on subject and body
	shouldShow := true
	confidence := 0.8
	reason := fmt.Sprintf("Email evaluation for personality %s", personality.Name)

	// Basic spam detection
	spamIndicators := []string{"MEGA SALE", "70% OFF", "LIMITED TIME", "DON'T MISS OUT"}
	for _, indicator := range spamIndicators {
		if contains(subject, indicator) || contains(body, indicator) {
			shouldShow = false
			confidence = 0.9
			reason = "Email appears to be spam"
			break
		}
	}

	// Check for investment-related content for tech entrepreneurs
	if personality.Name == "tech_entrepreneur" && (contains(subject, "investment") || contains(body, "funding") || contains(body, "startup")) {
		shouldShow = true
		confidence = 0.95
		reason = "Tech entrepreneur is highly interested in investment opportunities"
	}

	newState := "visible"
	if !shouldShow {
		newState = "hidden"
	}

	return &GenericEvaluationResult{
		ShouldShow: shouldShow,
		Reason:     reason,
		Confidence: confidence,
		NewState:   newState,
		Metadata: map[string]interface{}{
			"handler":        "email",
			"subject":        subject,
			"priority":       priority,
			"subject_length": len(subject),
			"body_length":    len(body),
			"personality":    personality.Name,
		},
	}, nil
}

// SocialPostEvaluationHandler handles social media post scenario evaluation.
type SocialPostEvaluationHandler struct {
	framework *PersonalityTestFramework
}

// NewSocialPostEvaluationHandler creates a new social post evaluation handler.
func NewSocialPostEvaluationHandler(framework *PersonalityTestFramework) *SocialPostEvaluationHandler {
	return &SocialPostEvaluationHandler{
		framework: framework,
	}
}

// GetSupportedType returns the scenario type this handler supports.
func (speh *SocialPostEvaluationHandler) GetSupportedType() ScenarioType {
	return ScenarioTypeSocialPost
}

// Evaluate evaluates a social media post scenario.
func (speh *SocialPostEvaluationHandler) Evaluate(ctx context.Context, content ScenarioContent, personality *ReferencePersonality, env *TestEnvironment) (*GenericEvaluationResult, error) {
	// Cast content to SocialPostContent
	socialContent, ok := content.(*SocialPostContent)
	if !ok {
		return nil, fmt.Errorf("invalid content type for social post handler")
	}

	postText := socialContent.Text
	platform := socialContent.Platform
	if platform == "" {
		platform = "unknown"
	}

	// Extract engagement metrics
	engagement := map[string]interface{}{
		"likes":    socialContent.Likes,
		"comments": socialContent.Comments,
		"shares":   socialContent.Shares,
	}

	// Simple evaluation logic based on content and platform
	shouldShow := true
	confidence := 0.8
	reason := fmt.Sprintf("Social post evaluation for personality %s on %s", personality.Name, platform)

	// Check for tech-related keywords
	techKeywords := []string{"AI", "quantum", "breakthrough", "technology", "innovation", "startup", "funding"}
	isTechRelated := false
	for _, keyword := range techKeywords {
		if contains(postText, keyword) {
			isTechRelated = true
			break
		}
	}

	// Personality-based filtering (simplified)
	if personality.Name == "tech_entrepreneur" && isTechRelated {
		shouldShow = true
		confidence = 0.95
		reason = "Tech entrepreneur is highly interested in technology content"
	} else if personality.Name == "creative_artist" && !isTechRelated {
		shouldShow = true
		confidence = 0.8
		reason = "Creative artist may be interested in non-tech content"
	}

	newState := "visible"
	if !shouldShow {
		newState = "hidden"
	}

	return &GenericEvaluationResult{
		ShouldShow: shouldShow,
		Reason:     reason,
		Confidence: confidence,
		NewState:   newState,
		Metadata: map[string]interface{}{
			"handler":        "social_post",
			"platform":       platform,
			"engagement":     engagement,
			"content_length": len(postText),
			"tech_related":   isTechRelated,
			"personality":    personality.Name,
		},
	}, nil
}

// Helper function to check if a string contains a substring (Unicode-aware case-insensitive).
func contains(text, substr string) bool {
	// Create a Unicode-aware lowercase caser for English
	caser := cases.Lower(language.English)

	// Use proper Unicode case folding and efficient string search
	return strings.Contains(caser.String(text), caser.String(substr))
}

// EvaluationHandlerRegistry manages evaluation handlers for different scenario types.
type EvaluationHandlerRegistry struct {
	handlers map[ScenarioType]EvaluationHandler
}

// NewEvaluationHandlerRegistry creates a new handler registry.
func NewEvaluationHandlerRegistry() *EvaluationHandlerRegistry {
	return &EvaluationHandlerRegistry{
		handlers: make(map[ScenarioType]EvaluationHandler),
	}
}

// Register adds a handler for a specific scenario type.
func (ehr *EvaluationHandlerRegistry) Register(handler EvaluationHandler) {
	ehr.handlers[handler.GetSupportedType()] = handler
}

// GetHandler retrieves a handler for a specific scenario type.
func (ehr *EvaluationHandlerRegistry) GetHandler(scenarioType ScenarioType) (EvaluationHandler, bool) {
	handler, exists := ehr.handlers[scenarioType]
	return handler, exists
}

// ListHandlers returns all registered scenario types.
func (ehr *EvaluationHandlerRegistry) ListHandlers() []ScenarioType {
	types := make([]ScenarioType, 0, len(ehr.handlers))
	for t := range ehr.handlers {
		types = append(types, t)
	}
	return types
}
