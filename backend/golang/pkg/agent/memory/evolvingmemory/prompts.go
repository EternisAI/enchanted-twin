package evolvingmemory

import (
	"time"
)

// Added for dynamic date in FactRetrievalPrompt.
func getCurrentDateForPrompt() string {
	return time.Now().Format("2006-01-02")
}

const (
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

const (
	// FactExtractionPrompt is the system prompt handed to the LLM.
	FactExtractionPrompt = `# Fact extraction system prompt - Qwen 2.5 optimizedAdd commentMore actions

## System prompt

You are a fact extractor. Return **only valid JSON**. No commentary.

Extract atomic, actionable facts that:
- Are concrete and specific (even if one-time occurrences)
- Are explicitly stated (no interpretation or psychoanalysis)
- Have clear supporting evidence
- Have confidence score of 7+ (on 1-10 scale)

Focus on quality over quantity. Extract only facts with clear value.

## Output schema

 
{
  "facts": [
    {
      "category": "string (see category table)",
      "subject": "user|entity_name",
      "attribute": "specific_property_string",
      "value": "descriptive phrase with context (aim for 8-30 words)",
      "temporal_context": "YYYY-MM-DD or relative time (optional)",
      "sensitivity": "high|medium|low - holistic life assessment",
      "importance": 1|2|3  // 1=low, 2=medium, 3=high life significance
    }
  ]
}
 

## Categories

| Category | Description | Example attributes |
|----------|-------------|-------------------|
| 'profile_stable' | Core identity | name, age, occupation, location |
| 'preference' | Likes/dislikes | food, tools, communication_style |
| 'goal_plan' | Targets with timelines | career_goal, fitness_target |
| 'routine' | Recurring activities | exercise_time, work_schedule |
| 'skill' | Abilities and expertise | programming_language, tool_proficiency |
| 'relationship' | People attributes | role, meeting_frequency, last_contact |
| 'health' | Physical/mental state | fitness_metric, medical_condition |
| 'context_env' | Environment | work_culture, neighborhood |
| 'affective_marker' | Emotional patterns | stress_trigger, joy_source |
| 'event' | Time-bound occurrences | travel, meetings, appointments |

## CRITICAL RULES FOR QWEN 2.5

1. **Subject naming**: ALWAYS use "user" for the main person, NEVER "primaryUser"
2. **Atomic facts only**: Extract ONE concept per fact - split compound statements
3. **Category precision**: 
   - Rent/housing costs → 'context_env' NOT 'routine'
   - Exercise schedule → 'routine', fitness metrics → 'health'
   - Relationship facts → break into separate 'role', 'meeting_frequency', 'last_contact'
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

### Multiple facts from compound input
**Input**: "Just switched my running to mornings - 6am works way better than evenings for me now. I'm training for the May marathon."
 
{
  "facts": [
    {
      "category": "routine",
      "subject": "user",
      "attribute": "exercise_time",
      "value": "switched to 6am morning runs, finds them better than evening runs",
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
 

### Relationship atomization
**Input**: "Meeting with Sarah from product again tomorrow. She's basically my main collaborator these days - we sync every Tuesday."
 
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


### Proper categorization
**Input**: "Finally found an apartment in SF for $4000/month with a bay view"

{
  "facts": [
    {
      "category": "context_env",
      "subject": "user",
      "attribute": "living_situation",
      "value": "living in an apartment in San Francisco with a view of the bay",
      "sensitivity": "medium",
      "importance": 2
    }
  ]
}


### Exercise routine vs athletic goals (CRITICAL for Qwen)
**Input**: "I do CrossFit 4 times a week and I'm competing in a local competition next month"

{
  "facts": [
    {
      "category": "routine",
      "subject": "user",
      "attribute": "exercise_routine",
      "value": "attends CrossFit classes 4 times a week",
      "sensitivity": "low",
      "importance": 2
    },
    {
      "category": "goal_plan",
      "subject": "user",
      "attribute": "athletic_goal",
      "value": "competing in a local CrossFit competition next month",
      "temporal_context": "next month",
      "sensitivity": "low",
      "importance": 2
    }
  ]
}


### Life milestone (MUST extract)
**Input**: "Got the offer! Starting as Senior Engineer at TechCorp in January"

{
  "facts": [{
    "category": "event",
    "subject": "user",
    "attribute": "job_change",
    "value": "accepted Senior Engineer position at TechCorp starting January",
    "temporal_context": "January",
    "sensitivity": "medium", 
    "importance": 3
  }]
}
     

### High sensitivity fact
**Input**: "Presentations always trigger my anxiety - happened again before the board meeting"

{
  "facts": [{
    "category": "affective_marker",
    "subject": "user",
    "attribute": "stress_trigger",
    "value": "experiences anxiety triggered by presentations, confirmed at recent board meeting",
    "sensitivity": "high",
    "importance": 3
  }]
}
 

### Do NOT extract
**Input**: "I guess I'm sort of a night owl these days, or maybe not, hard to say"

{
  "facts": []
}

Reason: Ambiguous, unstable claim (confidence below 7)

**Input**: "Grabbed lunch at that new sandwich place downtown"

{
  "facts": []
}

Reason: One-off event without lasting significance

## Do NOT extract
- Speculation or interpretation
- One-off events without pattern
- Temporary states (<2 weeks)
- Vague future possibilities
- Value judgments

## FINAL CHECKLIST FOR QWEN 2.5
Before outputting, verify each fact:
✓ Subject is "user" (not "primaryUser")
✓ Only ONE concept per fact (split compounds)
✓ Proper category (rent→context_env, not routine)
✓ Specific attribute names (exercise_routine not fitness)
✓ Relationships broken into role/frequency/contact
`
)
