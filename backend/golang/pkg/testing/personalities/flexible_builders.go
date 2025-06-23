package personalities

import (
	"fmt"
	"time"
)

// FlexibleScenarioBuilder provides a fluent interface for building generic test scenarios
type FlexibleScenarioBuilder struct {
	scenario *GenericTestScenario
}

// NewFlexibleScenarioBuilder creates a new flexible scenario builder
func NewFlexibleScenarioBuilder(name, description string, scenarioType ScenarioType) *FlexibleScenarioBuilder {
	return &FlexibleScenarioBuilder{
		scenario: &GenericTestScenario{
			Name:                    name,
			Description:             description,
			Type:                    scenarioType,
			Content:                 make(map[string]interface{}),
			Context:                 make(map[string]interface{}),
			PersonalityExpectations: make([]PersonalityExpectedOutcome, 0),
			CreatedAt:               time.Now(),
		},
	}
}

// WithContent sets the content as a map
func (fsb *FlexibleScenarioBuilder) WithContent(content map[string]interface{}) *FlexibleScenarioBuilder {
	fsb.scenario.Content = content
	return fsb
}

// WithAuthor sets the author information
func (fsb *FlexibleScenarioBuilder) WithAuthor(identity string, name, alias, email *string) *FlexibleScenarioBuilder {
	fsb.scenario.Author = ContentAuthor{
		Identity: identity,
		Name:     name,
		Alias:    alias,
		Email:    email,
	}
	return fsb
}

// WithContext adds a context key-value pair
func (fsb *FlexibleScenarioBuilder) WithContext(key string, value interface{}) *FlexibleScenarioBuilder {
	fsb.scenario.Context[key] = value
	return fsb
}

// WithEvaluationHandler sets the evaluation handler
func (fsb *FlexibleScenarioBuilder) WithEvaluationHandler(handler EvaluationHandler) *FlexibleScenarioBuilder {
	fsb.scenario.EvaluationHandler = handler
	return fsb
}

// ExpectPersonality adds a personality expectation
func (fsb *FlexibleScenarioBuilder) ExpectPersonality(personalityName string, shouldShow bool, confidence float64, priority int, rationale string) *FlexibleScenarioBuilder {
	expectation := PersonalityExpectedOutcome{
		PersonalityName: personalityName,
		ShouldShow:      shouldShow,
		Confidence:      confidence,
		Priority:        priority,
		Rationale:       rationale,
		ExpectedState:   "visible",
	}
	if !shouldShow {
		expectation.ExpectedState = "hidden"
	}
	fsb.scenario.PersonalityExpectations = append(fsb.scenario.PersonalityExpectations, expectation)
	return fsb
}

// Build returns the completed scenario
func (fsb *FlexibleScenarioBuilder) Build() *GenericTestScenario {
	return fsb.scenario
}

// ChatMessageScenarioBuilder builds chat message scenarios
type ChatMessageScenarioBuilder struct {
	builder *FlexibleScenarioBuilder
	content *ChatMessageContent
}

// NewChatMessageScenario creates a new chat message scenario builder
func NewChatMessageScenario(name, description string) *ChatMessageScenarioBuilder {
	now := time.Now()
	return &ChatMessageScenarioBuilder{
		builder: NewFlexibleScenarioBuilder(name, description, ScenarioTypeChatMessage),
		content: &ChatMessageContent{
			MessageID: fmt.Sprintf("msg-%d", now.UnixNano()),
			Timestamp: now,
			CreatedAt: now,
			Metadata:  make(map[string]interface{}),
		},
	}
}

// WithMessage sets the message content
func (cmsb *ChatMessageScenarioBuilder) WithMessage(content, authorIdentity string) *ChatMessageScenarioBuilder {
	cmsb.content.Content = content
	cmsb.content.Text = content // Set both fields
	cmsb.content.Author.Identity = authorIdentity
	return cmsb
}

// WithAuthor sets the author details
func (cmsb *ChatMessageScenarioBuilder) WithAuthor(identity string, name, alias *string) *ChatMessageScenarioBuilder {
	cmsb.content.Author = ContentAuthor{
		Identity: identity,
		Name:     name,
		Alias:    alias,
	}
	return cmsb
}

// WithChatContext sets the chat context
func (cmsb *ChatMessageScenarioBuilder) WithChatContext(context string) *ChatMessageScenarioBuilder {
	cmsb.content.ChatContext = context
	return cmsb
}

// WithTimestamp sets the message timestamp
func (cmsb *ChatMessageScenarioBuilder) WithTimestamp(timestamp time.Time) *ChatMessageScenarioBuilder {
	cmsb.content.Timestamp = timestamp
	cmsb.content.CreatedAt = timestamp
	return cmsb
}

// WithMetadata adds metadata
func (cmsb *ChatMessageScenarioBuilder) WithMetadata(key string, value interface{}) *ChatMessageScenarioBuilder {
	cmsb.content.Metadata[key] = value
	return cmsb
}

