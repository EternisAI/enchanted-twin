package personalities

import (
	"fmt"
	"time"
)

// ===== FLEXIBLE SCENARIO BUILDERS =====

// GenericScenarioBuilder provides a fluent interface for building any type of scenario
type GenericScenarioBuilder struct {
	scenario *GenericTestScenario
}

// NewGenericScenarioBuilder creates a new generic scenario builder
func NewGenericScenarioBuilder(name, description string, scenarioType ScenarioType) *GenericScenarioBuilder {
	return &GenericScenarioBuilder{
		scenario: &GenericTestScenario{
			Name:        name,
			Description: description,
			Type:        scenarioType,
			Context:     make(map[string]interface{}),
		},
	}
}

// WithContent sets the scenario content
func (gsb *GenericScenarioBuilder) WithContent(content ScenarioContent) *GenericScenarioBuilder {
	gsb.scenario.Content = content
	return gsb
}

// WithContext adds context metadata
func (gsb *GenericScenarioBuilder) WithContext(key string, value interface{}) *GenericScenarioBuilder {
	gsb.scenario.Context[key] = value
	return gsb
}

// WithPersonalityExpectations adds personality-specific expectations
func (gsb *GenericScenarioBuilder) WithPersonalityExpectations(expectations ...PersonalityExpectedOutcome) *GenericScenarioBuilder {
	gsb.scenario.PersonalityExpectations = append(gsb.scenario.PersonalityExpectations, expectations...)
	return gsb
}

// WithEvaluationHandler sets the evaluation handler
func (gsb *GenericScenarioBuilder) WithEvaluationHandler(handler EvaluationHandler) *GenericScenarioBuilder {
	gsb.scenario.EvaluationHandler = handler
	return gsb
}

// Build creates the final scenario
func (gsb *GenericScenarioBuilder) Build() *GenericTestScenario {
	return gsb.scenario
}

// ===== CONTENT-SPECIFIC BUILDERS =====

// ChatMessageScenarioBuilder builds chat message scenarios
type ChatMessageScenarioBuilder struct {
	builder *GenericScenarioBuilder
	content *ChatMessageContent
}

// NewChatMessageScenario creates a new chat message scenario builder
func NewChatMessageScenario(name, description string) *ChatMessageScenarioBuilder {
	content := &ChatMessageContent{
		MessageID: fmt.Sprintf("msg-%d", time.Now().UnixNano()),
		CreatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}
	
	builder := NewGenericScenarioBuilder(name, description, ScenarioTypeChatMessage)
	builder.WithContent(content)
	
	return &ChatMessageScenarioBuilder{
		builder: builder,
		content: content,
	}
}

// WithMessage sets the message content
func (cmsb *ChatMessageScenarioBuilder) WithMessage(text, authorIdentity string) *ChatMessageScenarioBuilder {
	cmsb.content.Text = text
	cmsb.content.Author.Identity = authorIdentity
	return cmsb
}

// WithAuthor sets detailed author information
func (cmsb *ChatMessageScenarioBuilder) WithAuthor(identity string, alias *string, name *string) *ChatMessageScenarioBuilder {
	cmsb.content.Author = ContentAuthor{
		Identity: identity,
		Alias:    alias,
		Name:     name,
	}
	return cmsb
}

// WithChatContext sets the chat context
func (cmsb *ChatMessageScenarioBuilder) WithChatContext(context string) *ChatMessageScenarioBuilder {
	cmsb.content.ChatContext = context
	return cmsb
}

// WithTimestamp sets the message timestamp
func (cmsb *ChatMessageScenarioBuilder) WithTimestamp(t time.Time) *ChatMessageScenarioBuilder {
	cmsb.content.CreatedAt = t
	return cmsb
}

// WithMetadata adds custom metadata
func (cmsb *ChatMessageScenarioBuilder) WithMetadata(key string, value interface{}) *ChatMessageScenarioBuilder {
	cmsb.content.Metadata[key] = value
	return cmsb
}

