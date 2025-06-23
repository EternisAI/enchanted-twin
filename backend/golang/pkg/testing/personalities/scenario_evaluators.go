package personalities

import (
	"context"
	"fmt"
	"strings"
)

// ThreadEvaluationHandler handles thread-based scenario evaluation
type ThreadEvaluationHandler struct {
	framework *PersonalityTestFramework
}

// NewThreadEvaluationHandler creates a new thread evaluation handler
func NewThreadEvaluationHandler(framework *PersonalityTestFramework) *ThreadEvaluationHandler {
	return &ThreadEvaluationHandler{
		framework: framework,
	}
}

// GetType returns the scenario type this handler supports
func (teh *ThreadEvaluationHandler) GetType() ScenarioType {
	return ScenarioTypeThread
}

// Evaluate evaluates a thread scenario
func (teh *ThreadEvaluationHandler) Evaluate(ctx context.Context, scenario GenericTestScenario, personality *ReferencePersonality, env *TestEnvironment) (*GenericEvaluationResult, error) {
	// For thread evaluation, we'd typically use the thread processor
	// For now, return a mock result
	return &GenericEvaluationResult{
		ShouldShow: true,
		Reason:     "Thread evaluation not fully implemented",
		Confidence: 0.5,
		NewState:   "visible",
		Metadata:   map[string]interface{}{"handler": "thread"},
	}, nil
}

// ChatMessageEvaluationHandler handles chat message scenario evaluation
type ChatMessageEvaluationHandler struct {
	framework *PersonalityTestFramework
}

// NewChatMessageEvaluationHandler creates a new chat message evaluation handler
func NewChatMessageEvaluationHandler(framework *PersonalityTestFramework) *ChatMessageEvaluationHandler {
	return &ChatMessageEvaluationHandler{
		framework: framework,
	}
}

// GetType returns the scenario type this handler supports
func (cmeh *ChatMessageEvaluationHandler) GetType() ScenarioType {
	return ScenarioTypeChatMessage
}

// Evaluate evaluates a chat message scenario
func (cmeh *ChatMessageEvaluationHandler) Evaluate(ctx context.Context, scenario GenericTestScenario, personality *ReferencePersonality, env *TestEnvironment) (*GenericEvaluationResult, error) {
	// Extract chat message content from scenario
	content, ok := scenario.Content["content"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid chat message content")
	}

	chatContext, _ := scenario.Content["chat_context"].(string)
	if chatContext == "" {
		chatContext = "unknown"
	}

	// Simple evaluation logic based on content
	shouldShow := true
	confidence := 0.8
	reason := fmt.Sprintf("Chat message evaluation for personality %s", personality.Name)

	// Basic filtering logic - this would be more sophisticated in practice
	if len(content) < 10 {
		shouldShow = false
		confidence = 0.9
		reason = "Message too short to be meaningful"
	}

	// Check for AI-related content for tech entrepreneurs
	if personality.Name == "tech_entrepreneur" && contains(content, "AI") {
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
			"content_length": len(content),
			"personality":    personality.Name,
			"chat_context":   chatContext,
			"message_type":   "chat",
		},
	}, nil
}

// EmailEvaluationHandler handles email scenario evaluation
type EmailEvaluationHandler struct {
	framework *PersonalityTestFramework
}

// NewEmailEvaluationHandler creates a new email evaluation handler
func NewEmailEvaluationHandler(framework *PersonalityTestFramework) *EmailEvaluationHandler {
	return &EmailEvaluationHandler{
		framework: framework,
	}
}

// GetType returns the scenario type this handler supports
func (eeh *EmailEvaluationHandler) GetType() ScenarioType {
	return ScenarioTypeEmail
}

