package personalities

import (
	"fmt"
	"time"
)

// ScenarioBuilder provides a fluent interface for building test scenarios
type ScenarioBuilder struct {
	scenario *ThreadTestScenario
}

// NewScenarioBuilder creates a new scenario builder
func NewScenarioBuilder(name, description string) *ScenarioBuilder {
	return &ScenarioBuilder{
		scenario: &ThreadTestScenario{
			Name:        name,
			Description: description,
			Context:     make(map[string]interface{}),
			ThreadData: ThreadData{
				CreatedAt: time.Now(),
			},
		},
	}
}

// WithThread sets the thread content
func (sb *ScenarioBuilder) WithThread(title, content, authorName string) *ScenarioBuilder {
	sb.scenario.ThreadData.Title = title
	sb.scenario.ThreadData.Content = content
	sb.scenario.ThreadData.AuthorName = authorName
	return sb
}

// WithAuthor sets the thread author details
func (sb *ScenarioBuilder) WithAuthor(name string, alias *string) *ScenarioBuilder {
	sb.scenario.ThreadData.AuthorName = name
	sb.scenario.ThreadData.AuthorAlias = alias
	return sb
}

// WithImages adds image URLs to the thread
func (sb *ScenarioBuilder) WithImages(urls ...string) *ScenarioBuilder {
	sb.scenario.ThreadData.ImageURLs = append(sb.scenario.ThreadData.ImageURLs, urls...)
	return sb
}

// WithMessage adds a message to the thread
func (sb *ScenarioBuilder) WithMessage(authorName, content string, alias *string) *ScenarioBuilder {
	message := ThreadMessageData{
		AuthorName:  authorName,
		AuthorAlias: alias,
		Content:     content,
		CreatedAt:   time.Now(),
	}
	sb.scenario.ThreadData.Messages = append(sb.scenario.ThreadData.Messages, message)
	return sb
}

// WithTimestamp sets the thread creation time
func (sb *ScenarioBuilder) WithTimestamp(t time.Time) *ScenarioBuilder {
	sb.scenario.ThreadData.CreatedAt = t
	return sb
}

// WithContext adds context metadata
func (sb *ScenarioBuilder) WithContext(key string, value interface{}) *ScenarioBuilder {
	sb.scenario.Context[key] = value
	return sb
}

// ExpectResult sets the expected evaluation result for the scenario
func (sb *ScenarioBuilder) ExpectResult(shouldShow bool, confidence float64, state string, priority int) *ScenarioBuilder {
	if sb.scenario.DefaultExpected == nil {
		sb.scenario.DefaultExpected = &ExpectedThreadEvaluation{}
	}
	sb.scenario.DefaultExpected.ShouldShow = shouldShow
	sb.scenario.DefaultExpected.Confidence = confidence
	sb.scenario.DefaultExpected.ExpectedState = state
	sb.scenario.DefaultExpected.Priority = priority
	return sb
}

// ExpectKeywords sets the expected keywords that should appear in evaluation reasoning
func (sb *ScenarioBuilder) ExpectKeywords(keywords ...string) *ScenarioBuilder {
	if sb.scenario.DefaultExpected == nil {
		sb.scenario.DefaultExpected = &ExpectedThreadEvaluation{}
	}
	sb.scenario.DefaultExpected.ReasonKeywords = append(sb.scenario.DefaultExpected.ReasonKeywords, keywords...)
	return sb
}

// WithPersonalityExpectations adds personality-specific expectations
func (sb *ScenarioBuilder) WithPersonalityExpectations(expectations ...PersonalityExpectedOutcome) *ScenarioBuilder {
	sb.scenario.PersonalityExpectations = append(sb.scenario.PersonalityExpectations, expectations...)
	return sb
}

// generatePersonalityExpectationsFromDefault creates personality expectations based on default expected result
func (sb *ScenarioBuilder) generatePersonalityExpectationsFromDefault() {
	if sb.scenario.DefaultExpected == nil {
		return
	}

	// If no personality expectations are defined, create them from default
	if len(sb.scenario.PersonalityExpectations) == 0 {
		// Generate expectations for standard personalities based on scenario context
		domain := ""
		if val, ok := sb.scenario.Context["domain"].(string); ok {
			domain = val
		}

		sb.scenario.PersonalityExpectations = sb.generateExpectationsForDomain(domain, sb.scenario.DefaultExpected)
	}
}