// WithContext adds a context key-value pair
func (cmsb *ChatMessageScenarioBuilder) WithContext(key string, value interface{}) *ChatMessageScenarioBuilder {
	cmsb.builder.WithContext(key, value)
	return cmsb
}

// ExpectPersonality adds a personality expectation
func (cmsb *ChatMessageScenarioBuilder) ExpectPersonality(personalityName string, shouldShow bool, confidence float64, priority int, rationale string) *ChatMessageScenarioBuilder {
	cmsb.builder.ExpectPersonality(personalityName, shouldShow, confidence, priority, rationale)
	return cmsb
}

// Build creates the final scenario
func (cmsb *ChatMessageScenarioBuilder) Build(ptf *PersonalityTestFramework) *GenericTestScenario {
	// Convert content to map[string]interface{}
	contentMap := map[string]interface{}{
		"message_id":    cmsb.content.MessageID,
		"content":       cmsb.content.Content,
		"text":          cmsb.content.Text,
		"chat_context":  cmsb.content.ChatContext,
		"timestamp":     cmsb.content.Timestamp,
		"created_at":    cmsb.content.CreatedAt,
		"metadata":      cmsb.content.Metadata,
	}
	
	cmsb.builder.WithContent(contentMap)
	cmsb.builder.WithAuthor(cmsb.content.Author.Identity, cmsb.content.Author.Name, cmsb.content.Author.Alias, nil)
	
	// Set evaluation handler
	handler := NewChatMessageEvaluationHandler(ptf)
	cmsb.builder.WithEvaluationHandler(handler)
	
	return cmsb.builder.Build()
}

// EmailScenarioBuilder builds email scenarios
type EmailScenarioBuilder struct {
	builder *FlexibleScenarioBuilder
	content *EmailContent
}

// NewEmailScenario creates a new email scenario builder
func NewEmailScenario(name, description string) *EmailScenarioBuilder {
	now := time.Now()
	return &EmailScenarioBuilder{
		builder: NewFlexibleScenarioBuilder(name, description, ScenarioTypeEmail),
		content: &EmailContent{
			MessageID: fmt.Sprintf("email-%d", now.UnixNano()),
			Timestamp: now,
			CreatedAt: now,
			Metadata:  make(map[string]interface{}),
		},
	}
}

// WithEmail sets the email subject and body
func (esb *EmailScenarioBuilder) WithEmail(subject, body string) *EmailScenarioBuilder {
	esb.content.Subject = subject
	esb.content.Body = body
	return esb
}

// WithFrom sets the sender information
func (esb *EmailScenarioBuilder) WithFrom(identity string, name, alias, email *string) *EmailScenarioBuilder {
	esb.content.From = ContentAuthor{
		Identity: identity,
		Name:     name,
		Alias:    alias,
		Email:    email,
	}
	return esb
}

// WithTo sets the recipient information
func (esb *EmailScenarioBuilder) WithTo(recipient ContentAuthor) *EmailScenarioBuilder {
	esb.content.To = recipient
	return esb
}

// WithPriority sets the email priority
func (esb *EmailScenarioBuilder) WithPriority(priority string) *EmailScenarioBuilder {
	esb.content.Priority = priority
	return esb
}

// WithTimestamp sets the email timestamp
func (esb *EmailScenarioBuilder) WithTimestamp(timestamp time.Time) *EmailScenarioBuilder {
	esb.content.Timestamp = timestamp
	esb.content.CreatedAt = timestamp
	return esb
}

// WithMetadata adds metadata
func (esb *EmailScenarioBuilder) WithMetadata(key string, value interface{}) *EmailScenarioBuilder {
	esb.content.Metadata[key] = value
	return esb
}

// WithContext adds a context key-value pair
func (esb *EmailScenarioBuilder) WithContext(key string, value interface{}) *EmailScenarioBuilder {
	esb.builder.WithContext(key, value)
	return esb
}

// ExpectPersonality adds a personality expectation
func (esb *EmailScenarioBuilder) ExpectPersonality(personalityName string, shouldShow bool, confidence float64, priority int, rationale string) *EmailScenarioBuilder {
	esb.builder.ExpectPersonality(personalityName, shouldShow, confidence, priority, rationale)
	return esb
}

// Build creates the final scenario
func (esb *EmailScenarioBuilder) Build(ptf *PersonalityTestFramework) *GenericTestScenario {
	// Convert content to map[string]interface{}
	contentMap := map[string]interface{}{
		"message_id": esb.content.MessageID,
		"subject":    esb.content.Subject,
		"body":       esb.content.Body,
		"priority":   esb.content.Priority,
		"timestamp":  esb.content.Timestamp,
		"created_at": esb.content.CreatedAt,
		"metadata":   esb.content.Metadata,
	}
	
	esb.builder.WithContent(contentMap)
	esb.builder.WithAuthor(esb.content.From.Identity, esb.content.From.Name, esb.content.From.Alias, esb.content.From.Email)
	esb.builder.WithContext("recipient", esb.content.To)
	
	// Set evaluation handler
	handler := NewEmailEvaluationHandler(ptf)
	esb.builder.WithEvaluationHandler(handler)
	
	return esb.builder.Build()
}

