package personalities

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/EternisAI/enchanted-twin/graph/model"
)

// ScenarioType represents different types of scenarios
type ScenarioType string

const (
	ScenarioTypeThread      ScenarioType = "thread"
	ScenarioTypeChatMessage ScenarioType = "chat_message"
	ScenarioTypeEmail       ScenarioType = "email"
	ScenarioTypeSocialPost  ScenarioType = "social_post"
	ScenarioTypeNewsArticle ScenarioType = "news_article"
	ScenarioTypeGeneric     ScenarioType = "generic"
)

// BaseScenario defines the common interface for all scenario types
type BaseScenario interface {
	// Core scenario information
	GetName() string
	GetDescription() string
	GetType() ScenarioType
	GetContext() map[string]interface{}

	// Content and evaluation
	GetContent() ScenarioContent
	GetExpectedOutcomes() []PersonalityExpectedOutcome
	GetExpectedOutcomeForPersonality(personalityName string, extensionNames []string) *PersonalityExpectedOutcome

	// Evaluation
	Evaluate(ctx context.Context, personality *ReferencePersonality, env *TestEnvironment) (*GenericEvaluationResult, error)
}

// ScenarioContent represents the content of any scenario type
type ScenarioContent interface {
	// Basic content information
	GetContentType() ScenarioType
	GetMainText() string
	GetAuthor() *ContentAuthor
	GetCreatedAt() time.Time
	GetMetadata() map[string]interface{}

	// For display and analysis
	GetDisplayTitle() string
	GetDisplaySummary() string
	GetKeywords() []string
}

// ContentAuthor represents the author of any content
type ContentAuthor struct {
	Identity string  `json:"identity"`
	Alias    *string `json:"alias,omitempty"`
	Name     *string `json:"name,omitempty"`
	Email    *string `json:"email,omitempty"`
}