// generateExpectationsForDomain creates personality expectations based on content domain
func (sb *ScenarioBuilder) generateExpectationsForDomain(domain string, defaultExpected *ExpectedThreadEvaluation) []PersonalityExpectedOutcome {
	expectations := make([]PersonalityExpectedOutcome, 0)

	// Define personality-specific adjustments based on domain
	switch domain {
	case "artificial_intelligence":
		// Tech entrepreneurs should be highly interested in AI
		expectations = append(expectations, PersonalityExpectedOutcome{
			PersonalityName: "tech_entrepreneur",
			ShouldShow:      true,
			Confidence:      0.95,
			ReasonKeywords:  []string{"AI", "breakthrough", "business opportunity", "automation", "competitive advantage"},
			ExpectedState:   "visible",
			Priority:        3,
			Rationale:       "Tech entrepreneurs are highly interested in AI breakthroughs due to potential business applications and competitive advantages",
		})

		// Creative artists moderately interested for creative applications
		expectations = append(expectations, PersonalityExpectedOutcome{
			PersonalityName: "creative_artist",
			ShouldShow:      true,
			Confidence:      0.65,
			ReasonKeywords:  []string{"AI", "creativity", "tools", "potential", "impact"},
			ExpectedState:   "visible",
			Priority:        2,
			Rationale:       "Creative artists are moderately interested in AI breakthroughs from the perspective of potential creative applications",
		})

	case "creative_tools":
		// Creative artists highly interested in creative tools
		expectations = append(expectations, PersonalityExpectedOutcome{
			PersonalityName: "creative_artist",
			ShouldShow:      true,
			Confidence:      0.95,
			ReasonKeywords:  []string{"creative", "design", "workflow", "artistic tools", "Adobe"},
			ExpectedState:   "visible",
			Priority:        3,
			Rationale:       "Creative artists are highly interested in new creative tools that could enhance their work",
		})

		// Tech entrepreneurs see business potential
		expectations = append(expectations, PersonalityExpectedOutcome{
			PersonalityName: "tech_entrepreneur",
			ShouldShow:      true,
			Confidence:      0.75,
			ReasonKeywords:  []string{"AI", "tool", "business opportunity", "market potential"},
			ExpectedState:   "visible",
			Priority:        2,
			Rationale:       "Tech entrepreneurs see business potential in creative tools and AI applications",
		})

	case "entertainment_gossip":
		// Both personalities should generally reject gossip but with different reasoning
		expectations = append(expectations, PersonalityExpectedOutcome{
			PersonalityName: "tech_entrepreneur",
			ShouldShow:      false,
			Confidence:      0.85,
			ReasonKeywords:  []string{"gossip", "irrelevant", "low priority", "entertainment"},
			ExpectedState:   "hidden",
			Priority:        1,
			Rationale:       "Tech entrepreneurs typically filter out celebrity gossip as it's not relevant to business or technical interests",
		})

		expectations = append(expectations, PersonalityExpectedOutcome{
			PersonalityName: "creative_artist",
			ShouldShow:      false,
			Confidence:      0.70,
			ReasonKeywords:  []string{"gossip", "superficial", "not creative"},
			ExpectedState:   "hidden",
			Priority:        2,
			Rationale:       "Creative artists may have mixed feelings about celebrity culture but generally focus on artistic content over gossip",
		})

	case "venture_capital":
		// Tech entrepreneurs highly interested in funding news
		expectations = append(expectations, PersonalityExpectedOutcome{
			PersonalityName: "tech_entrepreneur",
			ShouldShow:      true,
			Confidence:      0.90,
			ReasonKeywords:  []string{"funding", "startup", "investment", "AI", "business"},
			ExpectedState:   "visible",
			Priority:        3,
			Rationale:       "Tech entrepreneurs are highly interested in startup funding announcements for market insights and opportunities",
		})

		// Creative artists less interested but may see relevance for creative industry funding
		expectations = append(expectations, PersonalityExpectedOutcome{
			PersonalityName: "creative_artist",
			ShouldShow:      true,
			Confidence:      0.60,
			ReasonKeywords:  []string{"funding", "startup", "creative industry", "innovation"},
			ExpectedState:   "visible",
			Priority:        2,
			Rationale:       "Creative artists may be interested in funding news if it relates to creative industry or innovative tools",
		})

	case "technical_education":
		// Tech entrepreneurs interested in technical learning
		expectations = append(expectations, PersonalityExpectedOutcome{
			PersonalityName: "tech_entrepreneur",
			ShouldShow:      true,
			Confidence:      0.75,
			ReasonKeywords:  []string{"technical", "tutorial", "guide", "production", "development"},
			ExpectedState:   "visible",
			Priority:        2,
			Rationale:       "Tech entrepreneurs value technical education content for staying current with technology trends",
		})

		// Creative artists interested if relevant to creative work
		expectations = append(expectations, PersonalityExpectedOutcome{
			PersonalityName: "creative_artist",
			ShouldShow:      true,
			Confidence:      0.75,
			ReasonKeywords:  []string{"technical", "tutorial", "guide", "production", "development"},
			ExpectedState:   "visible",
			Priority:        2,
			Rationale:       "Creative artists may be interested in technical tutorials that could apply to their creative workflows",
		})

	default:
		// Use default expectations for unknown domains
		expectations = append(expectations, PersonalityExpectedOutcome{
			PersonalityName: "tech_entrepreneur",
			ShouldShow:      defaultExpected.ShouldShow,
			Confidence:      defaultExpected.Confidence,
			ReasonKeywords:  defaultExpected.ReasonKeywords,
			ExpectedState:   defaultExpected.ExpectedState,
			Priority:        defaultExpected.Priority,
			Rationale:       "Default expectation for tech entrepreneur personality",
		})

		expectations = append(expectations, PersonalityExpectedOutcome{
			PersonalityName: "creative_artist",
			ShouldShow:      defaultExpected.ShouldShow,
			Confidence:      defaultExpected.Confidence,
			ReasonKeywords:  defaultExpected.ReasonKeywords,
			ExpectedState:   defaultExpected.ExpectedState,
			Priority:        defaultExpected.Priority,
			Rationale:       "Default expectation for creative artist personality",
		})
	}

	return expectations
}

