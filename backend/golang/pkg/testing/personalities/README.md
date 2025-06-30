# Personality Testing Framework

A comprehensive testing framework for modeling reference personalities and testing both holon thread processing functionality and arbitrary content scenarios. This framework allows you to create detailed personality profiles with rich memory facts, then test how different personalities respond to various content types including chat messages, emails, social media posts, and traditional thread scenarios using LLM-based evaluation.

## Overview

The personality testing framework consists of several key components:

- **Reference Personalities**: Detailed personality profiles with memory facts, conversations, and expected behaviors
- **Flexible Scenarios**: Test cases supporting multiple content types (chat messages, emails, social posts, threads)
- **Thread Scenarios**: Legacy thread-based test cases for holon processing
- **LLM-as-a-Judge**: Automated evaluation of how well actual results match expected personality behaviors
- **Memory Tracking**: Analysis of which memories are accessed during evaluation
- **Comprehensive Reporting**: Detailed analysis and scoring of personality performance

## Architecture

```
PersonalityTestFramework
├── framework.go              # Core data structures and loading logic
├── flexible_scenarios.go     # New flexible scenario system for arbitrary content
├── flexible_handlers.go      # Evaluation handlers for different content types
├── flexible_builders.go      # Fluent builders for different scenario types
├── scenario_builders.go      # Legacy thread scenario builders (maintained for compatibility)
├── runner.go                 # Test execution and LLM judge evaluation
├── reporting.go              # Report generation and analytics
├── modular_test.go          # Tests for modular personality system
└── personality_test.go       # Integration tests demonstrating both systems
```

## New Flexible Scenario System

The framework now supports testing personalities against different types of content beyond just holon threads:

### Supported Content Types

1. **Chat Messages** (`ScenarioTypeChatMessage`)
   - Direct messages, group chats, team discussions
   - Context-aware evaluation based on chat environment
   - Author identity and message threading support

2. **Emails** (`ScenarioTypeEmail`)
   - Business emails, newsletters, notifications
   - Priority-based filtering, sender reputation
   - Subject line and body content analysis

3. **Social Media Posts** (`ScenarioTypeSocialPost`)
   - LinkedIn, Twitter, Instagram posts
   - Engagement metrics, hashtags, platform-specific behavior
   - Visual content and viral content patterns

4. **Threads** (`ScenarioTypeThread`) 
   - Traditional holon thread scenarios (backward compatible)
   - Multi-message discussions with replies
   - Integration with existing holon processing system

### Content-Specific Evaluation

Each content type has specialized evaluation handlers that consider:

- **Chat Messages**: Conversation context, urgency, social dynamics
- **Emails**: Business relevance, sender authority, call-to-action strength
- **Social Posts**: Platform norms, engagement potential, trend relevance
- **Threads**: Topic depth, discussion quality, community value

## Usage

### 1. Flexible Scenario Creation

#### Chat Message Scenarios
```go
scenario := NewChatMessageScenario("ai_discussion_chat", "Chat about AI development").
    WithMessage("GPT-5 just got released and the performance improvements are incredible!", "tech_enthusiast").
    WithAuthor("tech_enthusiast", stringPtr("Alex"), stringPtr("Alex Chen")).
    WithChatContext("Tech Innovation Discussion").
    WithContext("domain", "artificial_intelligence").
    ExpectPersonality("tech_entrepreneur", true, 0.9, 3, "Tech entrepreneurs are highly interested in AI developments").
    ExpectPersonality("creative_artist", true, 0.6, 2, "Creative artists have moderate interest in AI tools").
    Build(framework)
```

#### Email Scenarios
```go
scenario := NewEmailScenario("business_proposal", "Business proposal email").
    WithEmail(
        "Series B Funding Opportunity - AI Infrastructure Startup", 
        "Hi, we're raising $25M Series B for our AI infrastructure startup...",
    ).
    WithFrom("founder", stringPtr("John Doe"), stringPtr("John Doe"), stringPtr("john@startup.ai")).
    WithTo(ContentAuthor{Identity: "investor", Name: stringPtr("VC Partner")}).
    WithPriority("high").
    WithContext("domain", "venture_capital").
    ExpectPersonality("tech_entrepreneur", true, 0.95, 3, "Tech entrepreneurs are extremely interested in investment opportunities").
    Build(framework)
```

#### Social Media Scenarios
```go
scenario := NewSocialPostScenario("instagram_art_post", "Instagram art showcase").
    WithPost(
        "🎨 New digital painting exploring AI-human collaboration! #AIArt #DigitalPainting", 
        "instagram",
    ).
    WithAuthor("digital_artist", stringPtr("@maya_creates"), stringPtr("Maya Rodriguez")).
    WithImages("https://example.com/art-piece.jpg").
    WithTags("AIArt", "DigitalPainting", "TechArt").
    WithEngagement(542, 89, 127).
    WithContext("domain", "creative_arts").
    ExpectPersonality("creative_artist", true, 0.95, 3, "Creative artists highly engage with artistic content").
    Build(framework)
```