// WithContext adds scenario context
func (cmsb *ChatMessageScenarioBuilder) WithContext(key string, value interface{}) *ChatMessageScenarioBuilder {
	cmsb.builder.WithContext(key, value)
	return cmsb
}

// WithPersonalityExpectations adds personality expectations
func (cmsb *ChatMessageScenarioBuilder) WithPersonalityExpectations(expectations ...PersonalityExpectedOutcome) *ChatMessageScenarioBuilder {
	cmsb.builder.WithPersonalityExpectations(expectations...)
	return cmsb
}

// ExpectPersonality adds a single personality expectation
func (cmsb *ChatMessageScenarioBuilder) ExpectPersonality(name string, shouldShow bool, confidence float64, priority int, rationale string) *ChatMessageScenarioBuilder {
	expectation := PersonalityExpectedOutcome{
		PersonalityName: name,
		ShouldShow:      shouldShow,
		Confidence:      confidence,
		Priority:        priority,
		Rationale:       rationale,
		ExpectedState:   "visible",
	}
	if !shouldShow {
		expectation.ExpectedState = "hidden"
	}
	return cmsb.WithPersonalityExpectations(expectation)
}

// Build creates the final scenario
func (cmsb *ChatMessageScenarioBuilder) Build(framework *PersonalityTestFramework) *GenericTestScenario {
	// Set up evaluation handler
	handler := NewChatMessageEvaluationHandler(framework)
	cmsb.builder.WithEvaluationHandler(handler)
	
	return cmsb.builder.Build()
}

// EmailScenarioBuilder builds email scenarios
type EmailScenarioBuilder struct {
	builder *GenericScenarioBuilder
	content *EmailContent
}

// NewEmailScenario creates a new email scenario builder
func NewEmailScenario(name, description string) *EmailScenarioBuilder {
	content := &EmailContent{
		CreatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}
	
	builder := NewGenericScenarioBuilder(name, description, ScenarioTypeEmail)
	builder.WithContent(content)
	
	return &EmailScenarioBuilder{
		builder: builder,
		content: content,
	}
}

// WithEmail sets the email content
func (esb *EmailScenarioBuilder) WithEmail(subject, body string) *EmailScenarioBuilder {
	esb.content.Subject = subject
	esb.content.Body = body
	return esb
}

// WithFrom sets the sender
func (esb *EmailScenarioBuilder) WithFrom(identity string, alias *string, name *string, email *string) *EmailScenarioBuilder {
	esb.content.From = ContentAuthor{
		Identity: identity,
		Alias:    alias,
		Name:     name,
		Email:    email,
	}
	return esb
}

// WithTo adds recipients
func (esb *EmailScenarioBuilder) WithTo(recipients ...ContentAuthor) *EmailScenarioBuilder {
	esb.content.To = append(esb.content.To, recipients...)
	return esb
}

// WithCC adds CC recipients
func (esb *EmailScenarioBuilder) WithCC(recipients ...ContentAuthor) *EmailScenarioBuilder {
	esb.content.CC = append(esb.content.CC, recipients...)
	return esb
}

// WithPriority sets the email priority
func (esb *EmailScenarioBuilder) WithPriority(priority string) *EmailScenarioBuilder {
	esb.content.Priority = priority
	return esb
}

// WithTimestamp sets the email timestamp
func (esb *EmailScenarioBuilder) WithTimestamp(t time.Time) *EmailScenarioBuilder {
	esb.content.CreatedAt = t
	return esb
}

// WithContext adds scenario context
func (esb *EmailScenarioBuilder) WithContext(key string, value interface{}) *EmailScenarioBuilder {
	esb.builder.WithContext(key, value)
	return esb
}

// WithPersonalityExpectations adds personality expectations
func (esb *EmailScenarioBuilder) WithPersonalityExpectations(expectations ...PersonalityExpectedOutcome) *EmailScenarioBuilder {
	esb.builder.WithPersonalityExpectations(expectations...)
	return esb
}