// Build creates the final scenario and converts thread data to model
func (sb *ScenarioBuilder) Build(framework *PersonalityTestFramework) ThreadTestScenario {
	// Generate personality expectations from default if none exist
	sb.generatePersonalityExpectationsFromDefault()

	// Convert ThreadData to actual Thread model
	sb.scenario.Thread = framework.createThreadFromData(sb.scenario.ThreadData)
	return *sb.scenario
}

// ScenarioTemplate defines a reusable scenario template
type ScenarioTemplate struct {
	Name        string
	Description string
	Builder     func() *ScenarioBuilder
}

// ScenarioLibrary contains predefined scenario templates and generators
type ScenarioLibrary struct {
	templates map[string]ScenarioTemplate
}

// NewScenarioLibrary creates a new scenario library
func NewScenarioLibrary() *ScenarioLibrary {
	lib := &ScenarioLibrary{
		templates: make(map[string]ScenarioTemplate),
	}

	// Register built-in scenario templates
	lib.registerBuiltinTemplates()

	return lib
}

// registerBuiltinTemplates adds the standard scenario templates
func (sl *ScenarioLibrary) registerBuiltinTemplates() {
	// AI/Tech News Template
	sl.RegisterTemplate("ai_news", ScenarioTemplate{
		Name:        "AI Breakthrough News",
		Description: "Technical AI advancement announcement",
		Builder: func() *ScenarioBuilder {
			return NewScenarioBuilder("ai_breakthrough_news", "Technical thread about AI model breakthrough").
				WithThread(
					"GPT-5 Achieves 95% Accuracy on Complex Reasoning Tasks",
					"New research from OpenAI shows GPT-5 achieving unprecedented 95% accuracy on mathematical reasoning benchmarks, with 40% improvement in code generation tasks.",
					"ai_researcher",
				).
				WithAuthor("ai_researcher", stringPtr("Dr. Sarah Chen")).
				WithMessage("tech_lead", "This is huge for our automation platform. The reasoning capabilities could revolutionize workflows.", stringPtr("Alex Kumar")).
				WithContext("domain", "artificial_intelligence").
				WithContext("technical_level", "high").
				WithContext("business_relevance", "high").
				ExpectResult(true, 0.85, "visible", 3).
				ExpectKeywords("AI", "breakthrough", "technical", "business relevant", "automation")
		},
	})

	// Creative Tools Template
	sl.RegisterTemplate("creative_tool", ScenarioTemplate{
		Name:        "Creative Tool Announcement",
		Description: "New creative software or tool release",
		Builder: func() *ScenarioBuilder {
			return NewScenarioBuilder("creative_tool_announcement", "Announcement about new creative digital art tool").
				WithThread(
					"Adobe Unveils Revolutionary AI-Powered Brush Engine",
					"Adobe announces 'Neural Brush' - an AI-powered painting tool that learns from your artistic style and suggests natural brush strokes.",
					"adobe_creative",
				).
				WithAuthor("adobe_creative", stringPtr("Adobe Creative Team")).
				WithImages("https://example.com/neural-brush-demo.jpg").
				WithMessage("digital_artist_1", "This looks amazing! Finally an AI tool designed BY artists FOR artists.", stringPtr("Maya Rodriguez")).
				WithContext("domain", "creative_tools").
				WithContext("technical_level", "medium").
				WithContext("creative_relevance", "high").
				ExpectResult(true, 0.9, "visible", 3).
				ExpectKeywords("creative", "art", "tool", "design", "artistic")
		},
	})

	// Celebrity Gossip Template
	sl.RegisterTemplate("celebrity_gossip", ScenarioTemplate{
		Name:        "Celebrity Gossip",
		Description: "Entertainment news with no practical value",
		Builder: func() *ScenarioBuilder {
			return NewScenarioBuilder("celebrity_gossip", "Entertainment gossip thread about celebrity relationships").
				WithThread(
					"BREAKING: Celebrity Couple Spotted at Exclusive Restaurant",
					"Sources confirm that A-list actors Jennifer Stone and Ryan Maxwell were seen dining together at the ultra-exclusive Le Bernardin last night.",
					"entertainment_insider",
				).
				WithAuthor("entertainment_insider", stringPtr("Hollywood Scoop")).
				WithImages("https://example.com/celebrity-photo.jpg").
				WithMessage("gossip_fan_1", "OMG I KNEW IT! They had such amazing chemistry in their movie! 😍", stringPtr("MovieLover2024")).
				WithContext("domain", "entertainment_gossip").
				WithContext("technical_level", "none").
				WithContext("business_relevance", "none").
				WithContext("creative_relevance", "none").
				ExpectResult(false, 0.8, "hidden", 1).
				ExpectKeywords("irrelevant", "gossip", "not interesting", "celebrity")
		},
	})

	// Startup Funding Template
	sl.RegisterTemplate("startup_funding", ScenarioTemplate{
		Name:        "Startup Funding News",
		Description: "Investment and funding announcements",
		Builder: func() *ScenarioBuilder {
			return NewScenarioBuilder("startup_funding_news", "Major startup funding announcement").
				WithThread(
					"AI Startup Anthropic Raises $4B Series C Led by Amazon",
					"Anthropic has closed a massive $4 billion Series C funding round led by Amazon, bringing total funding to $7.3B. The company plans to use funds to scale Claude AI and compete with OpenAI.",
					"venture_beat",
				).
				WithAuthor("venture_beat", stringPtr("VentureBeat")).
				WithMessage("vc_partner", "Massive round! The AI infrastructure space is heating up. This validates our thesis on enterprise AI.", stringPtr("Sarah Kim")).
				WithMessage("startup_founder", "Incredible scale. Shows how much capital is flowing into AI right now.", stringPtr("David Chen")).
				WithContext("domain", "venture_capital").
				WithContext("technical_level", "medium").
				WithContext("business_relevance", "high").
				ExpectResult(true, 0.9, "visible", 3).
				ExpectKeywords("funding", "startup", "investment", "AI", "business")
		},
	})

	// Technical Tutorial Template
	sl.RegisterTemplate("technical_tutorial", ScenarioTemplate{
		Name:        "Technical Tutorial",
		Description: "Educational technical content",
		Builder: func() *ScenarioBuilder {
			return NewScenarioBuilder("technical_tutorial", "Deep technical tutorial or guide").
				WithThread(
					"Building Production-Ready RAG Systems: A Complete Guide",
					"Comprehensive guide covering vector databases, embedding strategies, retrieval optimization, and deployment patterns for production RAG applications. Includes code examples and performance benchmarks.",
					"tech_educator",
				).
				WithAuthor("tech_educator", stringPtr("Dr. Alex Thompson")).
				WithMessage("ml_engineer", "Excellent breakdown of retrieval strategies. The chunk overlap optimization section is particularly useful.", stringPtr("Lisa Wang")).
				WithMessage("startup_cto", "Bookmarking this for our team. We're implementing RAG for customer support.", stringPtr("Mike Rodriguez")).
				WithContext("domain", "technical_education").
				WithContext("technical_level", "high").
				WithContext("business_relevance", "medium").
				ExpectResult(true, 0.75, "visible", 2).
				ExpectKeywords("technical", "tutorial", "guide", "production", "development")
		},
	})
}

