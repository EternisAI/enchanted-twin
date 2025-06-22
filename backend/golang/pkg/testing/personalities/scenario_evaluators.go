package personalities

import (
	"context"
	"fmt"
	"strings"

	"github.com/EternisAI/enchanted-twin/pkg/holon"
)

// ===== EVALUATION HANDLERS =====

// ThreadEvaluationHandler handles evaluation of holon thread scenarios (backward compatibility)
type ThreadEvaluationHandler struct {
	framework *PersonalityTestFramework
}

func NewThreadEvaluationHandler(framework *PersonalityTestFramework) *ThreadEvaluationHandler {
	return &ThreadEvaluationHandler{framework: framework}
}

func (teh *ThreadEvaluationHandler) GetSupportedType() ScenarioType {
	return ScenarioTypeThread
}

func (teh *ThreadEvaluationHandler) Evaluate(ctx context.Context, content ScenarioContent, personality *ReferencePersonality, env *TestEnvironment) (*GenericEvaluationResult, error) {
	threadContent, ok := content.(*ThreadContent)
	if !ok {
		return nil, fmt.Errorf("invalid content type for thread evaluation handler")
	}

	var evaluation *holon.ThreadEvaluationResult
	var err error

	// Use the existing holon thread processor if available
	if env.ThreadProcessor != nil {
		evaluation, err = env.ThreadProcessor.EvaluateThread(ctx, threadContent.Thread)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate thread: %w", err)
		}
	} else {
		// For mock testing, create a simulated evaluation
		evaluation = &holon.ThreadEvaluationResult{
			ShouldShow: true, // Default to showing threads
			Reason:     fmt.Sprintf("Simulated evaluation for %s personality", personality.Name),
			Confidence: 0.75,
			NewState:   "visible",
		}
	}

	// Convert to generic result
	return &GenericEvaluationResult{
		ShouldShow: evaluation.ShouldShow,
		Reason:     evaluation.Reason,
		Confidence: evaluation.Confidence,
		NewState:   evaluation.NewState,
		Metadata: map[string]interface{}{
			"thread_id": threadContent.GetMetadata()["thread_id"],
			"title":     threadContent.GetDisplayTitle(),
		},
	}, nil
}

// ChatMessageEvaluationHandler handles evaluation of chat message scenarios
type ChatMessageEvaluationHandler struct {
	framework *PersonalityTestFramework
}

func NewChatMessageEvaluationHandler(framework *PersonalityTestFramework) *ChatMessageEvaluationHandler {
	return &ChatMessageEvaluationHandler{framework: framework}
}

func (cmeh *ChatMessageEvaluationHandler) GetSupportedType() ScenarioType {
	return ScenarioTypeChatMessage
}

func (cmeh *ChatMessageEvaluationHandler) Evaluate(ctx context.Context, content ScenarioContent, personality *ReferencePersonality, env *TestEnvironment) (*GenericEvaluationResult, error) {
	messageContent, ok := content.(*ChatMessageContent)
	if !ok {
		return nil, fmt.Errorf("invalid content type for chat message evaluation handler")
	}

	// Use LLM-based evaluation for chat messages
	if cmeh.framework.aiService != nil {
		return cmeh.evaluateWithLLM(ctx, messageContent, personality)
	}

	// Fallback to rule-based evaluation
	return cmeh.evaluateWithRules(messageContent, personality), nil
}

func (cmeh *ChatMessageEvaluationHandler) evaluateWithLLM(ctx context.Context, content *ChatMessageContent, personality *ReferencePersonality) (*GenericEvaluationResult, error) {
	// Build evaluation prompt for chat messages
	// Note: In a full implementation, you would use the AI service with this prompt:
	// prompt := fmt.Sprintf(`Evaluate this chat message for a %s personality: ...`, personality.Name, ...)
	
	// For now, return a basic evaluation
	shouldShow := cmeh.evaluateBasedOnInterests(content, personality)
	confidence := 0.7
	if shouldShow {
		confidence = 0.8
	} else {
		confidence = 0.6
	}

	return &GenericEvaluationResult{
		ShouldShow: shouldShow,
		Reason:     fmt.Sprintf("Chat message evaluation for %s personality based on content relevance", personality.Name),
		Confidence: confidence,
		NewState:   "visible",
		Metadata: map[string]interface{}{
			"message_id":   content.MessageID,
			"chat_context": content.ChatContext,
			"author":       content.Author.Identity,
		},
	}, nil
}

