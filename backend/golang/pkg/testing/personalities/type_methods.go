package personalities

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

// Method implementations for various types

// MemoryFact methods
func (mf *MemoryFact) GenerateContent() string {
	return fmt.Sprintf("[%s] %s (Importance: %.2f, Tags: %s)",
		mf.Category, mf.Content, mf.Importance, strings.Join(mf.Tags, ", "))
}

// ExtendedPersonality methods
func (ep *ExtendedPersonality) ToReferencePersonality() *ReferencePersonality {
	result := &ReferencePersonality{
		Name:              ep.Base.Name,
		Description:       ep.Base.Description,
		Profile:           ep.Base.Profile,
		MemoryFacts:       make([]MemoryFact, len(ep.Base.MemoryFacts)),
		Conversations:     ep.Base.Conversations,
		Plans:             ep.Base.Plans,
		ExpectedBehaviors: make([]ExpectedBehavior, 0),
	}

	// Copy base memory facts
	copy(result.MemoryFacts, ep.Base.MemoryFacts)

	// Apply all extensions
	if len(ep.Extensions) > 0 {
		var names []string
		var descs []string
		for _, ext := range ep.Extensions {
			// Override profile if specified
			if ext.ProfileOverrides != nil {
				result.Profile = mergeProfiles(result.Profile, *ext.ProfileOverrides)
			}
			
			// Add extension memory facts
			result.MemoryFacts = append(result.MemoryFacts, ext.AdditionalFacts...)
			
			// Add extension plans
			result.Plans = append(result.Plans, ext.AdditionalPlans...)
			
			// Add expected behaviors
			result.ExpectedBehaviors = append(result.ExpectedBehaviors, ext.ExpectedBehaviors...)
			
			// Collect for naming
			names = append(names, ext.TestName)
			descs = append(descs, ext.Description)
		}
		
		// Update name and description
		if len(names) > 0 {
			result.Name = fmt.Sprintf("%s+%s", result.Name, strings.Join(names, "+"))
		}
		if len(descs) > 0 {
			result.Description = fmt.Sprintf("%s (Extended: %s)", result.Description, strings.Join(descs, "; "))
		}
	}

	return result
}

// mergeProfiles merges profile overrides with base profile
func mergeProfiles(base PersonalityProfile, override PersonalityProfile) PersonalityProfile {
	result := base

	if override.Age > 0 {
		result.Age = override.Age
	}
	if override.Occupation != "" {
		result.Occupation = override.Occupation
	}
	if len(override.Interests) > 0 {
		result.Interests = append(result.Interests, override.Interests...)
	}
	if len(override.CoreTraits) > 0 {
		result.CoreTraits = append(result.CoreTraits, override.CoreTraits...)
	}
	if override.CommunicationStyle != "" {
		result.CommunicationStyle = override.CommunicationStyle
	}
	if override.Location != "" {
		result.Location = override.Location
	}
	if override.Background != "" {
		result.Background = override.Background
	}

	return result
}

// ThreadTestScenario methods
func (tts *ThreadTestScenario) GetExpectedOutcomeForPersonality(personalityName string, extensionNames []string) *PersonalityExpectedOutcome {
	// First try to find exact match with extensions
	if len(extensionNames) > 0 {
		for _, outcome := range tts.PersonalityExpectations {
			if outcome.PersonalityName == personalityName && stringSlicesEqual(outcome.ExtensionNames, extensionNames) {
				return &outcome
			}
		}
	}

	// Then try to find base personality match (no extensions)
	for _, outcome := range tts.PersonalityExpectations {
		if outcome.PersonalityName == personalityName && len(outcome.ExtensionNames) == 0 {
			return &outcome
		}
	}

	// Return nil if no specific expectation found
	return nil
}

// PersonalityExpectedOutcome methods
func (peo *PersonalityExpectedOutcome) GetExpectedThreadEvaluation() ExpectedThreadEvaluation {
	return ExpectedThreadEvaluation{
		ShouldShow:     peo.ShouldShow,
		Confidence:     peo.Confidence,
		ReasonKeywords: peo.ReasonKeywords,
		ExpectedState:  peo.ExpectedState,
		Priority:       peo.Priority,
	}
}

// ConversationDocument implements memory.Document interface
func (cd *ConversationDocument) ID() string {
	return cd.DocumentID
}