// GenericEvaluationResult represents the result of evaluating any scenario type
type GenericEvaluationResult struct {
	ShouldShow bool                   `json:"should_show"`
	Reason     string                 `json:"reason"`
	Confidence float64                `json:"confidence"`
	NewState   string                 `json:"new_state"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// GenericTestScenario is the new flexible scenario structure
type GenericTestScenario struct {
	Name                    string                       `json:"name"`
	Description             string                       `json:"description"`
	Type                    ScenarioType                 `json:"type"`
	Content                 ScenarioContent              `json:"content"`
	Context                 map[string]interface{}       `json:"context"`
	PersonalityExpectations []PersonalityExpectedOutcome `json:"personality_expectations"`
	DefaultExpected         *ExpectedThreadEvaluation    `json:"default_expected,omitempty"`

	// For evaluation
	EvaluationHandler EvaluationHandler `json:"-"` // Not serialized
}

// EvaluationHandler defines how to evaluate different content types
type EvaluationHandler interface {
	Evaluate(ctx context.Context, content ScenarioContent, personality *ReferencePersonality, env *TestEnvironment) (*GenericEvaluationResult, error)
	GetSupportedType() ScenarioType
}

// Implementation of BaseScenario interface
func (gts *GenericTestScenario) GetName() string {
	return gts.Name
}

func (gts *GenericTestScenario) GetDescription() string {
	return gts.Description
}

func (gts *GenericTestScenario) GetType() ScenarioType {
	return gts.Type
}

func (gts *GenericTestScenario) GetContext() map[string]interface{} {
	return gts.Context
}

func (gts *GenericTestScenario) GetContent() ScenarioContent {
	return gts.Content
}

func (gts *GenericTestScenario) GetExpectedOutcomes() []PersonalityExpectedOutcome {
	return gts.PersonalityExpectations
}

func (gts *GenericTestScenario) GetExpectedOutcomeForPersonality(personalityName string, extensionNames []string) *PersonalityExpectedOutcome {
	// First try to find exact match with extensions
	if len(extensionNames) > 0 {
		for _, outcome := range gts.PersonalityExpectations {
			if outcome.PersonalityName == personalityName &&
				len(outcome.ExtensionNames) == len(extensionNames) &&
				stringSlicesEqual(outcome.ExtensionNames, extensionNames) {
				return &outcome
			}
		}
	}

	// Then try to find base personality match (no extensions)
	for _, outcome := range gts.PersonalityExpectations {
		if outcome.PersonalityName == personalityName && len(outcome.ExtensionNames) == 0 {
			return &outcome
		}
	}

	return nil
}

func (gts *GenericTestScenario) Evaluate(ctx context.Context, personality *ReferencePersonality, env *TestEnvironment) (*GenericEvaluationResult, error) {
	if gts.EvaluationHandler == nil {
		return nil, fmt.Errorf("no evaluation handler configured for scenario type: %s", gts.Type)
	}

	return gts.EvaluationHandler.Evaluate(ctx, gts.Content, personality, env)
}

// Custom JSON unmarshaling for GenericTestScenario
func (gts *GenericTestScenario) UnmarshalJSON(data []byte) error {
	// First unmarshal into a temporary struct with raw content
	var temp struct {
		Name                    string                       `json:"name"`
		Description             string                       `json:"description"`
		Type                    ScenarioType                 `json:"type"`
		Content                 json.RawMessage              `json:"content"`
		Context                 map[string]interface{}       `json:"context"`
		PersonalityExpectations []PersonalityExpectedOutcome `json:"personality_expectations"`
		DefaultExpected         *ExpectedThreadEvaluation    `json:"default_expected,omitempty"`
	}

	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	// Set the basic fields
	gts.Name = temp.Name
	gts.Description = temp.Description
	gts.Type = temp.Type
	gts.Context = temp.Context
	gts.PersonalityExpectations = temp.PersonalityExpectations
	gts.DefaultExpected = temp.DefaultExpected

	// Now unmarshal the content based on the type
	var content ScenarioContent
	switch temp.Type {
	case ScenarioTypeChatMessage:
		var chatContent ChatMessageContent
		if err := json.Unmarshal(temp.Content, &chatContent); err != nil {
			return fmt.Errorf("failed to unmarshal chat message content: %w", err)
		}
		content = &chatContent

	case ScenarioTypeEmail:
		var emailContent EmailContent
		if err := json.Unmarshal(temp.Content, &emailContent); err != nil {
			return fmt.Errorf("failed to unmarshal email content: %w", err)
		}
		content = &emailContent

	case ScenarioTypeSocialPost:
		var socialContent SocialPostContent
		if err := json.Unmarshal(temp.Content, &socialContent); err != nil {
			return fmt.Errorf("failed to unmarshal social post content: %w", err)
		}
		content = &socialContent

	case ScenarioTypeThread:
		var threadContent ThreadContent
		if err := json.Unmarshal(temp.Content, &threadContent); err != nil {
			return fmt.Errorf("failed to unmarshal thread content: %w", err)
		}
		content = &threadContent

	case ScenarioTypeNewsArticle:
		var newsContent NewsArticleContent
		if err := json.Unmarshal(temp.Content, &newsContent); err != nil {
			return fmt.Errorf("failed to unmarshal news article content: %w", err)
		}
		content = &newsContent

	case ScenarioTypeGeneric:
		var genericContent GenericContent
		if err := json.Unmarshal(temp.Content, &genericContent); err != nil {
			return fmt.Errorf("failed to unmarshal generic content: %w", err)
		}
		content = &genericContent

	default:
		return fmt.Errorf("unsupported scenario type: %s", temp.Type)
	}

	gts.Content = content
	return nil
}

// ===== SPECIFIC CONTENT TYPES =====

// ThreadContent represents holon thread content (backward compatibility)
type ThreadContent struct {
	Thread     *model.Thread       `json:"thread"`
	ThreadData ThreadData          `json:"thread_data"`
	Messages   []ThreadMessageData `json:"messages,omitempty"`
}

func (tc *ThreadContent) GetContentType() ScenarioType {
	return ScenarioTypeThread
}

func (tc *ThreadContent) GetMainText() string {
	if tc.Thread != nil {
		return tc.Thread.Content
	}
	return tc.ThreadData.Content
}

func (tc *ThreadContent) GetAuthor() *ContentAuthor {
	if tc.Thread != nil && tc.Thread.Author != nil {
		return &ContentAuthor{
			Identity: tc.Thread.Author.Identity,
			Alias:    tc.Thread.Author.Alias,
		}
	}
	return &ContentAuthor{
		Identity: tc.ThreadData.AuthorName,
		Alias:    tc.ThreadData.AuthorAlias,
	}
}

func (tc *ThreadContent) GetCreatedAt() time.Time {
	if tc.Thread != nil {
		if createdAt, err := time.Parse(time.RFC3339, tc.Thread.CreatedAt); err == nil {
			return createdAt
		}
	}
	return tc.ThreadData.CreatedAt
}

func (tc *ThreadContent) GetMetadata() map[string]interface{} {
	metadata := make(map[string]interface{})
	if tc.Thread != nil {
		metadata["thread_id"] = tc.Thread.ID
		metadata["title"] = tc.Thread.Title
		metadata["image_urls"] = tc.Thread.ImageURLs
		metadata["actions"] = tc.Thread.Actions
	} else {
		metadata["title"] = tc.ThreadData.Title
		metadata["image_urls"] = tc.ThreadData.ImageURLs
	}
	metadata["message_count"] = len(tc.Messages)
	return metadata
}

func (tc *ThreadContent) GetDisplayTitle() string {
	if tc.Thread != nil {
		return tc.Thread.Title
	}
	return tc.ThreadData.Title
}

func (tc *ThreadContent) GetDisplaySummary() string {
	content := tc.GetMainText()
	if len(content) > 150 {
		return content[:147] + "..."
	}
	return content
}

func (tc *ThreadContent) GetKeywords() []string {
	// Extract keywords from title and content
	var keywords []string
	title := tc.GetDisplayTitle()
	content := tc.GetMainText()

	// Simple keyword extraction (could be enhanced with NLP)
	text := title + " " + content
	words := strings.Fields(strings.ToLower(text))

	wordCount := make(map[string]int)
	for _, word := range words {
		word = strings.Trim(word, ".,!?;:")
		if len(word) > 3 { // Only include longer words
			wordCount[word]++
		}
	}

	// Return most frequent words as keywords
	for word, count := range wordCount {
		if count >= 2 || len(word) > 6 {
			keywords = append(keywords, word)
		}
	}

	return keywords
}

// ChatMessageContent represents a chat message scenario
type ChatMessageContent struct {
	MessageID   string                 `json:"message_id"`
	Content     string                 `json:"content"` // Main field for content
	Text        string                 `json:"text"`    // Legacy field for backward compatibility
	Author      ContentAuthor          `json:"author"`
	CreatedAt   time.Time              `json:"created_at"`
	ChatContext string                 `json:"chat_context,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// GetText returns Text if set, otherwise falls back to Content for backward compatibility
func (cmc *ChatMessageContent) GetText() string {
	if cmc.Text != "" {
		return cmc.Text
	}
	return cmc.Content
}

func (cmc *ChatMessageContent) GetContentType() ScenarioType {
	return ScenarioTypeChatMessage
}

func (cmc *ChatMessageContent) GetMainText() string {
	return cmc.GetText()
}

func (cmc *ChatMessageContent) GetAuthor() *ContentAuthor {
	return &cmc.Author
}

func (cmc *ChatMessageContent) GetCreatedAt() time.Time {
	return cmc.CreatedAt
}

func (cmc *ChatMessageContent) GetMetadata() map[string]interface{} {
	metadata := make(map[string]interface{})
	if cmc.Metadata != nil {
		for k, v := range cmc.Metadata {
			metadata[k] = v
		}
	}
	metadata["message_id"] = cmc.MessageID
	metadata["chat_context"] = cmc.ChatContext
	return metadata
}

func (cmc *ChatMessageContent) GetDisplayTitle() string {
	if cmc.ChatContext != "" {
		return fmt.Sprintf("Message in %s", cmc.ChatContext)
	}
	return "Chat Message"
}

func (cmc *ChatMessageContent) GetDisplaySummary() string {
	text := cmc.GetText()
	if len(text) > 100 {
		return text[:97] + "..."
	}
	return text
}

func (cmc *ChatMessageContent) GetKeywords() []string {
	words := strings.Fields(strings.ToLower(cmc.GetText()))
	var keywords []string

	for _, word := range words {
		word = strings.Trim(word, ".,!?;:")
		if len(word) > 3 {
			keywords = append(keywords, word)
		}
	}

	return keywords
}

// EmailContent represents an email scenario
type EmailContent struct {
	Subject   string                 `json:"subject"`
	Body      string                 `json:"body"`
	From      ContentAuthor          `json:"from"`
	To        []ContentAuthor        `json:"to"`
	CC        []ContentAuthor        `json:"cc,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	Priority  string                 `json:"priority,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

func (ec *EmailContent) GetContentType() ScenarioType {
	return ScenarioTypeEmail
}

func (ec *EmailContent) GetMainText() string {
	return ec.Body
}

func (ec *EmailContent) GetAuthor() *ContentAuthor {
	return &ec.From
}

func (ec *EmailContent) GetCreatedAt() time.Time {
	return ec.CreatedAt
}

func (ec *EmailContent) GetMetadata() map[string]interface{} {
	metadata := make(map[string]interface{})
	if ec.Metadata != nil {
		for k, v := range ec.Metadata {
			metadata[k] = v
		}
	}
	metadata["subject"] = ec.Subject
	metadata["priority"] = ec.Priority
	metadata["recipient_count"] = len(ec.To)
	metadata["cc_count"] = len(ec.CC)
	return metadata
}

func (ec *EmailContent) GetDisplayTitle() string {
	return ec.Subject
}

func (ec *EmailContent) GetDisplaySummary() string {
	if len(ec.Body) > 150 {
		return ec.Body[:147] + "..."
	}
	return ec.Body
}

func (ec *EmailContent) GetKeywords() []string {
	text := ec.Subject + " " + ec.Body
	words := strings.Fields(strings.ToLower(text))
	var keywords []string

	wordCount := make(map[string]int)
	for _, word := range words {
		word = strings.Trim(word, ".,!?;:")
		if len(word) > 3 {
			wordCount[word]++
		}
	}

	for word, count := range wordCount {
		if count >= 2 || len(word) > 6 {
			keywords = append(keywords, word)
		}
	}

	return keywords
}

// SocialPostContent represents a social media post scenario
type SocialPostContent struct {
	PostID    string                 `json:"post_id"`
	Text      string                 `json:"text"`
	Author    ContentAuthor          `json:"author"`
	Platform  string                 `json:"platform"` // "twitter", "facebook", "linkedin", etc.
	CreatedAt time.Time              `json:"created_at"`
	ImageURLs []string               `json:"image_urls,omitempty"`
	Tags      []string               `json:"tags,omitempty"`
	Likes     int                    `json:"likes,omitempty"`
	Shares    int                    `json:"shares,omitempty"`
	Comments  int                    `json:"comments,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

func (spc *SocialPostContent) GetContentType() ScenarioType {
	return ScenarioTypeSocialPost
}

func (spc *SocialPostContent) GetMainText() string {
	return spc.Text
}

func (spc *SocialPostContent) GetAuthor() *ContentAuthor {
	return &spc.Author
}

func (spc *SocialPostContent) GetCreatedAt() time.Time {
	return spc.CreatedAt
}

func (spc *SocialPostContent) GetMetadata() map[string]interface{} {
	metadata := make(map[string]interface{})
	if spc.Metadata != nil {
		for k, v := range spc.Metadata {
			metadata[k] = v
		}
	}
	metadata["post_id"] = spc.PostID
	metadata["platform"] = spc.Platform
	metadata["likes"] = spc.Likes
	metadata["shares"] = spc.Shares
	metadata["comments"] = spc.Comments
	metadata["tags"] = spc.Tags
	metadata["image_count"] = len(spc.ImageURLs)
	return metadata
}

func (spc *SocialPostContent) GetDisplayTitle() string {
	return fmt.Sprintf("%s Post", cases.Title(language.English).String(spc.Platform))
}

func (spc *SocialPostContent) GetDisplaySummary() string {
	if len(spc.Text) > 120 {
		return spc.Text[:117] + "..."
	}
	return spc.Text
}

func (spc *SocialPostContent) GetKeywords() []string {
	// Include both content keywords and tags
	var keywords []string

	// Add tags as keywords
	keywords = append(keywords, spc.Tags...)

	// Extract keywords from text
	words := strings.Fields(strings.ToLower(spc.Text))
	for _, word := range words {
		word = strings.Trim(word, ".,!?;:")
		if len(word) > 3 {
			keywords = append(keywords, word)
		}
	}

	return keywords
}

// NewsArticleContent represents a news article scenario
type NewsArticleContent struct {
	ArticleID   string                 `json:"article_id"`
	Headline    string                 `json:"headline"`
	Body        string                 `json:"body"`
	Author      ContentAuthor          `json:"author"`
	Publication string                 `json:"publication"`
	Category    string                 `json:"category,omitempty"` // "technology", "business", "science", etc.
	CreatedAt   time.Time              `json:"created_at"`
	Tags        []string               `json:"tags,omitempty"`
	ImageURLs   []string               `json:"image_urls,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

func (nac *NewsArticleContent) GetContentType() ScenarioType {
	return ScenarioTypeNewsArticle
}

func (nac *NewsArticleContent) GetMainText() string {
	return nac.Body
}

func (nac *NewsArticleContent) GetAuthor() *ContentAuthor {
	return &nac.Author
}

func (nac *NewsArticleContent) GetCreatedAt() time.Time {
	return nac.CreatedAt
}

func (nac *NewsArticleContent) GetMetadata() map[string]interface{} {
	metadata := make(map[string]interface{})
	if nac.Metadata != nil {
		for k, v := range nac.Metadata {
			metadata[k] = v
		}
	}
	metadata["article_id"] = nac.ArticleID
	metadata["publication"] = nac.Publication
	metadata["category"] = nac.Category
	metadata["tags"] = nac.Tags
	metadata["image_count"] = len(nac.ImageURLs)
	return metadata
}

func (nac *NewsArticleContent) GetDisplayTitle() string {
	return nac.Headline
}

func (nac *NewsArticleContent) GetDisplaySummary() string {
	if len(nac.Body) > 200 {
		return nac.Body[:197] + "..."
	}
	return nac.Body
}

func (nac *NewsArticleContent) GetKeywords() []string {
	// Extract keywords from headline and body
	var keywords []string

	// Add tags as keywords
	keywords = append(keywords, nac.Tags...)

	// Extract keywords from text
	text := nac.Headline + " " + nac.Body
	words := strings.Fields(strings.ToLower(text))

	wordCount := make(map[string]int)
	for _, word := range words {
		word = strings.Trim(word, ".,!?;:")
		if len(word) > 3 {
			wordCount[word]++
		}
	}

	for word, count := range wordCount {
		if count >= 2 || len(word) > 6 {
			keywords = append(keywords, word)
		}
	}

	return keywords
}

// GenericContent represents a generic content scenario that can adapt to various content types
type GenericContent struct {
	ContentID   string                 `json:"content_id"`
	Title       string                 `json:"title"`
	Body        string                 `json:"body"`
	Author      ContentAuthor          `json:"author"`
	ContentType string                 `json:"content_type"` // "article", "blog_post", "document", etc.
	CreatedAt   time.Time              `json:"created_at"`
	Tags        []string               `json:"tags,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

func (gc *GenericContent) GetContentType() ScenarioType {
	return ScenarioTypeGeneric
}

func (gc *GenericContent) GetMainText() string {
	return gc.Body
}

func (gc *GenericContent) GetAuthor() *ContentAuthor {
	return &gc.Author
}

func (gc *GenericContent) GetCreatedAt() time.Time {
	return gc.CreatedAt
}

func (gc *GenericContent) GetMetadata() map[string]interface{} {
	metadata := make(map[string]interface{})
	if gc.Metadata != nil {
		for k, v := range gc.Metadata {
			metadata[k] = v
		}
	}
	metadata["content_id"] = gc.ContentID
	metadata["content_type"] = gc.ContentType
	metadata["tags"] = gc.Tags
	return metadata
}

func (gc *GenericContent) GetDisplayTitle() string {
	return gc.Title
}

func (gc *GenericContent) GetDisplaySummary() string {
	if len(gc.Body) > 150 {
		return gc.Body[:147] + "..."
	}
	return gc.Body
}

func (gc *GenericContent) GetKeywords() []string {
	// Extract keywords from title and body
	var keywords []string

	// Add tags as keywords
	keywords = append(keywords, gc.Tags...)

	// Extract keywords from text
	text := gc.Title + " " + gc.Body
	words := strings.Fields(strings.ToLower(text))

	wordCount := make(map[string]int)
	for _, word := range words {
		word = strings.Trim(word, ".,!?;:")
		if len(word) > 3 {
			wordCount[word]++
		}
	}

	for word, count := range wordCount {
		if count >= 2 || len(word) > 6 {
			keywords = append(keywords, word)
		}
	}

	return keywords
}