func (cmeh *ChatMessageEvaluationHandler) evaluateWithRules(content *ChatMessageContent, personality *ReferencePersonality) *GenericEvaluationResult {
	shouldShow := cmeh.evaluateBasedOnInterests(content, personality)
	
	confidence := 0.5
	reason := "Rule-based evaluation for chat message"
	
	if shouldShow {
		confidence = 0.7
		reason = fmt.Sprintf("Message contains topics relevant to %s interests", personality.Name)
	} else {
		confidence = 0.6
		reason = fmt.Sprintf("Message not relevant to %s interests", personality.Name)
	}

	return &GenericEvaluationResult{
		ShouldShow: shouldShow,
		Reason:     reason,
		Confidence: confidence,
		NewState:   "visible",
		Metadata: map[string]interface{}{
			"message_id":   content.MessageID,
			"chat_context": content.ChatContext,
		},
	}
}

func (cmeh *ChatMessageEvaluationHandler) evaluateBasedOnInterests(content *ChatMessageContent, personality *ReferencePersonality) bool {
	messageText := strings.ToLower(content.Text)
	
	// Check if message contains personality interests
	for _, interest := range personality.Profile.Interests {
		if strings.Contains(messageText, strings.ToLower(interest)) {
			return true
		}
	}
	
	// Check keywords from message against personality traits
	keywords := content.GetKeywords()
	for _, keyword := range keywords {
		for _, trait := range personality.Profile.CoreTraits {
			if strings.Contains(strings.ToLower(trait), strings.ToLower(keyword)) {
				return true
			}
		}
	}
	
	return false
}

// EmailEvaluationHandler handles evaluation of email scenarios
type EmailEvaluationHandler struct {
	framework *PersonalityTestFramework
}

func NewEmailEvaluationHandler(framework *PersonalityTestFramework) *EmailEvaluationHandler {
	return &EmailEvaluationHandler{framework: framework}
}

func (eeh *EmailEvaluationHandler) GetSupportedType() ScenarioType {
	return ScenarioTypeEmail
}

func (eeh *EmailEvaluationHandler) Evaluate(ctx context.Context, content ScenarioContent, personality *ReferencePersonality, env *TestEnvironment) (*GenericEvaluationResult, error) {
	emailContent, ok := content.(*EmailContent)
	if !ok {
		return nil, fmt.Errorf("invalid content type for email evaluation handler")
	}

	// Email evaluation logic
	shouldShow := eeh.evaluateEmailRelevance(emailContent, personality)
	confidence := eeh.calculateEmailConfidence(emailContent, personality)
	
	reason := "Email evaluation based on subject, content, and sender relevance"
	if !shouldShow {
		reason = "Email marked as low priority or not relevant to personality interests"
	}

	return &GenericEvaluationResult{
		ShouldShow: shouldShow,
		Reason:     reason,
		Confidence: confidence,
		NewState:   "visible",
		Metadata: map[string]interface{}{
			"subject":  emailContent.Subject,
			"from":     emailContent.From.Identity,
			"priority": emailContent.Priority,
		},
	}, nil
}

func (eeh *EmailEvaluationHandler) evaluateEmailRelevance(content *EmailContent, personality *ReferencePersonality) bool {
	// High priority emails should generally be shown
	if content.Priority == "high" || content.Priority == "urgent" {
		return true
	}
	
	// Check subject and body for relevant keywords
	fullText := strings.ToLower(content.Subject + " " + content.Body)
	
	for _, interest := range personality.Profile.Interests {
		if strings.Contains(fullText, strings.ToLower(interest)) {
			return true
		}
	}
	
	// Check if it's work-related for entrepreneurs
	if strings.Contains(personality.Name, "entrepreneur") {
		workKeywords := []string{"business", "meeting", "proposal", "investment", "startup", "funding"}
		for _, keyword := range workKeywords {
			if strings.Contains(fullText, keyword) {
				return true
			}
		}
	}
	
	return len(content.Body) > 500 // Show longer emails as potentially important
}

func (eeh *EmailEvaluationHandler) calculateEmailConfidence(content *EmailContent, personality *ReferencePersonality) float64 {
	confidence := 0.5
	
	// Increase confidence for high priority
	if content.Priority == "high" || content.Priority == "urgent" {
		confidence += 0.3
	}
	
	// Increase confidence for relevant keywords
	fullText := strings.ToLower(content.Subject + " " + content.Body)
	matchCount := 0
	for _, interest := range personality.Profile.Interests {
		if strings.Contains(fullText, strings.ToLower(interest)) {
			matchCount++
		}
	}
	
	confidence += float64(matchCount) * 0.1
	
	if confidence > 1.0 {
		confidence = 1.0
	}
	
	return confidence
}

// SocialPostEvaluationHandler handles evaluation of social media post scenarios
type SocialPostEvaluationHandler struct {
	framework *PersonalityTestFramework
}

func NewSocialPostEvaluationHandler(framework *PersonalityTestFramework) *SocialPostEvaluationHandler {
	return &SocialPostEvaluationHandler{framework: framework}
}

func (speh *SocialPostEvaluationHandler) GetSupportedType() ScenarioType {
	return ScenarioTypeSocialPost
}