// SocialPostScenarioBuilder builds social media post scenarios
type SocialPostScenarioBuilder struct {
	builder *FlexibleScenarioBuilder
	content *SocialPostContent
}

// NewSocialPostScenario creates a new social post scenario builder
func NewSocialPostScenario(name, description string) *SocialPostScenarioBuilder {
	now := time.Now()
	return &SocialPostScenarioBuilder{
		builder: NewFlexibleScenarioBuilder(name, description, ScenarioTypeSocialPost),
		content: &SocialPostContent{
			PostID:    fmt.Sprintf("post-%d", now.UnixNano()),
			Timestamp: now,
			CreatedAt: now,
			Metadata:  make(map[string]interface{}),
		},
	}
}

// WithPost sets the post content and platform
func (spsb *SocialPostScenarioBuilder) WithPost(content, platform string) *SocialPostScenarioBuilder {
	spsb.content.Content = content
	spsb.content.Platform = platform
	return spsb
}

// WithAuthor sets the author details
func (spsb *SocialPostScenarioBuilder) WithAuthor(identity string, name, alias *string) *SocialPostScenarioBuilder {
	spsb.content.Author = ContentAuthor{
		Identity: identity,
		Name:     name,
		Alias:    alias,
	}
	return spsb
}

// WithTags sets the post tags
func (spsb *SocialPostScenarioBuilder) WithTags(tags ...string) *SocialPostScenarioBuilder {
	spsb.content.Tags = tags
	return spsb
}

// WithImages sets the image URLs
func (spsb *SocialPostScenarioBuilder) WithImages(urls ...string) *SocialPostScenarioBuilder {
	spsb.content.ImageURLs = urls
	return spsb
}

// WithEngagement sets the engagement metrics
func (spsb *SocialPostScenarioBuilder) WithEngagement(likes, comments, shares int) *SocialPostScenarioBuilder {
	spsb.content.Engagement.Likes = likes
	spsb.content.Engagement.Comments = comments
	spsb.content.Engagement.Shares = shares
	return spsb
}

// WithTimestamp sets the post timestamp
func (spsb *SocialPostScenarioBuilder) WithTimestamp(timestamp time.Time) *SocialPostScenarioBuilder {
	spsb.content.Timestamp = timestamp
	spsb.content.CreatedAt = timestamp
	return spsb
}

// WithMetadata adds metadata
func (spsb *SocialPostScenarioBuilder) WithMetadata(key string, value interface{}) *SocialPostScenarioBuilder {
	spsb.content.Metadata[key] = value
	return spsb
}

// WithContext adds a context key-value pair
func (spsb *SocialPostScenarioBuilder) WithContext(key string, value interface{}) *SocialPostScenarioBuilder {
	spsb.builder.WithContext(key, value)
	return spsb
}

// ExpectPersonality adds a personality expectation
func (spsb *SocialPostScenarioBuilder) ExpectPersonality(personalityName string, shouldShow bool, confidence float64, priority int, rationale string) *SocialPostScenarioBuilder {
	spsb.builder.ExpectPersonality(personalityName, shouldShow, confidence, priority, rationale)
	return spsb
}

// Build creates the final scenario
func (spsb *SocialPostScenarioBuilder) Build(ptf *PersonalityTestFramework) *GenericTestScenario {
	// Convert engagement struct to map to avoid type assertion issues
	engagementMap := map[string]interface{}{
		"likes":    spsb.content.Engagement.Likes,
		"comments": spsb.content.Engagement.Comments,
		"shares":   spsb.content.Engagement.Shares,
	}
	
	// Convert content to map[string]interface{}
	contentMap := map[string]interface{}{
		"post_id":     spsb.content.PostID,
		"content":     spsb.content.Content,
		"platform":    spsb.content.Platform,
		"tags":        spsb.content.Tags,
		"image_urls":  spsb.content.ImageURLs,
		"engagement":  engagementMap, // Use the converted map instead of struct
		"timestamp":   spsb.content.Timestamp,
		"created_at":  spsb.content.CreatedAt,
		"metadata":    spsb.content.Metadata,
	}
	
	spsb.builder.WithContent(contentMap)
	spsb.builder.WithAuthor(spsb.content.Author.Identity, spsb.content.Author.Name, spsb.content.Author.Alias, nil)
	
	// Set evaluation handler
	handler := NewSocialPostEvaluationHandler(ptf)
	spsb.builder.WithEvaluationHandler(handler)
	
	return spsb.builder.Build()
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}