// RegisterTemplate adds a new scenario template to the library
func (sl *ScenarioLibrary) RegisterTemplate(key string, template ScenarioTemplate) {
	sl.templates[key] = template
}

// GetTemplate retrieves a scenario template by key
func (sl *ScenarioLibrary) GetTemplate(key string) (ScenarioTemplate, bool) {
	template, exists := sl.templates[key]
	return template, exists
}

// ListTemplates returns all available template keys
func (sl *ScenarioLibrary) ListTemplates() []string {
	keys := make([]string, 0, len(sl.templates))
	for key := range sl.templates {
		keys = append(keys, key)
	}
	return keys
}

// GenerateScenario creates a scenario from a template
func (sl *ScenarioLibrary) GenerateScenario(templateKey string, framework *PersonalityTestFramework) (ThreadTestScenario, error) {
	template, exists := sl.GetTemplate(templateKey)
	if !exists {
		return ThreadTestScenario{}, fmt.Errorf("template not found: %s", templateKey)
	}

	builder := template.Builder()
	return builder.Build(framework), nil
}

// GenerateScenarioVariant creates a variant of a template with modifications
func (sl *ScenarioLibrary) GenerateScenarioVariant(templateKey string, framework *PersonalityTestFramework, modifier func(*ScenarioBuilder) *ScenarioBuilder) (ThreadTestScenario, error) {
	template, exists := sl.GetTemplate(templateKey)
	if !exists {
		return ThreadTestScenario{}, fmt.Errorf("template not found: %s", templateKey)
	}

	builder := template.Builder()
	modifiedBuilder := modifier(builder)
	return modifiedBuilder.Build(framework), nil
}