func (cd *ConversationDocument) Content() string {
	var messageTexts []string
	for _, msg := range cd.Messages {
		messageTexts = append(messageTexts, fmt.Sprintf("%s: %s", msg.Speaker, msg.Content))
	}
	return fmt.Sprintf("Conversation between %v: %s", cd.Participants, strings.Join(messageTexts, " | "))
}

func (cd *ConversationDocument) Chunk() []memory.Document {
	// Simple chunking for conversation documents - return the document itself as a single chunk
	return []memory.Document{cd}
}

func (cd *ConversationDocument) Timestamp() *time.Time {
	return &cd.CreatedAt
}

func (cd *ConversationDocument) Tags() []string {
	return []string{}
}

func (cd *ConversationDocument) Metadata() map[string]string {
	return map[string]string{
		"participants": strings.Join(cd.Participants, ","),
		"context":      cd.Context,
		"created_at":   cd.CreatedAt.Format(time.RFC3339),
	}
}

func (cd *ConversationDocument) Source() string {
	return "conversation"
}

// MemoryTracker methods
func NewMemoryTracker() *MemoryTracker {
	return &MemoryTracker{
		accessedMemories: make([]string, 0),
	}
}

func (mt *MemoryTracker) Reset() {
	mt.accessedMemories = make([]string, 0)
}

func (mt *MemoryTracker) GetAccessedMemories() []string {
	return mt.accessedMemories
}

func (mt *MemoryTracker) TrackAccess(memoryID string) {
	mt.accessedMemories = append(mt.accessedMemories, memoryID)
}

// Helper utility functions
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func mergeStringSlices(base, additions []string) []string {
	result := make([]string, len(base))
	copy(result, base)
	
	for _, addition := range additions {
		found := false
		for _, existing := range result {
			if existing == addition {
				found = true
				break
			}
		}
		if !found {
			result = append(result, addition)
		}
	}
	
	return result
}

// NewThreadScenario creates a new thread scenario for testing
func NewThreadScenario(name, description string, threadData ThreadData) *ThreadTestScenario {
	return &ThreadTestScenario{
		Name:                    name,
		Description:             description,
		ThreadData:              threadData,
		Context:                 make(map[string]interface{}),
		PersonalityExpectations: make([]PersonalityExpectedOutcome, 0),
	}
}

// GenericTestScenario methods
func (gts *GenericTestScenario) GetExpectedOutcomeForPersonality(personalityName string, extensionNames []string) *PersonalityExpectedOutcome {
	// First try to find exact match with extensions
	if len(extensionNames) > 0 {
		for _, outcome := range gts.PersonalityExpectations {
			if outcome.PersonalityName == personalityName && stringSlicesEqual(outcome.ExtensionNames, extensionNames) {
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

	// Return nil if no specific expectation found
	return nil
}

// Evaluate evaluates a generic scenario for a personality in a test environment
func (gts *GenericTestScenario) Evaluate(ctx context.Context, personality *ReferencePersonality, env *TestEnvironment) (*GenericEvaluationResult, error) {
	// For mock testing, create a basic evaluation based on scenario type
	result := &GenericEvaluationResult{
		ShouldShow: true, // Default to showing
		Reason:     fmt.Sprintf("Mock evaluation of %s scenario for personality %s", gts.Type, personality.Name),
		Confidence: 0.8,
		NewState:   "visible",
		Metadata:   make(map[string]interface{}),
	}

	// Add some basic logic based on scenario type
	switch gts.Type {
	case ScenarioTypeThread:
		result.Reason = fmt.Sprintf("Thread scenario evaluated for %s", personality.Name)
	case ScenarioTypeChatMessage:
		result.Reason = fmt.Sprintf("Chat message scenario evaluated for %s", personality.Name)
	case ScenarioTypeEmail:
		result.Reason = fmt.Sprintf("Email scenario evaluated for %s", personality.Name)
	case ScenarioTypeSocialPost:
		result.Reason = fmt.Sprintf("Social post scenario evaluated for %s", personality.Name)
	default:
		result.Reason = fmt.Sprintf("Generic scenario evaluated for %s", personality.Name)
	}

	// Add personality-specific metadata
	result.Metadata["personality_name"] = personality.Name
	result.Metadata["scenario_type"] = string(gts.Type)
	result.Metadata["evaluation_time"] = time.Now().Format(time.RFC3339)

	return result, nil
}