// ExpectPersonality adds a single personality expectation
func (esb *EmailScenarioBuilder) ExpectPersonality(name string, shouldShow bool, confidence float64, priority int, rationale string) *EmailScenarioBuilder {
	expectation := PersonalityExpectedOutcome{
		PersonalityName: name,
		ShouldShow:      shouldShow,
		Confidence:      confidence,
		Priority:        priority,
		Rationale:       rationale,
		ExpectedState:   "visible",
	}
	if !shouldShow {
		expectation.ExpectedState = "hidden"
	}
	return esb.WithPersonalityExpectations(expectation)
}

// Build creates the final scenario
func (esb *EmailScenarioBuilder) Build(framework *PersonalityTestFramework) *GenericTestScenario {
	// Set up evaluation handler
	handler := NewEmailEvaluationHandler(framework)
	esb.builder.WithEvaluationHandler(handler)
	
	return esb.builder.Build()
}

// SocialPostScenarioBuilder builds social media post scenarios
type SocialPostScenarioBuilder struct {
	builder *GenericScenarioBuilder
	content *SocialPostContent
}

// NewSocialPostScenario creates a new social post scenario builder
func NewSocialPostScenario(name, description string) *SocialPostScenarioBuilder {
	content := &SocialPostContent{
		PostID:    fmt.Sprintf("post-%d", time.Now().UnixNano()),
		CreatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}
	
	builder := NewGenericScenarioBuilder(name, description, ScenarioTypeSocialPost)
	builder.WithContent(content)
	
	return &SocialPostScenarioBuilder{
		builder: builder,
		content: content,
	}
}

// WithPost sets the post content
func (spsb *SocialPostScenarioBuilder) WithPost(text, platform string) *SocialPostScenarioBuilder {
	spsb.content.Text = text
	spsb.content.Platform = platform
	return spsb
}

// WithAuthor sets the post author
func (spsb *SocialPostScenarioBuilder) WithAuthor(identity string, alias *string, name *string) *SocialPostScenarioBuilder {
	spsb.content.Author = ContentAuthor{
		Identity: identity,
		Alias:    alias,
		Name:     name,
	}
	return spsb
}

// WithImages adds image URLs
func (spsb *SocialPostScenarioBuilder) WithImages(urls ...string) *SocialPostScenarioBuilder {
	spsb.content.ImageURLs = append(spsb.content.ImageURLs, urls...)
	return spsb
}

// WithTags adds hashtags/tags
func (spsb *SocialPostScenarioBuilder) WithTags(tags ...string) *SocialPostScenarioBuilder {
	spsb.content.Tags = append(spsb.content.Tags, tags...)
	return spsb
}

// WithEngagement sets engagement metrics
func (spsb *SocialPostScenarioBuilder) WithEngagement(likes, shares, comments int) *SocialPostScenarioBuilder {
	spsb.content.Likes = likes
	spsb.content.Shares = shares
	spsb.content.Comments = comments
	return spsb
}

// WithTimestamp sets the post timestamp
func (spsb *SocialPostScenarioBuilder) WithTimestamp(t time.Time) *SocialPostScenarioBuilder {
	spsb.content.CreatedAt = t
	return spsb
}

// WithContext adds scenario context
func (spsb *SocialPostScenarioBuilder) WithContext(key string, value interface{}) *SocialPostScenarioBuilder {
	spsb.builder.WithContext(key, value)
	return spsb
}

// WithPersonalityExpectations adds personality expectations
func (spsb *SocialPostScenarioBuilder) WithPersonalityExpectations(expectations ...PersonalityExpectedOutcome) *SocialPostScenarioBuilder {
	spsb.builder.WithPersonalityExpectations(expectations...)
	return spsb
}