// ParameterizedScenarioBuilder allows for dynamic scenario generation with parameters
type ParameterizedScenarioBuilder struct {
	baseTemplate string
	parameters   map[string]interface{}
}

// NewParameterizedScenario creates a new parameterized scenario builder
func NewParameterizedScenario(baseTemplate string) *ParameterizedScenarioBuilder {
	return &ParameterizedScenarioBuilder{
		baseTemplate: baseTemplate,
		parameters:   make(map[string]interface{}),
	}
}

// WithParameter sets a parameter value
func (psb *ParameterizedScenarioBuilder) WithParameter(key string, value interface{}) *ParameterizedScenarioBuilder {
	psb.parameters[key] = value
	return psb
}

// Build generates the scenario with the given parameters
func (psb *ParameterizedScenarioBuilder) Build(library *ScenarioLibrary, framework *PersonalityTestFramework) (ThreadTestScenario, error) {
	return library.GenerateScenarioVariant(psb.baseTemplate, framework, func(builder *ScenarioBuilder) *ScenarioBuilder {
		// Apply parameters to modify the builder
		if title, ok := psb.parameters["title"].(string); ok {
			builder.scenario.ThreadData.Title = title
		}
		if content, ok := psb.parameters["content"].(string); ok {
			builder.scenario.ThreadData.Content = content
		}
		if authorName, ok := psb.parameters["author_name"].(string); ok {
			builder.scenario.ThreadData.AuthorName = authorName
		}
		if shouldShow, ok := psb.parameters["should_show"].(bool); ok {
			builder.scenario.DefaultExpected.ShouldShow = shouldShow
		}
		if confidence, ok := psb.parameters["confidence"].(float64); ok {
			builder.scenario.DefaultExpected.Confidence = confidence
		}
		if keywords, ok := psb.parameters["keywords"].([]string); ok {
			builder.scenario.DefaultExpected.ReasonKeywords = keywords
		}

		return builder
	})
}