### 2. Backward Compatible Thread Scenarios

The legacy thread system continues to work exactly as before:

```go
scenario := NewThreadScenario("ai_breakthrough", "AI breakthrough discussion").
    WithThread(
        "GPT-5 Achieves 95% Accuracy on Complex Reasoning Tasks",
        "New research shows unprecedented capabilities...",
        "ai_researcher",
    ).
    WithAuthor("ai_researcher", stringPtr("Dr. Sarah Chen")).
    WithMessage("tech_lead", "This changes everything for our product roadmap!", stringPtr("Alex Kim")).
    WithContext("domain", "artificial_intelligence").
    Build(framework)
```

### 3. Running Flexible Tests

```go
// Load personalities and flexible scenarios
err := framework.LoadBasePersonalities()
err = framework.LoadGenericScenarios()

// Run flexible personality tests
results, err := framework.RunFlexiblePersonalityTests(ctx, memoryStorage, holonRepo)

// Generate comprehensive report
report := framework.GenerateReport(results)
framework.PrintSummary(report)
framework.SaveReport(report, "flexible_test_report.json")
```

## Test Data Structure

```
testdata/
├── personalities/                    # Personality definitions
│   ├── tech_entrepreneur/
│   │   └── personality.json
│   ├── creative_artist/
│   │   └── personality.json
│   └── {personality_name}/
│       └── personality.json
├── scenarios/                        # Legacy thread scenarios
│   ├── ai_breakthrough_news.json
│   ├── creative_tool_announcement.json
│   └── {scenario_name}.json
├── generic_scenarios/                # New flexible scenarios (optional)
│   ├── chat_messages/
│   ├── emails/
│   ├── social_posts/
│   └── mixed/
├── extensions/                       # Personality extensions (optional)
│   ├── tech_entrepreneur/
│   │   ├── ai_research_focused.json
│   │   └── startup_ecosystem_focused.json
│   └── creative_artist/
│       └── creative_tools_focused.json
└── reports/
    ├── personality_test_report_{timestamp}.json
    └── flexible_scenarios_report_{timestamp}.json
```

## Content Type Specifications

### ChatMessageContent
```go
type ChatMessageContent struct {
    MessageID    string                 `json:"message_id"`
    Text         string                 `json:"text"`
    Author       ContentAuthor          `json:"author"`
    ChatContext  string                 `json:"chat_context"`
    CreatedAt    time.Time             `json:"created_at"`
    Metadata     map[string]interface{} `json:"metadata"`
}
```

### EmailContent
```go
type EmailContent struct {
    Subject   string          `json:"subject"`
    Body      string          `json:"body"`
    From      ContentAuthor   `json:"from"`
    To        []ContentAuthor `json:"to"`
    CC        []ContentAuthor `json:"cc"`
    Priority  string          `json:"priority"`
    CreatedAt time.Time       `json:"created_at"`
    Metadata  map[string]interface{} `json:"metadata"`
}
```

### SocialPostContent
```go
type SocialPostContent struct {
    PostID    string          `json:"post_id"`
    Text      string          `json:"text"`
    Author    ContentAuthor   `json:"author"`
    Platform  string          `json:"platform"`
    ImageURLs []string        `json:"image_urls"`
    Tags      []string        `json:"tags"`
    Likes     int             `json:"likes"`
    Shares    int             `json:"shares"`
    Comments  int             `json:"comments"`
    CreatedAt time.Time       `json:"created_at"`
    Metadata  map[string]interface{} `json:"metadata"`
}
```

## Evaluation Handlers

Each content type has a specialized evaluation handler:

### Chat Message Handler
- Analyzes conversational context and social dynamics
- Considers message urgency and relevance to ongoing discussions
- Evaluates based on relationship with message author
- Factors in group dynamics and chat environment

### Email Handler
- Prioritizes based on sender authority and email priority
- Analyzes subject line relevance and call-to-action strength
- Considers business context and professional relationships
- Evaluates time-sensitivity and response requirements

### Social Post Handler
- Considers platform-specific norms and engagement patterns
- Analyzes hashtag relevance and trending topics
- Evaluates visual content and shareability potential
- Factors in follower relationships and social proof

## Key Features

### Content-Agnostic Framework
- **Unified Interface**: Same evaluation framework works across all content types
- **Pluggable Handlers**: Easy to add new content types and evaluation logic
- **Consistent Reporting**: Same metrics and analysis across different content scenarios

