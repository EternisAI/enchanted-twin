package personalities

import (
	"fmt"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

// MemoryFactBuilder helps build memory facts with proper validation
type MemoryFactBuilder struct {
	fact memory.MemoryFact
}

// NewMemoryFactBuilder creates a new memory fact builder
func NewMemoryFactBuilder() *MemoryFactBuilder {
	return &MemoryFactBuilder{
		fact: memory.MemoryFact{
			Timestamp: time.Now(),
			Source:    "personality_test",
		},
	}
}

// Category sets the memory fact category
func (mfb *MemoryFactBuilder) Category(category string) *MemoryFactBuilder {
	mfb.fact.Category = category
	return mfb
}

// Subject sets the subject of the memory fact
func (mfb *MemoryFactBuilder) Subject(subject string) *MemoryFactBuilder {
	mfb.fact.Subject = subject
	return mfb
}

// Attribute sets the attribute of the memory fact
func (mfb *MemoryFactBuilder) Attribute(attribute string) *MemoryFactBuilder {
	mfb.fact.Attribute = attribute
	return mfb
}

// Value sets the value of the memory fact
func (mfb *MemoryFactBuilder) Value(value string) *MemoryFactBuilder {
	mfb.fact.Value = value
	return mfb
}

// TemporalContext sets the temporal context
func (mfb *MemoryFactBuilder) TemporalContext(context string) *MemoryFactBuilder {
	mfb.fact.TemporalContext = &context
	return mfb
}

// Sensitivity sets the sensitivity level
func (mfb *MemoryFactBuilder) Sensitivity(sensitivity string) *MemoryFactBuilder {
	mfb.fact.Sensitivity = sensitivity
	return mfb
}

// Importance sets the importance level (1-3)
func (mfb *MemoryFactBuilder) Importance(importance int) *MemoryFactBuilder {
	if importance < 1 {
		importance = 1
	}
	if importance > 3 {
		importance = 3
	}
	mfb.fact.Importance = importance
	return mfb
}

// Content sets the generated content
func (mfb *MemoryFactBuilder) Content(content string) *MemoryFactBuilder {
	mfb.fact.Content = content
	return mfb
}

// Build creates the final memory fact with validation
func (mfb *MemoryFactBuilder) Build() (memory.MemoryFact, error) {
	// Validate required fields
	if mfb.fact.Category == "" {
		return memory.MemoryFact{}, fmt.Errorf("category is required")
	}
	if mfb.fact.Subject == "" {
		return memory.MemoryFact{}, fmt.Errorf("subject is required")
	}
	if mfb.fact.Attribute == "" {
		return memory.MemoryFact{}, fmt.Errorf("attribute is required")
	}
	if mfb.fact.Value == "" {
		return memory.MemoryFact{}, fmt.Errorf("value is required")
	}

	// Auto-generate content if not set
	if mfb.fact.Content == "" {
		mfb.fact.Content = fmt.Sprintf("%s - %s", mfb.fact.Subject, mfb.fact.Value)
	}

	// Set defaults
	if mfb.fact.Sensitivity == "" {
		mfb.fact.Sensitivity = "low"
	}
	if mfb.fact.Importance == 0 {
		mfb.fact.Importance = 2
	}

	return mfb.fact, nil
}

// PersonalityMemoryBuilder helps build comprehensive memory profiles for personalities
type PersonalityMemoryBuilder struct {
	personalityName string
	facts           []memory.MemoryFact
}

// NewPersonalityMemoryBuilder creates a new personality memory builder
func NewPersonalityMemoryBuilder(personalityName string) *PersonalityMemoryBuilder {
	return &PersonalityMemoryBuilder{
		personalityName: personalityName,
		facts:           make([]memory.MemoryFact, 0),
	}
}

// AddPreference adds a preference memory fact
func (pmb *PersonalityMemoryBuilder) AddPreference(attribute, value string, importance int) *PersonalityMemoryBuilder {
	fact, err := NewMemoryFactBuilder().
		Category("preference").
		Subject("user").
		Attribute(attribute).
		Value(value).
		Importance(importance).
		Build()

	if err == nil {
		pmb.facts = append(pmb.facts, fact)
	}
	return pmb
}

// AddGoal adds a goal/plan memory fact
func (pmb *PersonalityMemoryBuilder) AddGoal(attribute, value, timeline string, importance int) *PersonalityMemoryBuilder {
	fact, err := NewMemoryFactBuilder().
		Category("goal_plan").
		Subject("user").
		Attribute(attribute).
		Value(value).
		TemporalContext(timeline).
		Importance(importance).
		Build()

	if err == nil {
		pmb.facts = append(pmb.facts, fact)
	}
	return pmb
}

// AddSkill adds a skill/expertise memory fact
func (pmb *PersonalityMemoryBuilder) AddSkill(attribute, value string, importance int) *PersonalityMemoryBuilder {
	fact, err := NewMemoryFactBuilder().
		Category("skill_expertise").
		Subject("user").
		Attribute(attribute).
		Value(value).
		Importance(importance).
		Build()

	if err == nil {
		pmb.facts = append(pmb.facts, fact)
	}
	return pmb
}

// AddRelationship adds a relationship memory fact
func (pmb *PersonalityMemoryBuilder) AddRelationship(attribute, value string, sensitivity string, importance int) *PersonalityMemoryBuilder {
	fact, err := NewMemoryFactBuilder().
		Category("relationship").
		Subject("user").
		Attribute(attribute).
		Value(value).
		Sensitivity(sensitivity).
		Importance(importance).
		Build()

	if err == nil {
		pmb.facts = append(pmb.facts, fact)
	}
	return pmb
}

// AddValue adds a value/belief memory fact
func (pmb *PersonalityMemoryBuilder) AddValue(attribute, value string, importance int) *PersonalityMemoryBuilder {
	fact, err := NewMemoryFactBuilder().
		Category("value_belief").
		Subject("user").
		Attribute(attribute).
		Value(value).
		Importance(importance).
		Build()

	if err == nil {
		pmb.facts = append(pmb.facts, fact)
	}
	return pmb
}

// AddHabit adds a habit/routine memory fact
func (pmb *PersonalityMemoryBuilder) AddHabit(attribute, value string, importance int) *PersonalityMemoryBuilder {
	fact, err := NewMemoryFactBuilder().
		Category("habit_routine").
		Subject("user").
		Attribute(attribute).
		Value(value).
		Importance(importance).
		Build()

	if err == nil {
		pmb.facts = append(pmb.facts, fact)
	}
	return pmb
}

// AddExperience adds an experience/event memory fact
func (pmb *PersonalityMemoryBuilder) AddExperience(attribute, value, timeContext string, importance int) *PersonalityMemoryBuilder {
	fact, err := NewMemoryFactBuilder().
		Category("experience_event").
		Subject("user").
		Attribute(attribute).
		Value(value).
		TemporalContext(timeContext).
		Importance(importance).
		Build()

	if err == nil {
		pmb.facts = append(pmb.facts, fact)
	}
	return pmb
}

// AddCustomFact adds a custom memory fact with full control
func (pmb *PersonalityMemoryBuilder) AddCustomFact(category, subject, attribute, value string, options ...func(*MemoryFactBuilder)) *PersonalityMemoryBuilder {
	builder := NewMemoryFactBuilder().
		Category(category).
		Subject(subject).
		Attribute(attribute).
		Value(value)

	// Apply custom options
	for _, option := range options {
		option(builder)
	}

	fact, err := builder.Build()
	if err == nil {
		pmb.facts = append(pmb.facts, fact)
	}
	return pmb
}

// Build returns all the memory facts
func (pmb *PersonalityMemoryBuilder) Build() []memory.MemoryFact {
	return pmb.facts
}

// GetFactCount returns the number of facts added
func (pmb *PersonalityMemoryBuilder) GetFactCount() int {
	return len(pmb.facts)
}

// CreateTechEntrepreneurProfile creates a comprehensive tech entrepreneur memory profile
func CreateTechEntrepreneurProfile() []memory.MemoryFact {
	return NewPersonalityMemoryBuilder("tech_entrepreneur").
		// Preferences
		AddPreference("content_interest", "highly interested in AI breakthrough news, startup funding announcements, and technical deep-dives", 3).
		AddPreference("communication_style", "prefers technical discussions with data and metrics, dislikes superficial content", 2).
		AddPreference("information_sources", "follows tech blogs, research papers, industry reports, and expert opinions", 2).

		// Goals and Plans
		AddGoal("business_goal", "planning to raise Series A funding for AI automation platform", "Q2 2025", 3).
		AddGoal("technical_goal", "scale platform to handle 10x customer growth while maintaining efficiency", "2025", 3).
		AddGoal("network_goal", "establish relationships with 5 top-tier VCs and 10 technical advisors", "2025", 2).

		// Skills
		AddSkill("technical_skills", "expert in Python, ML frameworks, cloud infrastructure, and product management", 2).
		AddSkill("business_skills", "experienced in fundraising, team building, and strategic planning", 2).
		AddSkill("domain_expertise", "deep knowledge of AI/ML market trends and enterprise automation", 3).

		// Relationships
		AddRelationship("professional_network", "actively maintains relationships with VCs, other founders, and technical talent", "medium", 2).
		AddRelationship("advisory_board", "works closely with industry mentors and technical advisors", "medium", 2).

		// Values
		AddValue("innovation", "believes in using technology to solve meaningful problems and create value", 3).
		AddValue("efficiency", "values data-driven decision making and operational efficiency", 2).
		AddValue("growth", "prioritizes scalable solutions and long-term sustainable growth", 2).

		// Habits
		AddHabit("daily_routine", "starts day reading tech news, checks metrics dashboards, blocks time for deep work", 1).
		AddHabit("learning", "regularly reads research papers and attends technical conferences", 2).

		// Experiences
		AddExperience("startup_experience", "founded two AI startups, one successful exit to Google", "2020-2024", 3).
		AddExperience("technical_background", "worked as software engineer at Google for 3 years", "2017-2020", 2).
		Build()
}

// CreateCreativeArtistProfile creates a comprehensive creative artist memory profile
func CreateCreativeArtistProfile() []memory.MemoryFact {
	return NewPersonalityMemoryBuilder("creative_artist").
		// Preferences
		AddPreference("content_interest", "loves discovering new creative tools, art techniques, and aesthetic inspiration", 3).
		AddPreference("work_style", "prefers visual and aesthetic content over technical jargon, values emotional resonance", 2).
		AddPreference("inspiration_sources", "follows art galleries, design studios, creative communities, and visual platforms", 2).

		// Goals and Plans
		AddGoal("career_goal", "working towards launching independent art studio focusing on sustainable digital art", "2025-2026", 3).
		AddGoal("creative_goal", "complete digital illustration series about climate change for upcoming gallery show", "March 2025", 3).
		AddGoal("skill_goal", "master new AI-assisted creative workflows while maintaining artistic authenticity", "2025", 2).

		// Skills
		AddSkill("creative_skills", "expert in Procreate, Photoshop, Illustrator, and traditional drawing techniques", 3).
		AddSkill("artistic_vision", "strong sense of color theory, composition, and visual storytelling", 3).
		AddSkill("client_work", "experienced in creating custom illustrations for tech companies and indie games", 2).

		// Relationships
		AddRelationship("creative_community", "active in local art scene and online creative communities", "low", 2).
		AddRelationship("client_network", "maintains relationships with design agencies and game studios", "medium", 2).

		// Values
		AddValue("sustainability", "strongly believes in environmental responsibility and sustainable creative practices", 3).
		AddValue("authenticity", "values original creative expression and artistic integrity", 3).
		AddValue("collaboration", "believes in supporting other artists and building creative communities", 2).

		// Habits
		AddHabit("creative_routine", "sketches daily, follows art accounts on social media, visits galleries monthly", 2).
		AddHabit("learning", "experiments with new tools and techniques, takes online art courses", 2).

		// Experiences
		AddExperience("education", "graduated from art school with focus on digital media and illustration", "2015-2019", 2).
		AddExperience("freelance_journey", "built successful freelance practice working with tech and gaming clients", "2019-present", 3).
		Build()
}
