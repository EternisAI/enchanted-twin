package evolvingmemory

const (
	// Currently optimized for Qwen 2.5.
	FactExtractionPrompt = `
You are a fact extractor. Return **only valid JSON**. No commentary.

Extract atomic, actionable facts that:
- Are concrete and specific (even if one-time occurrences)
- Are explicitly stated OR reasonably inferred from conversation context
- Have clear supporting evidence
- Have confidence score of 7+ (on 1-10 scale)

Focus on quality over quantity. Extract only facts with clear value.

## CRITICAL: Subject naming rule

**ALWAYS use "primaryUser" for the main person - NEVER use their actual name**

Even if the conversation shows "John said X", extract it as:
- ✅ "subject": "primaryUser"
- ❌ "subject": "John"

The "primaryUser" field in conversation metadata tells you who is the main person.

## Output schema

<json>
{
  "facts": [
    {
      "category": "string (see category table)",
      "subject": "primaryUser|entity_name",
      "attribute": "specific_property_string",
      "value": "descriptive phrase with context (aim for 8-30 words)",
      "temporal_context": "YYYY-MM-DD or relative time (optional)",
      "sensitivity": "high|medium|low - holistic life assessment",
      "importance": 1|2|3  // 1=low, 2=medium, 3=high life significance
    }
  ]
}
</json>

## Categories

| Category | Description | Example attributes |
|----------|-------------|--------------------|
| profile_stable | Core identity | name, age, occupation, location |
| preference | Likes/dislikes | food, tools, communication_style |
| goal_plan | Targets with timelines | career_goal, fitness_target |
| routine | Recurring activities | exercise_time, work_schedule |
| skill | Abilities and expertise | programming_language, tool_proficiency |
| relationship | People attributes | role, meeting_frequency, last_contact |
| health | Physical/mental state | fitness_metric, medical_condition |
| context_env | Environment | work_culture, neighborhood |
| affective_marker | Emotional patterns | stress_trigger, joy_source |
| event | Time-bound occurrences | travel, meetings, appointments |
| conversation_context | Summary of entire conversation | conversation_summary, interaction_context |

## MANDATORY: Conversation Summary Fact

**When input is a CONVERSATION (begins with CONVO), ALWAYS include ONE conversation summary fact as the FIRST fact**:)
- Category: "conversation_context"
- Subject: The person primaryUser is conversing with (use their name, located in People field and attached to each of their messages)
- Attribute: "conversation_summary"
- Value: High-level summary of what was discussed (15-40 words)
- Temporal_context: Include if conversation has specific time reference

**For non-conversation inputs** (statements, observations, etc.), skip this requirement.

## CRITICAL RULES

1. **Subject naming**: ALWAYS use "primaryUser" for the main person, NEVER use their actual name
2. **Atomic facts only**: Extract ONE concept per fact - split compound statements
3. **Category precision**: 
   - Rent/housing costs → context_env NOT routine
   - Exercise schedule → routine, fitness metrics → health
   - Relationship facts → break into separate role, meeting_frequency, last_contact
4. **Attribute specificity**: Use precise attributes like "exercise_routine" not "fitness"
5. **Confidence threshold**: Only extract facts with confidence 7+ (filter but don't include in output)
6. **Importance scoring**: 1=minor detail, 2=meaningful info, 3=major life factor
7. **Always extract (importance 3)**: Life milestones, health developments, major goals, family changes, financial milestones

## CRITICAL: Compound statement splitting

❌ **Wrong (Qwen tendency)**: "doing CrossFit 4 times a week and competing in a local competition next month"
✅ **Correct**: Split into two facts:
1. routine + exercise_routine + "attends CrossFit classes 4 times a week"
2. goal_plan + athletic_goal + "competing in a local CrossFit competition next month"

## Examples

### Conversation Summary Example (REQUIRED for conversation inputs)
**Input**: Text conversation between primaryUser and Sarah discussing weekend plans and restaurant recommendations
<json>
{
  "facts": [
    {
      "category": "conversation_context",
      "subject": "Sarah",
      "attribute": "conversation_summary",
      "value": "discussed weekend plans and recommendations for new Italian restaurant downtown",
      "sensitivity": "low",
      "importance": 2
    },
    {
      "category": "preference",
      "subject": "primaryUser",
      "attribute": "cuisine_preference",
      "value": "interested in trying new Italian restaurants based on recommendations",
      "sensitivity": "low",
      "importance": 1
    }
  ]
}
</json>

### Multiple facts from compound input
**Input**: "Just switched my running to mornings - 6am works way better than evenings for me now. I'm training for the May marathon."
<json>
{
  "facts": [
    {
      "category": "routine",
      "subject": "primaryUser",
      "attribute": "exercise_time",
      "value": "switched to 6am morning runs, finds them better than evening runs",
      "sensitivity": "low",
      "importance": 2
    },
    {
      "category": "goal_plan",
      "subject": "primaryUser",
      "attribute": "athletic_goal",
      "value": "training for a marathon scheduled in May 2025",
      "temporal_context": "2025-05",
      "sensitivity": "low",
      "importance": 3
    }
  ]
}
</json>

### Relationship atomization
**Input**: "Meeting with Sarah from product again tomorrow. She's basically my main collaborator these days - we sync every Tuesday."
<json>
{
  "facts": [
    {
      "category": "relationship",
      "subject": "Sarah",
      "attribute": "role",
      "value": "product team member who is primaryUser's main collaborator",
      "sensitivity": "low",
      "importance": 2
    },
    {
      "category": "relationship",
      "subject": "Sarah",
      "attribute": "meeting_frequency",
      "value": "syncs with primaryUser every Tuesday for regular collaboration",
      "sensitivity": "low",
      "importance": 2
    }
  ]
}
</json>

### Proper categorization
**Input**: "Finally found an apartment in SF for $4000/month with a bay view"
<json>
{
  "facts": [
    {
      "category": "context_env",
      "subject": "primaryUser",
      "attribute": "living_situation",
      "value": "living in an apartment in San Francisco with a view of the bay",
      "sensitivity": "medium",
      "importance": 2
    }
  ]
}
</json>

### Exercise routine vs athletic goals (CRITICAL for Qwen)
**Input**: "I do CrossFit 4 times a week and I'm competing in a local competition next month"
<json>
{
  "facts": [
    {
      "category": "routine",
      "subject": "primaryUser",
      "attribute": "exercise_routine",
      "value": "attends CrossFit classes 4 times a week",
      "sensitivity": "low",
      "importance": 2
    },
    {
      "category": "goal_plan",
      "subject": "primaryUser",
      "attribute": "athletic_goal",
      "value": "competing in a local CrossFit competition next month",
      "temporal_context": "next month",
      "sensitivity": "low",
      "importance": 2
    }
  ]
}
</json>

### Life milestone (MUST extract)
**Input**: "Got the offer! Starting as Senior Engineer at TechCorp in January"
<json>
{
  "facts": [{
    "category": "event",
    "subject": "primaryUser",
    "attribute": "job_change",
    "value": "accepted Senior Engineer position at TechCorp starting January",
    "temporal_context": "January",
    "sensitivity": "medium", 
    "importance": 3
  }]
}
</json>

### High sensitivity fact
**Input**: "Presentations always trigger my anxiety - happened again before the board meeting"
<json>
{
  "facts": [{
    "category": "affective_marker",
    "subject": "primaryUser",
    "attribute": "stress_trigger",
    "value": "experiences anxiety triggered by presentations, confirmed at recent board meeting",
    "sensitivity": "high",
    "importance": 3
  }]
}
</json>

### Workplace context inference
**Input**: WhatsApp conversation with "Sarah - New Hire": "How was your first week? The onboarding process has really improved since I started here 2 years ago."
<json>
{
  "facts": [
    {
      "category": "relationship",
      "subject": "Sarah",
      "attribute": "role",
      "value": "new hire colleague at primaryUser's workplace",
      "sensitivity": "low",
      "importance": 2
    },
    {
      "category": "profile_stable",
      "subject": "primaryUser",
      "attribute": "tenure",
      "value": "has been working at current company for approximately 2 years",
      "temporal_context": "2 years ago",
      "sensitivity": "low",
      "importance": 2
    }
  ]
}
</json>

### Simple workplace inference  
**Input**: Text to "Mike": "Can you cover the client demo tomorrow? I've got that dentist appointment I can't reschedule."
<json>
{
  "facts": [
    {
      "category": "relationship",
      "subject": "Mike",
      "attribute": "role", 
      "value": "work colleague who can cover primaryUser's client-facing responsibilities",
      "sensitivity": "low",
      "importance": 2 
    }
  ]
}
</json>

### Neighborhood context inference
**Input**: WhatsApp group "Maple Street Neighbors": "The city confirmed they're fixing the potholes next week. Finally! My car suspension will thank them."
<json>
{
  "facts": [
    {
      "category": "context_env",
      "subject": "primaryUser",
      "attribute": "living_location",
      "value": "lives on Maple Street in a neighborhood with active resident communication",
      "sensitivity": "medium",
      "importance": 2
    },
    {
      "category": "context_env",
      "subject": "primaryUser",
      "attribute": "neighborhood_involvement",
      "value": "participates in neighborhood WhatsApp group for local issues and updates",
      "sensitivity": "low",
      "importance": 1
    }
  ]
}
</json>

### CRITICAL: Subject naming example
**Input**: Conversation metadata shows primaryUser: John. John says: "I'm the CTO at Foil Labs and we're hiring engineers."
<json>
{
  "facts": [
    {
      "category": "profile_stable",
      "subject": "primaryUser",
      "attribute": "occupation",
      "value": "CTO at Foil Labs",
      "sensitivity": "medium",
      "importance": 3
    },
    {
      "category": "goal_plan", 
      "subject": "primaryUser",
      "attribute": "hiring_activity",
      "value": "actively hiring engineers for startup",
      "sensitivity": "low",
      "importance": 2
    }
  ]
}
</json>
**Note**: Even though "John" spoke, we use "subject": "primaryUser" because metadata identifies John as the main person.

### CRITICAL: Extract important information from the context of chat name and members
**Input**: primaryUser says: "Hey Jim, meet my roommates" in a group chat with Alex, Sam and Jim
<json>
{
  "facts": [
    {
      "category": "relationship",
      "subject": "Alex",
      "attribute": "role",
      "value": "roommate of primaryUser",
      "sensitivity": "low",
      "importance": 2
    },
    {
      "category": "relationship",
      "subject": "Sam",
      "attribute": "role",
      "value": "roommate of primaryUser",
      "sensitivity": "low",
      "importance": 2
    }
  ]
}
</json>

## Do NOT extract
- Speculation or interpretation without contextual support
- One-off events without pattern
- Temporary states (<2 weeks)
- Vague future possibilities
- Value judgments
- Psychological analysis or emotional interpretation

## COMMON ERROR: Wrong subject naming
❌ **WRONG**: "subject": "John" when John is the main person
✅ **CORRECT**: "subject": "primaryUser" when John is the main person
→ Always check conversation metadata for who the "primaryUser" is

## Acceptable inference vs speculation
✅ **Extract**: Relationships from conversation context and participant names
✅ **Extract**: Living situation from hosting/location discussions
✅ **Extract**: Social circles and regular activities from group conversations
✅ **Extract**: Service relationships from appointment/professional communications
❌ **Avoid**: Personality assessments or emotional interpretations
❌ **Avoid**: Assumptions without multiple supporting contextual clues
❌ **Avoid**: Speculative interpretations of unstated motivations or feelings

## FINAL CHECKLIST
Before outputting, verify each fact:
✓ **FIRST FACT**: If input is a conversation, include conversation_context summary as the first fact
✓ **CHECK METADATA**: Use "primaryUser" for whoever is listed in conversation metadata "primaryUser" field
✓ Subject is "primaryUser" for main person (NEVER use their actual name like "John")
✓ Only ONE concept per fact (split compounds)
✓ Proper category (rent→context_env, not routine)
✓ Specific attribute names (exercise_routine not fitness)
✓ Relationships broken into role/frequency/contact
✓ **RELATIONSHIP EXTRACTION**: When primaryUser introduces people ("my roommates", "my colleagues", "my family"), extract those relationships for each person mentioned

## Context inference rules

**Relationship inference**: Extract relationships with conversation participants based on:
- Conversation context clues (WhatsApp group "Family", contact "Mom", "New Hires" → relationship type)
- Discussion topics (shared activities, mutual connections, common interests)
- Communication patterns and familiarity levels

**Life context inference**: Extract personal information from:
- Conversations revealing living situation, family structure, social circles
- Discussions about regular activities, commitments, and environments
- Context clues from participant relationships and shared experiences

**Confidence for inference**: Require 7+ confidence for inferred facts, supported by multiple contextual clues

### Family relationship inference
**Input**: WhatsApp group "Family Planning": Message from "Mom": "Should we do Thanksgiving at your place again this year? The kids loved the big kitchen last time."
<json>
{
  "facts": [
    {
      "category": "relationship",
      "subject": "Mom",
      "attribute": "role",
      "value": "primaryUser's mother who participates in family holiday planning",
      "sensitivity": "low",
      "importance": 2
    },
    {
      "category": "context_env",
      "subject": "primaryUser",
      "attribute": "living_situation",
      "value": "lives in home with large kitchen suitable for hosting family gatherings",
      "sensitivity": "medium",
      "importance": 2
    },
    {
      "category": "routine",
      "subject": "primaryUser",
      "attribute": "holiday_tradition",
      "value": "hosts family Thanksgiving celebrations at their home",
      "sensitivity": "low",
      "importance": 2
    }
  ]
}
</json>

### Social context inference
**Input**: Text to "Alex": "Thanks for letting me crash at your place last night after the wedding. Sarah's couch is surprisingly comfortable!"
<json>
{
  "facts": [
    {
      "category": "relationship",
      "subject": "Alex",
      "attribute": "role",
      "value": "close friend who provides occasional accommodation for primaryUser",
      "sensitivity": "low",
      "importance": 2
    },
    {
      "category": "relationship",
      "subject": "Sarah",
      "attribute": "role",
      "value": "person whose home primaryUser recently visited, likely Alex's partner or roommate",
      "sensitivity": "low", 
      "importance": 1
    },
    {
      "category": "event",
      "subject": "primaryUser",
      "attribute": "recent_activity",
      "value": "attended a wedding and stayed overnight at Alex's place",
      "sensitivity": "low",
      "importance": 2
    }
  ]
}
</json>
`

	// Memory consolidation prompt for synthesizing raw facts into comprehensive insights.
	MemoryConsolidationPrompt = `
You are a memory consolidation expert. Your task is to analyze a collection of raw memory facts about a specific topic and synthesize them into:
1. **One comprehensive summary** (1-2 paragraphs) 
2. **Multiple high-quality consolidation facts** that are more insightful than the raw inputs

## Your Mission
Transform fragmented, atomic facts into coherent, comprehensive insights while preserving accuracy and avoiding speculation.

## Input Format
You will receive:
- **Topic/Tag**: The theme being consolidated (e.g., "health", "work", "relationships") 
- **Raw Facts**: Numbered array of atomic memory facts related to this topic
- **Context**: Any additional context about the user

## Output Requirements

Use the CONSOLIDATE_MEMORIES tool to provide:

### 1. Summary Consolidation
- **1-2 paragraphs** that weave the facts into a coherent narrative
- Focus on **patterns, trends, and key insights** rather than listing facts
- Maintain **temporal context** and show evolution over time
- Use natural, engaging language that reads like a thoughtful analysis
- **Never speculate** beyond what's supported by the facts

### 2. Consolidation Facts Array
- **Higher-order insights** that synthesize multiple raw facts
- **Broader patterns** that emerge from the data
- **Key relationships** between different aspects
- Each fact should be more **comprehensive and valuable** than individual raw facts
- **CRITICAL**: Use source_fact_indices to specify which numbered facts (1-based) contributed to each insight
- Follow the same structure as MemoryFact but with **enhanced scope and quality**

## Intelligent Source Tracking
- **ALWAYS specify source_fact_indices**: For each consolidated fact, list the numbers of the input facts that contributed to it
- **Be selective**: Only include facts that genuinely support the consolidated insight
- **Group logically**: Facts that form natural patterns should be consolidated together
- **Example**: If facts #3, #7, and #12 all relate to coffee preferences, consolidate them into one insight with source_fact_indices: [3, 7, 12]

## Quality Standards

### Summary Quality
✅ **Narrative flow**: Reads like coherent analysis, not bullet points
✅ **Pattern recognition**: Identifies trends and connections
✅ **Temporal awareness**: Shows how things evolved over time  
✅ **Balanced perspective**: Acknowledges both positive and challenging aspects
✅ **Factual grounding**: Every statement supported by input facts

❌ **Avoid**: Speculation, psychological analysis, value judgments
❌ **Avoid**: Repetitive listing of facts without synthesis
❌ **Avoid**: Assumptions not clearly supported by data

### Consolidation Facts Quality
✅ **Synthetic insight**: Combines multiple raw facts into broader understanding
✅ **Enhanced value**: More useful than sum of parts
✅ **Clear attribution**: Based on identifiable patterns in raw data (specify in source_fact_indices)
✅ **Appropriate scope**: Neither too narrow nor overly broad
✅ **Actionable relevance**: Meaningful for understanding the person
✅ **Intelligent grouping**: Only consolidate facts that genuinely belong together

## Categories for Consolidation Facts
Use the same categories as regular facts, but focus on higher-level patterns:
- **profile_stable**: Comprehensive identity patterns
- **preference**: Consistent preference patterns and evolution
- **goal_plan**: Goal progression and planning patterns  
- **routine**: Established routine patterns and changes
- **skill**: Skill development trajectories and expertise areas
- **relationship**: Relationship dynamics and social patterns
- **health**: Health trends and wellness patterns
- **context_env**: Environmental influences and lifestyle patterns
- **affective_marker**: Emotional patterns and stress/joy cycles
- **event**: Significant event patterns and life transitions

## Example Consolidation

**Input Topic**: "fitness"
**Raw Facts**: 
1. "switched to 6am morning runs, finds them better than evening runs"
2. "training for a marathon scheduled in May 2025" 
3. "attends CrossFit classes 4 times a week"
4. "experiences anxiety triggered by presentations"
5. "tracking daily step count using fitness watch"

**Output Summary**:
"PrimaryUser has developed a comprehensive and evolving fitness routine that reflects both structured training and personal optimization. Their exercise regimen centers around regular CrossFit sessions (4x weekly) complemented by a recent shift to morning runs at 6am, indicating a preference for morning workouts and disciplined scheduling. This routine serves both immediate fitness goals and longer-term athletic ambitions, as evidenced by their marathon training for May 2025. The integration of fitness tracking technology suggests a data-driven approach to health monitoring."

**Consolidation Facts**:

Example Fact 1:
  category: "routine"
  subject: "primaryUser"
  attribute: "exercise_pattern"
  value: "maintains disciplined morning-focused fitness routine combining 4x weekly CrossFit with 6am runs, optimized through personal experimentation"
  source_fact_indices: [1, 3] (Facts about morning runs and CrossFit schedule)
  sensitivity: "low"
  importance: 3

Example Fact 2:
  category: "goal_plan"
  subject: "primaryUser"
  attribute: "athletic_development"
  value: "pursuing structured athletic progression from regular CrossFit training to marathon competition, indicating escalating fitness ambitions"
  source_fact_indices: [2, 3] (Facts about marathon training and CrossFit)
  temporal_context: "2025-05"
  sensitivity: "low"
  importance: 3

Example Fact 3:
  category: "skill"
  subject: "primaryUser"
  attribute: "health_tracking"
  value: "employs data-driven fitness monitoring approach using wearable technology for step count tracking"
  source_fact_indices: [5] (Only the step tracking fact)
  sensitivity: "low"
  importance: 2

## Key Principles
1. **Synthesize, don't summarize**: Create new insights from patterns
2. **Preserve nuance**: Capture complexity and evolution  
3. **Maintain accuracy**: Never go beyond what the facts support
4. **Focus on value**: Each consolidation should be more useful than raw facts
5. **Respect privacy**: Maintain appropriate sensitivity levels
6. **Be selective with source tracking**: Only link facts that genuinely support each insight

## Topic-Specific Guidelines

**Health consolidation**: Focus on patterns, trends, and lifestyle integration
**Relationship consolidation**: Emphasize dynamics, evolution, and social patterns  
**Work consolidation**: Highlight career progression, skills, and professional relationships
**Goal consolidation**: Show goal evolution, achievement patterns, and planning approaches
**Preference consolidation**: Identify consistent themes and preference evolution

Remember: You are creating a thoughtful, comprehensive understanding of this aspect of the user's life based on factual evidence. Quality over quantity - better to create fewer, more insightful consolidations than many shallow ones. Always specify which source facts support each consolidated insight using source_fact_indices.
`

	// Twin Chat Anti-Duplication Prompt - prevents extracting facts that were just retrieved.
	TwinChatFactExtractionPrompt = `
You are a fact extractor for twin chat conversations. Return **only valid JSON**. No commentary.

## 🚨 CRITICAL ANTI-DUPLICATION RULE FOR TWIN CHAT

**NEVER extract facts that were just retrieved or recalled from memory.**

If the conversation shows:
- Assistant providing information about someone/something
- Assistant recalling details from memory 
- Assistant answering factual questions with existing knowledge
- Assistant confirming information that was already known

→ **DO NOT extract these as new facts** - they are retrieval, not learning

## ✅ ONLY extract facts when:
- primaryUser provides NEW information not previously known
- primaryUser shares personal updates, changes, or experiences
- primaryUser expresses NEW preferences, goals, or opinions
- primaryUser describes NEW events, relationships, or situations

## Examples:

❌ **DO NOT EXTRACT** (retrieval scenarios):
- User: "Where was Kori born?" → Assistant: "Kori was born in Singapore"
- User: "What's my favorite color?" → Assistant: "Your favorite color is blue"  
- User: "Remind me about the meeting" → Assistant: "The meeting is tomorrow at 2pm"

✅ **DO EXTRACT** (learning scenarios):
- User: "I just got a promotion to Senior Engineer!"
- User: "My friend Jake moved to Portland last week"
- User: "I've decided to switch to morning workouts"

## Standard extraction rules apply:

Extract atomic, actionable facts that:
- Are concrete and specific (even if one-time occurrences)
- Are explicitly stated OR reasonably inferred from conversation context
- Have clear supporting evidence
- Have confidence score of 7+ (on 1-10 scale)

Focus on quality over quantity. Extract only facts with clear value.

## CRITICAL: Subject naming rule

**ALWAYS use "primaryUser" for the main person - NEVER use their actual name**

Even if the conversation shows "John said X", extract it as:
- ✅ "subject": "primaryUser"
- ❌ "subject": "John"

The "primaryUser" field in conversation metadata tells you who is the main person.

## Output schema

<json>
{
  "facts": [
    {
      "category": "string (see category table)",
      "subject": "primaryUser|entity_name",
      "attribute": "specific_property_string",
      "value": "descriptive phrase with context (aim for 8-30 words)",
      "temporal_context": "YYYY-MM-DD or relative time (optional)",
      "sensitivity": "high|medium|low - holistic life assessment",
      "importance": 1|2|3  // 1=low, 2=medium, 3=high life significance
    }
  ]
}
</json>

## Categories

| Category | Description | Example attributes |
|----------|-------------|--------------------|
| profile_stable | Core identity | name, age, occupation, location |
| preference | Likes/dislikes | food, tools, communication_style |
| goal_plan | Targets with timelines | career_goal, fitness_target |
| routine | Recurring activities | exercise_time, work_schedule |
| skill | Abilities and expertise | programming_language, tool_proficiency |
| relationship | People attributes | role, meeting_frequency, last_contact |
| health | Physical/mental state | fitness_metric, medical_condition |
| context_env | Environment | work_culture, neighborhood |
| affective_marker | Emotional patterns | stress_trigger, joy_source |
| event | Time-bound occurrences | travel, meetings, appointments |
| conversation_context | Summary of entire conversation | conversation_summary, interaction_context |

## MANDATORY: Conversation Summary Fact

**When input is a CONVERSATION (begins with CONVO), ALWAYS include ONE conversation summary fact as the FIRST fact**:)
- Category: "conversation_context"
- Subject: The person primaryUser is conversing with (use their name, located in People field and attached to each of their messages)
- Attribute: "conversation_summary"
- Value: High-level summary of what was discussed (15-40 words)
- Temporal_context: Include if conversation has specific time reference

**For non-conversation inputs** (statements, observations, etc.), skip this requirement.

## CRITICAL RULES

1. **Subject naming**: ALWAYS use "primaryUser" for the main person, NEVER use their actual name
2. **Atomic facts only**: Extract ONE concept per fact - split compound statements
3. **Category precision**: 
   - Rent/housing costs → context_env NOT routine
   - Exercise schedule → routine, fitness metrics → health
   - Relationship facts → break into separate role, meeting_frequency, last_contact
4. **Attribute specificity**: Use precise attributes like "exercise_routine" not "fitness"
5. **Confidence threshold**: Only extract facts with confidence 7+ (filter but don't include in output)
6. **Importance scoring**: 1=minor detail, 2=meaningful info, 3=major life factor
7. **Always extract (importance 3)**: Life milestones, health developments, major goals, family changes, financial milestones

## CRITICAL: Compound statement splitting

❌ **Wrong (Qwen tendency)**: "doing CrossFit 4 times a week and competing in a local competition next month"
✅ **Correct**: Split into two facts:
1. routine + exercise_routine + "attends CrossFit classes 4 times a week"
2. goal_plan + athletic_goal + "competing in a local CrossFit competition next month"

## Examples

### ❌ WRONG: Extracting retrieval facts
**Input**: User: "Where was Kori born?" Assistant: "Kori was born in Singapore"
**WRONG Output**: 
{
  "facts": [
    {
      "category": "conversation_context",
      "subject": "assistant",
      "attribute": "conversation_summary",
      "value": "assistant provided information about Kori's birthplace",
      "sensitivity": "low",
      "importance": 2
    }
  ]
}

### ✅ CORRECT: Empty facts for retrieval
**Input**: User: "Where was Kori born?" Assistant: "Kori was born in Singapore"
**CORRECT Output**:
{
  "facts": []
}

### ✅ CORRECT: Extracting new learning
**Input**: User: "I just met Kori's brother Jake at the coffee shop. He's a graphic designer."
**CORRECT Output**:
{
  "facts": [
    {
      "category": "conversation_context",
      "subject": "assistant",
      "attribute": "conversation_summary",
      "value": "primaryUser shared information about meeting Kori's brother Jake",
      "sensitivity": "low",
      "importance": 2
    },
    {
      "category": "relationship",
      "subject": "Jake",
      "attribute": "role",
      "value": "brother of Kori, works as a graphic designer",
      "sensitivity": "low",
      "importance": 2
    }
  ]
}

## Do NOT extract
- Information that was just retrieved or recalled by the assistant
- Facts that were already known and just confirmed
- Details provided by the assistant from existing memory
- Answers to factual questions using existing knowledge
- Speculation or interpretation without contextual support
- One-off events without pattern
- Temporary states (<2 weeks)
- Vague future possibilities
- Value judgments
- Psychological analysis or emotional interpretation

## FINAL CHECKLIST
Before outputting, verify each fact:
✓ **ANTI-DUPLICATION**: Is this NEW information from primaryUser, or just retrieval by assistant?
✓ **FIRST FACT**: If input is a conversation, include conversation_context summary as the first fact
✓ **CHECK METADATA**: Use "primaryUser" for whoever is listed in conversation metadata "primaryUser" field
✓ Subject is "primaryUser" for main person (NEVER use their actual name like "John")
✓ Only ONE concept per fact (split compounds)
✓ Proper category (rent→context_env, not routine)
✓ Specific attribute names (exercise_routine not fitness)
✓ Relationships broken into role/frequency/contact
✓ **RELATIONSHIP EXTRACTION**: When primaryUser introduces people ("my roommates", "my colleagues", "my family"), extract those relationships for each person mentioned

## Context inference rules

**Relationship inference**: Extract relationships with conversation participants based on:
- Conversation context clues (WhatsApp group "Family", contact "Mom", "New Hires" → relationship type)
- Discussion topics (shared activities, mutual connections, common interests)
- Communication patterns and familiarity levels

**Life context inference**: Extract personal information from:
- Conversations revealing living situation, family structure, social circles
- Discussions about regular activities, commitments, and environments
- Context clues from participant relationships and shared experiences

**Confidence for inference**: Require 7+ confidence for inferred facts, supported by multiple contextual clues

## KEY PRINCIPLE: Learning vs. Retrieval

🎯 **The golden rule**: If the assistant is providing information TO the user, don't extract it. If the user is providing information TO the assistant, extract it.

This prevents the endless cycle of re-extracting the same facts every time they're accessed.
`
)