// ExpectPersonality adds a single personality expectation
func (spsb *SocialPostScenarioBuilder) ExpectPersonality(name string, shouldShow bool, confidence float64, priority int, rationale string) *SocialPostScenarioBuilder {
	expectation := PersonalityExpectedOutcome{
		PersonalityName: name,
		ShouldShow:      shouldShow,
		Confidence:      confidence,
		Priority:        priority,
		Rationale:       rationale,
		ExpectedState:   "visible",
	}
	if !shouldShow {
		expectation.ExpectedState = "hidden"
	}
	return spsb.WithPersonalityExpectations(expectation)
}

// Build creates the final scenario
func (spsb *SocialPostScenarioBuilder) Build(framework *PersonalityTestFramework) *GenericTestScenario {
	// Set up evaluation handler
	handler := NewSocialPostEvaluationHandler(framework)
	spsb.builder.WithEvaluationHandler(handler)
	
	return spsb.builder.Build()
}

// ===== BACKWARD COMPATIBILITY HELPERS =====

// ThreadScenarioBuilder provides backward compatibility for existing thread scenarios
type ThreadScenarioBuilder struct {
	builder *GenericScenarioBuilder
	content *ThreadContent
}

// NewThreadScenario creates a thread scenario builder for backward compatibility
func NewThreadScenario(name, description string) *ThreadScenarioBuilder {
	content := &ThreadContent{
		ThreadData: ThreadData{
			CreatedAt: time.Now(),
		},
	}
	
	builder := NewGenericScenarioBuilder(name, description, ScenarioTypeThread)
	builder.WithContent(content)
	
	return &ThreadScenarioBuilder{
		builder: builder,
		content: content,
	}
}

// WithThread sets the thread content (backward compatibility)
func (tsb *ThreadScenarioBuilder) WithThread(title, content, authorName string) *ThreadScenarioBuilder {
	tsb.content.ThreadData.Title = title
	tsb.content.ThreadData.Content = content
	tsb.content.ThreadData.AuthorName = authorName
	return tsb
}

// WithAuthor sets the thread author details
func (tsb *ThreadScenarioBuilder) WithAuthor(name string, alias *string) *ThreadScenarioBuilder {
	tsb.content.ThreadData.AuthorName = name
	tsb.content.ThreadData.AuthorAlias = alias
	return tsb
}

// WithImages adds image URLs to the thread
func (tsb *ThreadScenarioBuilder) WithImages(urls ...string) *ThreadScenarioBuilder {
	tsb.content.ThreadData.ImageURLs = append(tsb.content.ThreadData.ImageURLs, urls...)
	return tsb
}

// WithMessage adds a message to the thread
func (tsb *ThreadScenarioBuilder) WithMessage(authorName, content string, alias *string) *ThreadScenarioBuilder {
	message := ThreadMessageData{
		AuthorName:  authorName,
		AuthorAlias: alias,
		Content:     content,
		CreatedAt:   time.Now(),
	}
	tsb.content.ThreadData.Messages = append(tsb.content.ThreadData.Messages, message)
	tsb.content.Messages = append(tsb.content.Messages, message)
	return tsb
}

// WithContext adds scenario context
func (tsb *ThreadScenarioBuilder) WithContext(key string, value interface{}) *ThreadScenarioBuilder {
	tsb.builder.WithContext(key, value)
	return tsb
}

// WithPersonalityExpectations adds personality expectations
func (tsb *ThreadScenarioBuilder) WithPersonalityExpectations(expectations ...PersonalityExpectedOutcome) *ThreadScenarioBuilder {
	tsb.builder.WithPersonalityExpectations(expectations...)
	return tsb
}

// Build creates the final scenario
func (tsb *ThreadScenarioBuilder) Build(framework *PersonalityTestFramework) *GenericTestScenario {
	// Create the thread model from thread data
	tsb.content.Thread = framework.createThreadFromData(tsb.content.ThreadData)
	
	// Set up evaluation handler
	handler := NewThreadEvaluationHandler(framework)
	tsb.builder.WithEvaluationHandler(handler)
	
	return tsb.builder.Build()
}

// Helper function for string pointers
func stringPtr(s string) *string {
	return &s
}