// Evaluate evaluates an email scenario
func (eeh *EmailEvaluationHandler) Evaluate(ctx context.Context, scenario GenericTestScenario, personality *ReferencePersonality, env *TestEnvironment) (*GenericEvaluationResult, error) {
	// Extract email content from scenario
	subject, ok := scenario.Content["subject"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid email subject")
	}

	body, ok := scenario.Content["body"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid email body")
	}

	priority, _ := scenario.Content["priority"].(string)
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

// SocialPostEvaluationHandler handles social media post scenario evaluation
type SocialPostEvaluationHandler struct {
	framework *PersonalityTestFramework
}

// NewSocialPostEvaluationHandler creates a new social post evaluation handler
func NewSocialPostEvaluationHandler(framework *PersonalityTestFramework) *SocialPostEvaluationHandler {
	return &SocialPostEvaluationHandler{
		framework: framework,
	}
}

// GetType returns the scenario type this handler supports
func (speh *SocialPostEvaluationHandler) GetType() ScenarioType {
	return ScenarioTypeSocialPost
}

// Evaluate evaluates a social media post scenario
func (speh *SocialPostEvaluationHandler) Evaluate(ctx context.Context, scenario GenericTestScenario, personality *ReferencePersonality, env *TestEnvironment) (*GenericEvaluationResult, error) {
	// Extract social post content from scenario
	content, ok := scenario.Content["content"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid social post content")
	}

	platform, ok := scenario.Content["platform"].(string)
	if !ok {
		platform = "unknown"
	}

	// Extract engagement metrics if available
	engagement := map[string]interface{}{
		"likes":    0,
		"comments": 0,
		"shares":   0,
	}
	if engagementData, ok := scenario.Content["engagement"].(map[string]interface{}); ok {
		if likes, ok := engagementData["likes"].(int); ok {
			engagement["likes"] = likes
		}
		if comments, ok := engagementData["comments"].(int); ok {
			engagement["comments"] = comments
		}
		if shares, ok := engagementData["shares"].(int); ok {
			engagement["shares"] = shares
		}
	}

	// Simple evaluation logic based on content and platform
	shouldShow := true
	confidence := 0.8
	reason := fmt.Sprintf("Social post evaluation for personality %s on %s", personality.Name, platform)

	// Check for tech-related keywords
	techKeywords := []string{"AI", "quantum", "breakthrough", "technology", "innovation", "startup", "funding"}
	isTechRelated := false
	for _, keyword := range techKeywords {
		if contains(content, keyword) {
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
			"content_length": len(content),
			"tech_related":   isTechRelated,
			"personality":    personality.Name,
		},
	}, nil
}

// Helper function to check if a string contains a substring (case-insensitive)
func contains(text, substr string) bool {
	// Simple case-insensitive contains check
	// In a real implementation, you'd want proper case folding
	return len(text) >= len(substr) &&
		findSubstring(strings.ToLower(text), strings.ToLower(substr))
}

// Simple substring search
func findSubstring(text, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(text) < len(substr) {
		return false
	}

	for i := 0; i <= len(text)-len(substr); i++ {
		if text[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// EvaluationHandlerRegistry manages evaluation handlers for different scenario types
type EvaluationHandlerRegistry struct {
	handlers map[ScenarioType]EvaluationHandler
}

// NewEvaluationHandlerRegistry creates a new handler registry
func NewEvaluationHandlerRegistry() *EvaluationHandlerRegistry {
	return &EvaluationHandlerRegistry{
		handlers: make(map[ScenarioType]EvaluationHandler),
	}
}

// Register adds a handler for a specific scenario type
func (ehr *EvaluationHandlerRegistry) Register(handler EvaluationHandler) {
	ehr.handlers[handler.GetType()] = handler
}

// GetHandler retrieves a handler for a specific scenario type
func (ehr *EvaluationHandlerRegistry) GetHandler(scenarioType ScenarioType) (EvaluationHandler, bool) {
	handler, exists := ehr.handlers[scenarioType]
	return handler, exists
}

// ListHandlers returns all registered scenario types
func (ehr *EvaluationHandlerRegistry) ListHandlers() []ScenarioType {
	types := make([]ScenarioType, 0, len(ehr.handlers))
	for t := range ehr.handlers {
		types = append(types, t)
	}
	return types
}