### Enhanced Realism
- **Platform Behavior**: Different evaluation logic for LinkedIn vs Instagram vs Email
- **Context Awareness**: Chat context, email threads, social engagement influence decisions
- **Multi-Modal Support**: Text, images, engagement metrics, metadata all factor into evaluation

### Backward Compatibility
- **Legacy Support**: All existing thread scenarios continue to work unchanged
- **Gradual Migration**: Can use both systems simultaneously during transition
- **Unified Reporting**: Legacy and flexible scenarios appear in same reports

## Sample Scenarios Included

### Chat Message Scenarios
- **AI Discussion**: Tech enthusiasts discussing AI breakthroughs in team chat
- **Creative Collaboration**: Artists sharing creative process in collaboration channel
- **Casual Personal**: Friends planning coffee meetup in group chat

### Email Scenarios
- **Business Proposal**: High-priority funding opportunity from startup founder
- **Creative Opportunity**: Art exhibition invitation with professional opportunity
- **Newsletter**: Technical newsletter with industry insights

### Social Media Scenarios
- **LinkedIn Tech Post**: AI researcher sharing research breakthrough with engagement metrics
- **Instagram Art Post**: Digital artist showcasing AI-human collaborative artwork
- **Twitter Opinion**: Tech commentator sharing hot take on AI industry trends

### Thread Scenarios (Legacy)
- **AI Breakthrough News**: Technical AI model improvements with metrics
- **Creative Tool Announcement**: New digital art tool with creative features  
- **Celebrity Gossip**: Entertainment news with no practical value
- **Startup Funding**: Major funding announcements and market implications

## Migration Guide

### For Existing Users
1. **No Changes Required**: Your existing thread scenarios continue to work exactly as before
2. **Gradual Enhancement**: Add flexible scenarios alongside existing ones
3. **Unified Testing**: Use `RunFlexiblePersonalityTests()` for new scenarios, `RunPersonalityTests()` for legacy

### For New Users
1. **Start with Flexible**: Use the new content-specific builders for realistic scenarios
2. **Content Diversity**: Create scenarios across different content types for comprehensive testing
3. **Platform-Specific**: Consider platform norms and user behavior patterns

## Running the Tests

### Prerequisites
- Go 1.21+
- OpenAI API key for LLM evaluation
- Weaviate or compatible vector database (for full integration)

### Quick Start - Flexible Scenarios
```bash
# Run flexible scenario tests
cd pkg/testing/personalities
go test -v -run TestFlexibleScenarioIntegration

# Run both legacy and flexible systems
go test -v -run TestPersonalityThreadProcessingIntegration
go test -v -run TestFlexibleScenarioSystem

# View generated reports
ls testdata/reports/
```

### Environment Setup
```bash
export COMPLETIONS_API_KEY="your-openai-api-key"
export COMPLETIONS_API_URL="https://api.openai.com/v1"
export EMBEDDINGS_API_KEY="your-openai-api-key"  # Optional, defaults to completions key
```

## Use Cases

### Development Testing
- **Content Processing**: Test how personalities respond to different content types
- **Platform Integration**: Validate behavior across chat, email, social platforms
- **Multi-Modal Scenarios**: Test responses to text + image + engagement combinations

### Product Development
- **Feed Algorithms**: Test content recommendation systems across platforms
- **Notification Systems**: Validate email and push notification relevance
- **Social Features**: Test engagement prediction and viral content identification

### Research Applications
- **Cross-Platform Behavior**: Study how personality traits manifest across different platforms
- **Content Preference**: Analyze content type preferences by personality segments
- **Communication Patterns**: Research how personalities respond to different communication channels

## Future Enhancements

### New Content Types
- **Video Content**: YouTube videos, TikToks, video calls with transcript analysis
- **Audio Content**: Podcasts, voice messages, meeting recordings
- **Documents**: PDFs, presentations, research papers with domain-specific evaluation
- **Code**: GitHub repositories, pull requests, technical documentation

### Advanced Features
- **Real-Time Evaluation**: Stream processing for live content evaluation
- **Multi-User Scenarios**: Group conversations, collaborative documents
- **Temporal Analysis**: Track personality evolution across content types over time
- **A/B Testing**: Compare different evaluation strategies for same content

### Platform Integrations
- **Native APIs**: Direct integration with Slack, Discord, Gmail, LinkedIn APIs
- **Webhook Support**: Real-time content evaluation via webhooks
- **Analytics Integration**: Connect with Google Analytics, social media analytics

This flexible scenario system provides a comprehensive foundation for testing personality-based content evaluation across the diverse landscape of modern digital communication platforms.