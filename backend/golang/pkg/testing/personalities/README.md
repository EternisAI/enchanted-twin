# Personality Testing Framework

A comprehensive testing framework for modeling reference personalities and testing holon thread processing functionality. This framework allows you to create detailed personality profiles with rich memory facts, then test how different personalities respond to various thread scenarios using LLM-based evaluation.

## Overview

The personality testing framework consists of several key components:

- **Reference Personalities**: Detailed personality profiles with memory facts, conversations, and expected behaviors
- **Thread Scenarios**: Test cases representing different types of content (AI news, creative tools, celebrity gossip, etc.)
- **LLM-as-a-Judge**: Automated evaluation of how well actual results match expected personality behaviors
- **Memory Tracking**: Analysis of which memories are accessed during thread evaluation
- **Comprehensive Reporting**: Detailed analysis and scoring of personality performance

## Architecture

```
PersonalityTestFramework
├── framework.go          # Core data structures and loading logic
├── runner.go             # Test execution and LLM judge evaluation
├── reporting.go          # Report generation and analytics
└── personality_test.go   # Integration tests demonstrating usage
```

## Usage

### 1. Define Reference Personalities

Create personality JSON files in `testdata/personalities/{name}/personality.json`:

```json
{
  "name": "tech_entrepreneur",
  "description": "A tech entrepreneur focused on AI and startups",
  "profile": {
    "age": 32,
    "occupation": "Tech Entrepreneur & AI Startup Founder",
    "interests": ["artificial intelligence", "machine learning", "venture capital"],
    "core_traits": ["analytical", "ambitious", "risk-taking"],
    "communication_style": "direct and data-driven"
  },
  "memory_facts": [
    {
      "category": "preference",
      "subject": "user",
      "attribute": "content_interest",
      "value": "highly interested in AI breakthrough news",
      "importance": 3,
      "content": "user - highly interested in AI breakthrough news"
    }
  ],
  "expected_behaviors": [
    {
      "scenario_type": "ai_news",
      "expected": {"interest_level": "high", "likely_to_engage": true},
      "confidence": 0.9
    }
  ]
}
```

### 2. Create Thread Scenarios

Define test scenarios in `testdata/scenarios/{name}.json`:

```json
{
  "name": "ai_breakthrough_news",
  "description": "Technical thread about AI model breakthrough",
  "thread_data": {
    "title": "GPT-5 Achieves 95% Accuracy on Complex Reasoning Tasks",
    "content": "New research shows unprecedented capabilities...",
    "author_name": "ai_researcher",
    "created_at": "2025-01-15T14:25:00Z"
  },
  "expected": {
    "should_show": true,
    "confidence": 0.85,
    "reason_keywords": ["AI", "breakthrough", "technical"],
    "expected_state": "visible"
  }
}
```

### 3. Run Tests

```go
package main

import (
    "context"
    "log"
    
    "github.com/EternisAI/enchanted-twin/pkg/testing/personalities"
    "github.com/EternisAI/enchanted-twin/pkg/ai"
)

func main() {
    // Initialize framework
    logger := log.New(os.Stdout)
    aiService := ai.NewOpenAIService(logger, apiKey, apiURL)
    framework := personalities.NewPersonalityTestFramework(logger, aiService, "testdata")
    
    // Load test data
    framework.LoadPersonalities()
    framework.LoadScenarios()
    
    // Run tests
    results, err := framework.RunPersonalityTests(ctx, memoryStorage, repository)
    if err != nil {
        log.Fatal(err)
    }
    
    // Generate report
    report := framework.GenerateReport(results)
    framework.PrintSummary(report)
    framework.SaveReport(report, "test_report.json")
}
```

## Test Data Structure

```
testdata/
├── personalities/
│   ├── tech_entrepreneur/
│   │   └── personality.json
│   ├── creative_artist/
│   │   └── personality.json
│   └── {personality_name}/
│       └── personality.json
├── scenarios/
│   ├── ai_breakthrough_news.json
│   ├── creative_tool_announcement.json
│   ├── celebrity_gossip.json
│   └── {scenario_name}.json
└── reports/
    └── personality_test_report_{timestamp}.json
```

## Memory Fact Categories

The framework supports rich memory facts with the following categories:

- **preference**: User preferences and interests
- **goal_plan**: Short-term and long-term goals
- **relationship**: Social connections and networks
- **skill_expertise**: Technical and creative skills
- **value_belief**: Core values and beliefs
- **habit_routine**: Daily patterns and behaviors
- **experience_event**: Past experiences and significant events

## Evaluation Metrics

### Test Results
- **Success Rate**: Percentage of tests that meet the score threshold (≥70%)
- **Similarity Score**: 0-1 score from LLM judge comparing actual vs expected results
- **Memory Usage**: Which memories were accessed during evaluation

### Personality Analysis
- **Average Score**: Overall performance across all scenarios
- **Best/Worst Scenarios**: Scenarios where personality performs best/worst
- **Memory Access Patterns**: Which memories are most frequently used

### Scenario Analysis
- **Difficulty Score**: How challenging each scenario is across personalities
- **Personality Differences**: How different personalities respond to same content

## Sample Personalities Included

### Tech Entrepreneur
- **Focus**: AI, startups, business opportunities
- **Traits**: Analytical, data-driven, networking-oriented
- **Expected**: High interest in AI news, low interest in celebrity gossip

### Creative Artist
- **Focus**: Art, design, creative tools, aesthetics
- **Traits**: Intuitive, emotionally-driven, aesthetically-focused
- **Expected**: High interest in creative tools, low interest in technical jargon

## Sample Scenarios Included

### AI Breakthrough News
- **Content**: Technical AI model improvements with metrics
- **Expected**: High interest from tech entrepreneur, medium from artist

### Creative Tool Announcement
- **Content**: New digital art tool with creative features
- **Expected**: High interest from artist, medium from entrepreneur

### Celebrity Gossip
- **Content**: Entertainment news with no practical value
- **Expected**: Low interest from both personalities

## LLM-as-a-Judge Evaluation

The framework uses an LLM judge to evaluate how well actual results match expected behaviors:

- **Similarity Scoring**: 0.0-1.0 scale comparing actual vs expected results
- **Reasoning Analysis**: Checks if expected keywords appear in reasoning
- **Nuanced Evaluation**: Considers multiple factors beyond just the binary decision
- **Fallback Scoring**: Basic algorithmic scoring when LLM judge fails

## Integration with Holon Thread Processing

The framework integrates with the existing holon thread processing system:

- **ThreadProcessor**: Uses actual thread evaluation logic
- **Memory Integration**: Loads personality data into evolving memory system
- **Real Evaluation**: Tests actual decision-making pathways
- **Memory Tracking**: Monitors which memories influence decisions

## Running the Tests

### Prerequisites
- Go 1.21+
- OpenAI API key for LLM evaluation
- Weaviate or compatible vector database (for full integration)

### Quick Start
```bash
# Run the integration test
cd pkg/testing/personalities
go test -v -run TestPersonalityThreadProcessingIntegration

# View generated reports
ls testdata/reports/
```

### Environment Setup
```bash
export COMPLETIONS_API_KEY="your-openai-api-key"
export COMPLETIONS_API_URL="https://api.openai.com/v1"
export EMBEDDINGS_API_KEY="your-openai-api-key"  # Optional, defaults to completions key
```

## Extending the Framework

### Adding New Personalities
1. Create a new directory in `testdata/personalities/{name}/`
2. Add `personality.json` with complete personality profile
3. Include rich memory facts, conversations, and expected behaviors

### Adding New Scenarios
1. Create `{scenario_name}.json` in `testdata/scenarios/`
2. Define thread content and expected evaluation results
3. Include context information and priority levels

### Custom Evaluation Logic
- Extend `LLM-as-a-judge` prompts for domain-specific evaluation
- Add custom scoring metrics for specific personality traits
- Implement memory tracking for specific categories

## Use Cases

### Development Testing
- Validate thread processing logic across different personality types
- Ensure memory integration works correctly
- Test edge cases and boundary conditions

### Personality Modeling
- Research how different personality types respond to content
- Validate psychological models with LLM behavior
- Study memory access patterns and decision-making

### Content Curation
- Test content filtering algorithms across user segments
- Optimize recommendation systems for different personalities
- Validate interest prediction models

## Future Enhancements

- **Multi-modal Scenarios**: Support for image and video content
- **Temporal Analysis**: Track personality changes over time
- **Social Dynamics**: Multi-personality interaction scenarios
- **Real User Validation**: Compare framework results with actual user behavior
- **Performance Optimization**: Batch processing and caching for large test suites