func (speh *SocialPostEvaluationHandler) Evaluate(ctx context.Context, content ScenarioContent, personality *ReferencePersonality, env *TestEnvironment) (*GenericEvaluationResult, error) {
	socialContent, ok := content.(*SocialPostContent)
	if !ok {
		return nil, fmt.Errorf("invalid content type for social post evaluation handler")
	}

	shouldShow := speh.evaluateSocialRelevance(socialContent, personality)
	confidence := speh.calculateSocialConfidence(socialContent, personality)
	
	reason := fmt.Sprintf("Social media post evaluation for %s platform", socialContent.Platform)
	if !shouldShow {
		reason = "Social post not relevant to personality interests or engagement patterns"
	}

	return &GenericEvaluationResult{
		ShouldShow: shouldShow,
		Reason:     reason,
		Confidence: confidence,
		NewState:   "visible",
		Metadata: map[string]interface{}{
			"platform":    socialContent.Platform,
			"likes":       socialContent.Likes,
			"shares":      socialContent.Shares,
			"engagement":  socialContent.Likes + socialContent.Shares + socialContent.Comments,
			"tag_count":   len(socialContent.Tags),
			"image_count": len(socialContent.ImageURLs),
		},
	}, nil
}

func (speh *SocialPostEvaluationHandler) evaluateSocialRelevance(content *SocialPostContent, personality *ReferencePersonality) bool {
	// Check engagement metrics - highly engaged content is more likely relevant
	totalEngagement := content.Likes + content.Shares + content.Comments
	if totalEngagement > 100 {
		return true
	}
	
	// Check content relevance
	postText := strings.ToLower(content.Text)
	for _, interest := range personality.Profile.Interests {
		if strings.Contains(postText, strings.ToLower(interest)) {
			return true
		}
	}
	
	// Check tags
	for _, tag := range content.Tags {
		for _, interest := range personality.Profile.Interests {
			if strings.Contains(strings.ToLower(tag), strings.ToLower(interest)) {
				return true
			}
		}
	}
	
	// Platform-specific logic
	switch content.Platform {
	case "linkedin":
		// LinkedIn posts are more relevant for entrepreneurs
		if strings.Contains(personality.Name, "entrepreneur") {
			return true
		}
	case "twitter":
		// Twitter posts with tech keywords for tech personalities
		techKeywords := []string{"ai", "tech", "startup", "innovation"}
		for _, keyword := range techKeywords {
			if strings.Contains(postText, keyword) && 
			   (strings.Contains(personality.Name, "tech") || strings.Contains(personality.Name, "entrepreneur")) {
				return true
			}
		}
	case "instagram":
		// Instagram posts more relevant for creative personalities
		if strings.Contains(personality.Name, "creative") || strings.Contains(personality.Name, "artist") {
			return len(content.ImageURLs) > 0 // Visual content for creatives
		}
	}
	
	return false
}

func (speh *SocialPostEvaluationHandler) calculateSocialConfidence(content *SocialPostContent, personality *ReferencePersonality) float64 {
	confidence := 0.4
	
	// Engagement boost
	totalEngagement := content.Likes + content.Shares + content.Comments
	if totalEngagement > 1000 {
		confidence += 0.3
	} else if totalEngagement > 100 {
		confidence += 0.2
	} else if totalEngagement > 10 {
		confidence += 0.1
	}
	
	// Content relevance boost
	postText := strings.ToLower(content.Text)
	for _, interest := range personality.Profile.Interests {
		if strings.Contains(postText, strings.ToLower(interest)) {
			confidence += 0.15
		}
	}
	
	// Platform alignment boost
	switch content.Platform {
	case "linkedin":
		if strings.Contains(personality.Name, "entrepreneur") {
			confidence += 0.1
		}
	case "instagram":
		if strings.Contains(personality.Name, "creative") || strings.Contains(personality.Name, "artist") {
			confidence += 0.1
		}
	}
	
	if confidence > 1.0 {
		confidence = 1.0
	}
	
	return confidence
}

// EvaluationHandlerRegistry manages different evaluation handlers
type EvaluationHandlerRegistry struct {
	handlers map[ScenarioType]EvaluationHandler
}

func NewEvaluationHandlerRegistry() *EvaluationHandlerRegistry {
	return &EvaluationHandlerRegistry{
		handlers: make(map[ScenarioType]EvaluationHandler),
	}
}

func (ehr *EvaluationHandlerRegistry) Register(handler EvaluationHandler) {
	ehr.handlers[handler.GetSupportedType()] = handler
}

func (ehr *EvaluationHandlerRegistry) GetHandler(scenarioType ScenarioType) (EvaluationHandler, bool) {
	handler, exists := ehr.handlers[scenarioType]
	return handler, exists
}

func (ehr *EvaluationHandlerRegistry) GetSupportedTypes() []ScenarioType {
	types := make([]ScenarioType, 0, len(ehr.handlers))
	for scenarioType := range ehr.handlers {
		types = append(types, scenarioType)
	}
	return types
}