// ScenarioGenerator provides high-level scenario generation functions
type ScenarioGenerator struct {
	library *ScenarioLibrary
}

// NewScenarioGenerator creates a new scenario generator
func NewScenarioGenerator() *ScenarioGenerator {
	return &ScenarioGenerator{
		library: NewScenarioLibrary(),
	}
}

// GetLibrary returns the scenario library
func (sg *ScenarioGenerator) GetLibrary() *ScenarioLibrary {
	return sg.library
}

// GenerateStandardScenarios creates a set of standard test scenarios
func (sg *ScenarioGenerator) GenerateStandardScenarios(framework *PersonalityTestFramework) ([]ThreadTestScenario, error) {
	scenarios := make([]ThreadTestScenario, 0)

	standardTemplates := []string{"ai_news", "creative_tool", "celebrity_gossip", "startup_funding", "technical_tutorial"}

	for _, templateKey := range standardTemplates {
		scenario, err := sg.library.GenerateScenario(templateKey, framework)
		if err != nil {
			return nil, fmt.Errorf("failed to generate scenario from template %s: %w", templateKey, err)
		}
		scenarios = append(scenarios, scenario)
	}

	return scenarios, nil
}

// GeneratePersonalityTargetedScenarios creates scenarios tailored for specific personalities
func (sg *ScenarioGenerator) GeneratePersonalityTargetedScenarios(personalityType string, framework *PersonalityTestFramework) ([]ThreadTestScenario, error) {
	scenarios := make([]ThreadTestScenario, 0)

	switch personalityType {
	case "tech_entrepreneur":
		// Generate scenarios that should appeal to tech entrepreneurs
		scenarios = append(scenarios, []ThreadTestScenario{
			sg.mustGenerateScenario("ai_news", framework),
			sg.mustGenerateScenario("startup_funding", framework),
			sg.mustGenerateScenario("technical_tutorial", framework),
		}...)

		// Generate variant scenarios
		variant, _ := sg.library.GenerateScenarioVariant("ai_news", framework, func(builder *ScenarioBuilder) *ScenarioBuilder {
			return builder.WithThread(
				"New AI Chip Architecture Delivers 10x Performance Gains",
				"Revolutionary neuromorphic chip design from Intel shows 10x improvement in AI inference with 50% lower power consumption. Game-changer for edge AI applications.",
				"tech_insider",
			).ExpectResult(true, 0.9, "visible", 3)
		})
		scenarios = append(scenarios, variant)

	case "creative_artist":
		// Generate scenarios that should appeal to creative artists
		scenarios = append(scenarios, []ThreadTestScenario{
			sg.mustGenerateScenario("creative_tool", framework),
		}...)

		// Generate art-focused variants
		variant, _ := sg.library.GenerateScenarioVariant("creative_tool", framework, func(builder *ScenarioBuilder) *ScenarioBuilder {
			return builder.WithThread(
				"Procreate Dreams Launches with Revolutionary Animation Features",
				"The new animation app from Procreate introduces frame-by-frame animation with AI-assisted in-betweening, bringing professional animation tools to iPad.",
				"procreate_team",
			).ExpectResult(true, 0.95, "visible", 3)
		})
		scenarios = append(scenarios, variant)
	}

	return scenarios, nil
}

// mustGenerateScenario is a helper that panics on error (for internal use)
func (sg *ScenarioGenerator) mustGenerateScenario(templateKey string, framework *PersonalityTestFramework) ThreadTestScenario {
	scenario, err := sg.library.GenerateScenario(templateKey, framework)
	if err != nil {
		panic(fmt.Sprintf("failed to generate scenario %s: %v", templateKey, err))
	}
	return scenario
}

// Helper function for string pointers
func stringPtr(s string) *string {
	return &s
}
