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

## CRITICAL: Contact Document Handling

**For CONTACT ENTRIES (documents starting with "CONTACT ENTRY:" or with metadata "document_type": "contact_entry"):**

1. **These are NOT about the primaryUser** - they are entries from the user's contact list
2. **Only extract relationship facts** - never extract facts about primaryUser from contact data alone
3. **Subject should be the contact's name** - NOT "primaryUser"
4. **Relationship context**: Contact entries represent people in the user's network

You must, as always, use tool EXTRACT_FACTS to extract facts from contact documents.

### Contact Document Examples:
**Input**: "CONTACT ENTRY: Guillaume Doe (Phone: 0033222222222) - This is a contact from the user's Telegram contact list"
<json>
{
  "facts": [
    {
      "category": "relationship",
      "subject": "Guillaume Doe",
      "attribute": "contact_method",
      "value": "has phone number 0033222222222 in primaryUser's Telegram contacts",
      "sensitivity": "low",
      "importance": 1
    }
  ]
}
</json>

**Input**: "CONTACT ENTRY: Sarah Johnson - This is a contact from the user's Telegram contact list"
<json>
{
  "facts": [
    {
      "category": "relationship", 
      "subject": "Sarah Johnson",
      "attribute": "role",
      "value": "person in primaryUser's Telegram contact list",
      "sensitivity": "low",
      "importance": 1
    }
  ]
}
</json>

## Regular Conversation/Text Processing

For non-contact documents, continue with the standard rules:

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
)
