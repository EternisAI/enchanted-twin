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
			Context:                 make(map[string]interface{}),
			PersonalityExpectations: make([]PersonalityExpectedOutcome, 0),
		},
	}
}

// WithContent sets the content as a ScenarioContent interface
func (fsb *FlexibleScenarioBuilder) WithContent(content ScenarioContent) *FlexibleScenarioBuilder {
	fsb.scenario.Content = content
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
			CreatedAt: now,
			Metadata:  make(map[string]interface{}),
		},
	}
}

// WithMessage sets the message content
func (cmsb *ChatMessageScenarioBuilder) WithMessage(content, authorIdentity string) *ChatMessageScenarioBuilder {
	cmsb.content.Content = content
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
	// Set the content directly as ScenarioContent
	cmsb.builder.WithContent(cmsb.content)

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
	esb.content.To = []ContentAuthor{recipient}
	return esb
}

// WithPriority sets the email priority
func (esb *EmailScenarioBuilder) WithPriority(priority string) *EmailScenarioBuilder {
	esb.content.Priority = priority
	return esb
}

// WithTimestamp sets the email timestamp
func (esb *EmailScenarioBuilder) WithTimestamp(timestamp time.Time) *EmailScenarioBuilder {
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
	// Set the content directly as ScenarioContent
	esb.builder.WithContent(esb.content)

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
			CreatedAt: now,
			Metadata:  make(map[string]interface{}),
		},
	}
}

// WithPost sets the post content and platform
func (spsb *SocialPostScenarioBuilder) WithPost(content, platform string) *SocialPostScenarioBuilder {
	spsb.content.Text = content
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
	spsb.content.Likes = likes
	spsb.content.Comments = comments
	spsb.content.Shares = shares
	return spsb
}

// WithTimestamp sets the post timestamp
func (spsb *SocialPostScenarioBuilder) WithTimestamp(timestamp time.Time) *SocialPostScenarioBuilder {
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
	// Set the content directly as ScenarioContent
	spsb.builder.WithContent(spsb.content)

	// Set evaluation handler
	handler := NewSocialPostEvaluationHandler(ptf)
	spsb.builder.WithEvaluationHandler(handler)

	return spsb.builder.Build()
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}
