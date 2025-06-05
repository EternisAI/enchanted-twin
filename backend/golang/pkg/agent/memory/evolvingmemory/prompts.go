package evolvingmemory

import (
	"time"
)

// Added for dynamic date in FactRetrievalPrompt.
func getCurrentDateForPrompt() string {
	return time.Now().Format("2006-01-02")
}

const (
	// FactExtractionPrompt is the system prompt handed to the LLM.
	FactExtractionPrompt = `
You are a fact extractor. Return **only valid JSON**. No commentary.

Extract atomic, actionable facts that:
- Are concrete and specific (even if one-time occurrences)
- Are explicitly stated (no interpretation or psychoanalysis)
- Have clear supporting evidence
- Have confidence score of 7+ (on 1-10 scale)

Focus on quality over quantity. Extract only facts with clear value.

## Extraction categories

Personal facts about the user

- Core identity (name, age, location, occupation)
- Preferences (food, music, activities, communication style)
- Values and beliefs
- Goals and aspirations
- Challenges and pain points
- Routines and habits
- Skills and expertise

Relationship mapping

- Key people in their life (family, friends, colleagues)
- Relationship dynamics and quality
- Shared activities and contexts
- Communication patterns with different people

Temporal patterns

- Daily / weekly routines
- Seasonal patterns
- Life phases and transitions
- Project timelines
- Recurring events

Emotional and cognitive patterns

- Stress triggers and coping mechanisms
- Sources of joy and fulfillment
- Decision-making patterns
- Learning preferences
- Communication style variations by context

Context and environment

- Work environment and culture
- Living situation
- Geographic preferences
- Digital tool usage patterns

## Output schema
` + "```json\n" + `
{
  "facts": [
    {
      "category": "string (see category table)",
      "subject": "user|entity_name",
      "attribute": "specific_property_string",
      "value": "descriptive phrase with context (aim for 8-30 words)",
      "temporal_context": "YYYY-MM-DD or relative time (optional)",
      "sensitivity": "high|medium|low - holistic life assessment",
      "importance": 1|2|3
    }
  ]
}
` + "```\n\n" + `
## Categories

| Category         | Description             | Example attributes                     |
|------------------|-------------------------|----------------------------------------|
| profile_stable   | Core identity           | name, age, occupation, location        |
| preference       | Likes / dislikes        | food, tools, communication_style       |
| goal_plan        | Targets with timelines  | career_goal, fitness_target            |
| routine          | Recurring activities    | exercise_time, work_schedule           |
| skill            | Abilities and expertise | programming_language, tool_proficiency |
| relationship     | People attributes       | role, meeting_frequency, last_contact  |
| health           | Physical / mental state | fitness_metric, medical_condition      |
| context_env      | Environment             | work_culture, neighborhood             |
| affective_marker | Emotional patterns      | stress_trigger, joy_source             |
| event            | Time-bound occurrences  | travel, meetings, appointments         |

## Extraction rules

1. **Atomic facts only**: "Prefers Thai food" not "likes Asian cuisine"
2. **No speculation**: Skip "seems stressed" → require "I'm stressed"
3. **Source conflicts**: Use most recent explicit self-statement
4. **Relationships**: Emit separate atomic facts for each attribute (role, meeting_frequency, last_contact)
5. **Confidence threshold**: Only extract facts with confidence 7+ (on 1-10 scale) – filter but don’t include in output
6. **Sensitivity assessment**: Consider impact across all life domains (personal, professional, social, health, financial)
7. **Importance scoring**:  
   - 1 = Minor detail worth noting  
   - 2 = Meaningful information affecting decisions / relationships  
   - 3 = Major life factor with significant ongoing impact
8. **Time format**: Use 24-hour format (06:00, 14:30)
9. **Skip mundane things**: Facts must be worth remembering over time

## What to ALWAYS extract (importance 3):
- Life milestones: moving, job changes, relationship status changes, major purchases
- Health developments: diagnoses, significant fitness achievements, medical procedures
- Major goals / commitments: training for events, education plans, career targets
- Family changes: new family members, deaths, major family events
- Financial milestones: home purchases, debt payoff, major investments

## Granularity guide

❌ Too coarse: "User is ambitious"  
✅ Just right: "Targets promotion to Senior Engineer by Q3 2025"  
❌ Too fine: "Ate sandwich at 12:47"

## Examples

### Multiple facts from one input
Input: "Just switched my running to mornings – 6 am works way better than evenings for me now. I'm training for the May marathon."
` + "```json\n" + `
{
  "facts": [
    {
      "category": "routine",
      "subject": "user",
      "attribute": "exercise_time",
      "value": "switched to 6 am morning runs, finds them better than evening runs",
      "sensitivity": "low",
      "importance": 2
    },
    {
      "category": "goal_plan",
      "subject": "user",
      "attribute": "athletic_goal",
      "value": "training for a marathon scheduled in May 2025",
      "temporal_context": "2025-05",
      "sensitivity": "low",
      "importance": 3
    }
  ]
}
` + "```\n\n" + `
### Relationship with multiple attributes
Input: "Meeting with Sarah from product again tomorrow. She's basically my main collaborator these days – we sync every Tuesday."
` + "```json\n" + `
{
  "facts": [
    {
      "category": "relationship",
      "subject": "Sarah",
      "attribute": "role",
      "value": "product team member who is user's main collaborator",
      "sensitivity": "low",
      "importance": 2
    },
    {
      "category": "relationship",
      "subject": "Sarah",
      "attribute": "meeting_frequency",
      "value": "syncs with user every Tuesday for regular collaboration",
      "sensitivity": "low",
      "importance": 2
    }
  ]
}
` + "```\n\n" + `
### Health fact (moderate sensitivity)
Input: "Crushed my 10 k run today in 48 minutes! My VO2 max is up to 52 according to my watch"
` + "```json\n" + `
{
  "facts": [
    {
      "category": "health",
      "subject": "user",
      "attribute": "10k_time",
      "value": "completed 10 k run in 48 minutes showing strong fitness level",
      "temporal_context": "today",
      "sensitivity": "medium",
      "importance": 2
    },
    {
      "category": "health",
      "subject": "user",
      "attribute": "vo2_max",
      "value": "VO2 max measured at 52 by fitness watch indicating good cardiovascular fitness",
      "sensitivity": "medium",
      "importance": 2
    }
  ]
}
` + "```\n\n" + `
### Affective marker with high sensitivity
Input: "Presentations always trigger my anxiety – happened again before the board meeting"
` + "```json\n" + `
{
  "facts": [
    {
      "category": "affective_marker",
      "subject": "user",
      "attribute": "stress_trigger",
      "value": "experiences anxiety triggered by presentations, confirmed at recent board meeting",
      "sensitivity": "high",
      "importance": 3
    }
  ]
}
` + "```\n\n" + `
### Negative example (no extraction)
Input: "I guess I'm sort of a night owl these days, or maybe not, hard to say"
` + "```json\n" + `
{ "facts": [] }
` + "```\n" + `
Reason: Ambiguous, unstable claim (confidence below 7)

### Life milestone example (MUST extract)
Input: "Finally signed the lease! Moving to Brooklyn next month"
` + "```json\n" + `
{
  "facts": [
    {
      "category": "event",
      "subject": "user",
      "attribute": "relocation",
      "value": "moving to Brooklyn with lease signed",
      "temporal_context": "next month",
      "sensitivity": "medium",
      "importance": 3
    }
  ]
}
` + "```\n\n" + `
### Job change example (MUST extract)
Input: "Got the offer! Starting as Senior Engineer at TechCorp in January"
` + "```json\n" + `
{
  "facts": [
    {
      "category": "event",
      "subject": "user",
      "attribute": "job_change",
      "value": "accepted Senior Engineer position at TechCorp starting January",
      "temporal_context": "January",
      "sensitivity": "medium",
      "importance": 3
    }
  ]
}
` + "```\n\n" + `
### Health milestone example (MUST extract)
Input: "Doctor confirmed I'm fully recovered from the surgery – cleared for all activities"
` + "```json\n" + `
{
  "facts": [
    {
      "category": "health",
      "subject": "user",
      "attribute": "recovery_status",
      "value": "fully recovered from surgery with doctor clearance for all activities",
      "sensitivity": "high",
      "importance": 3
    }
  ]
}
` + "```\n\n" + `
### Major purchase example (MUST extract)
Input: "Just bought my first house! Closing was yesterday, keys in hand"
` + "```json\n" + `
{
  "facts": [
    {
      "category": "event",
      "subject": "user",
      "attribute": "home_purchase",
      "value": "purchased first house with closing completed",
      "temporal_context": "yesterday",
      "sensitivity": "medium",
      "importance": 3
    }
  ]
}
` + "```\n\n" + `
### Mundane examples (DO NOT extract)
Input: "Grabbed lunch at that new sandwich place downtown"
` + "```json\n" + `
{ "facts": [] }
` + "```\n" + `
Reason: One-off dining experience without lasting significance

Input: "Feeling pretty tired today, long week"
` + "```json\n" + `
{ "facts": [] }
` + "```\n" + `
Reason: Temporary state, not a lasting pattern or significant development

Input: "Thinking I might want to learn Spanish someday"
` + "```json\n" + `
{ "facts": [] }
` + "```\n" + `
Reason: Vague consideration without commitment or concrete plans

## Chunk metadata (DO NOT OUTPUT)
` + "```json\n" + `
{
  "chunk_id": "conv_12345_chunk_3",
  "previous_facts_hash": "a7b9c2...",
  "timestamp": "2024-03-15T10:30:00Z"
}
` + "```\n\n" + `
## Do NOT extract
- Speculation or interpretation
- One-off events without pattern
- Quotes from others about the user
- Temporary states (<2 weeks)
- Granular timestamps
- Value judgments
`
	// MemoryUpdatePrompt - Comprehensive memory management decision system for conversations.
	MemoryUpdatePrompt = `You are a smart memory manager which controls the memory of a system for the primary user.
You can perform four operations: (1) add into the memory, (2) update the memory, (3) delete from the memory, and (4) no change.

Compare newly retrieved facts with the existing memory. For each new fact, decide whether to:
- ADD: Add it to the memory as a new element
- UPDATE: Update an existing memory element
- DELETE: Delete an existing memory element
- NONE: Make no change (if the fact is already present or irrelevant)

There are specific guidelines to select which operation to perform:

1. **Add**: If the retrieved facts contain new information not present in the memory, then you have to add it.
- **Example**:
    - Old Memory:
        [
            {
                "id" : "0",
                "text" : "The primary user is a software engineer"
            }
        ]
    - Retrieved facts: ["The primary user's name is John"]
    - New Memory:
        {
            "memory" : [
                {
                    "id" : "0",
                    "text" : "The primary user is a software engineer",
                    "event" : "NONE"
                },
                {
                    "id" : "1",
                    "text" : "The primary user's name is John",
                    "event" : "ADD"
                }
            ]
        }

2. **Update**: If the retrieved facts contain information that is already present in the memory but the information is totally different, then you have to update it. 
If the retrieved fact contains information that conveys the same thing as the elements present in the memory, then you have to keep the fact which has the most information. 
Example (a) -- if the memory contains "The primary user likes to play cricket" and the retrieved fact is "The primary user loves to play cricket with friends", then update the memory with the retrieved facts.
Example (b) -- if the memory contains "The primary user likes cheese pizza" and the retrieved fact is "The primary user loves cheese pizza", then you do not need to update it because they convey the same information.
Please keep in mind while updating you have to keep the same ID.
- **Example**:
    - Old Memory:
        [
            {
                "id" : "0",
                "text" : "The primary user really likes cheese pizza"
            },
            {
                "id" : "1",
                "text" : "The primary user is a software engineer"
            },
            {
                "id" : "2",
                "text" : "The primary user likes to play cricket"
            }
        ]
    - Retrieved facts: ["The primary user loves chicken pizza", "The primary user loves to play cricket with friends"]
    - New Memory:
        {
        "memory" : [
                {
                    "id" : "0",
                    "text" : "The primary user loves cheese and chicken pizza",
                    "event" : "UPDATE",
                    "old_memory" : "The primary user really likes cheese pizza"
                },
                {
                    "id" : "1",
                    "text" : "The primary user is a software engineer",
                    "event" : "NONE"
                },
                {
                    "id" : "2",
                    "text" : "The primary user loves to play cricket with friends",
                    "event" : "UPDATE",
                    "old_memory" : "The primary user likes to play cricket"
                }
            ]
        }

3. **Delete**: If the retrieved facts contain information that contradicts the information present in the memory, then you have to delete it.
- **Example**:
    - Old Memory:
        [
            {
                "id" : "0",
                "text" : "The primary user's name is John"
            },
            {
                "id" : "1",
                "text" : "The primary user loves cheese pizza"
            }
        ]
    - Retrieved facts: ["The primary user dislikes cheese pizza"]
    - New Memory:
        {
        "memory" : [
                {
                    "id" : "0",
                    "text" : "The primary user's name is John",
                    "event" : "NONE"
                },
                {
                    "id" : "1",
                    "text" : "The primary user loves cheese pizza",
                    "event" : "DELETE"
                }
        ]
        }

4. **No Change**: If the retrieved facts contain information that is already present in the memory, then you do not need to make any changes.
- **Example**:
    - Old Memory:
        [
            {
                "id" : "0",
                "text" : "The primary user's name is John"
            },
            {
                "id" : "1",
                "text" : "The primary user loves cheese pizza"
            }
        ]
    - Retrieved facts: ["The primary user's name is John"]
    - New Memory:
        {
        "memory" : [
                {
                    "id" : "0",
                    "text" : "The primary user's name is John",
                    "event" : "NONE"
                },
                {
                    "id" : "1",
                    "text" : "The primary user loves cheese pizza",
                    "event" : "NONE"
                }
            ]
        }

Based on the guidelines above, analyze the provided context and decide what action should be taken for the new fact.
Use the appropriate tool to indicate your decision.
`
)
