package evolvingmemory

import (
	"time"
)

// Added for dynamic date in FactRetrievalPrompt.
func getCurrentDateForPrompt() string {
	return time.Now().Format("2006-01-02")
}

const (
	// FactExtractionPrompt - Extracts facts for a specific person from structured conversation.
	FactExtractionPrompt = `You are a Personal Information Organizer. Your task is to extract ONLY directly observable facts for a SPECIFIC PERSON based EXCLUSIVELY on what THAT PERSON explicitly states in the provided conversation.

For your reference, the current system date is {current_date}.
The conversation you are analyzing primarily occurred around the date: {conversation_date}.
The person for whom you are extracting memories is: {speaker_name}.

CRITICAL RULES - NEVER VIOLATE THESE:

1. **ONLY EXTRACT WHAT IS EXPLICITLY STATED**: Extract ONLY information that {speaker_name} directly and explicitly says. Do NOT infer, interpret, or extrapolate anything.

2. **NO EMOTIONAL INTERPRETATION**: Do NOT interpret emotions, feelings, or psychological states unless {speaker_name} explicitly states them using clear emotional language (e.g., "I am happy", "I feel sad").

3. **NO ASSUMPTIONS ABOUT RELATIONSHIPS**: Do NOT assume relationship types, family connections, or social dynamics unless explicitly stated by {speaker_name}.

4. **NO FUTURE PREDICTIONS**: Do NOT extract intentions, plans, or goals unless {speaker_name} explicitly states them as definite plans (e.g., "I will do X tomorrow").

5. **NO CONTEXT FILLING**: Do NOT add context, background, or explanatory details that {speaker_name} did not explicitly provide.

6. **DIRECT QUOTES ONLY**: Each fact should be based on something {speaker_name} directly said, not your interpretation of what they meant.

EXTRACTION REQUIREMENTS - BE THOROUGH:

1. **EXTRACT EVERYTHING STATED**: Be comprehensive and thorough. Extract EVERY piece of factual information that {speaker_name} explicitly mentions. Do not be overly restrictive or miss obvious facts.

2. **CAPTURE ALL EXPLICIT DETAILS**: If {speaker_name} mentions names, places, dates, activities, preferences, experiences, or any other concrete details, extract them ALL.

3. **INCLUDE CASUAL MENTIONS**: Extract facts from casual mentions, not just formal statements. If {speaker_name} says "I grabbed coffee at Starbucks" - extract that they went to Starbucks.

4. **CAPTURE TEMPORAL REFERENCES**: If {speaker_name} mentions "yesterday", "last week", "when I was young", etc., include these temporal references as stated.

5. **EXTRACT COMPOUND STATEMENTS**: If {speaker_name} says multiple things in one sentence, extract each distinct fact separately.

Types of Information to Extract (ONLY if explicitly stated):
- Direct statements about preferences: "I like pizza", "I don't enjoy running", "I hate mornings"
- Explicit personal details: "My name is John", "I work as a teacher", "I live in Seattle"
- Concrete plans with specific details: "I'm going to Paris next week", "I have a meeting tomorrow"
- Factual statements about activities: "I went to the gym yesterday", "I bought groceries"
- Direct statements about health: "I am allergic to peanuts", "I have a headache"
- Explicit professional information: "I got promoted to manager", "I work at Google"
- People mentioned: "I talked to Sarah", "My brother called me"
- Places mentioned: "I went to the store", "I was at the office"
- Experiences described: "I watched a movie", "I had dinner", "I took a walk"
- Opinions expressed: "I think the weather is nice", "I believe this is wrong"
- Current states: "I am tired", "I am at home", "I am hungry"

Guidelines for fact extraction:

1. **Verbatim Accuracy**: Each extracted fact must be directly traceable to something {speaker_name} explicitly said.

2. **No Narrative Construction**: Do NOT create stories or narratives. Extract discrete, standalone facts only.

3. **Include Speaker Name**: Start each fact with "{speaker_name}" but do NOT add interpretive context.

4. **Preserve Exact Meaning**: Do NOT rephrase or interpret. Stay as close to the original statement as possible.

5. **When in Doubt, Don't Extract**: If you're unsure whether something was explicitly stated or if you're interpreting, do NOT extract it.

6. **No Temporal Assumptions**: Only include dates/times if {speaker_name} explicitly mentioned them.

7. **BE COMPREHENSIVE**: Go through {speaker_name}'s messages systematically and extract EVERY explicit fact. Don't be lazy or cursory.

EXAMPLES OF WHAT NOT TO DO:
- ❌ "John seems to be going through a difficult time" (interpretation)
- ❌ "John is passionate about his career" (inference from enthusiasm)
- ❌ "John values family relationships" (assumption from context)
- ❌ "John is planning to improve his health" (extrapolation from gym mention)

EXAMPLES OF WHAT TO DO:
- ✅ "John said he likes pizza"
- ✅ "John mentioned he works as a software engineer"
- ✅ "John stated he is going to the gym tomorrow"
- ✅ "John said he feels tired today"
- ✅ "John mentioned he talked to his mom yesterday"
- ✅ "John said he bought coffee at Starbucks"
- ✅ "John stated he lives in San Francisco"
- ✅ "John mentioned he has a meeting at 3pm"

The conversation is provided as a structured format where each message clearly identifies the speaker. Extract memories for {speaker_name} based EXCLUSIVELY on the direct, explicit statements made by {speaker_name}.

BE THOROUGH AND COMPREHENSIVE. Extract EVERY explicit fact that {speaker_name} states. Do not miss anything that is actually there.

Extract ONLY directly observable facts about {speaker_name}:`

	// MemoryUpdatePrompt - Comprehensive memory management decision system.
	MemoryUpdatePrompt = `You are a smart memory manager which controls the memory of a system for {speaker_name}.
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
                "text" : "{speaker_name} is a software engineer"
            }
        ]
    - Retrieved facts: ["{speaker_name}'s name is John"]
    - New Memory:
        {
            "memory" : [
                {
                    "id" : "0",
                    "text" : "{speaker_name} is a software engineer",
                    "event" : "NONE"
                },
                {
                    "id" : "1",
                    "text" : "{speaker_name}'s name is John",
                    "event" : "ADD"
                }
            ]
        }

2. **Update**: If the retrieved facts contain information that is already present in the memory but the information is totally different, then you have to update it. 
If the retrieved fact contains information that conveys the same thing as the elements present in the memory, then you have to keep the fact which has the most information. 
Example (a) -- if the memory contains "{speaker_name} likes to play cricket" and the retrieved fact is "{speaker_name} loves to play cricket with friends", then update the memory with the retrieved facts.
Example (b) -- if the memory contains "{speaker_name} likes cheese pizza" and the retrieved fact is "{speaker_name} loves cheese pizza", then you do not need to update it because they convey the same information.
Please keep in mind while updating you have to keep the same ID.
- **Example**:
    - Old Memory:
        [
            {
                "id" : "0",
                "text" : "{speaker_name} really likes cheese pizza"
            },
            {
                "id" : "1",
                "text" : "{speaker_name} is a software engineer"
            },
            {
                "id" : "2",
                "text" : "{speaker_name} likes to play cricket"
            }
        ]
    - Retrieved facts: ["{speaker_name} loves chicken pizza", "{speaker_name} loves to play cricket with friends"]
    - New Memory:
        {
        "memory" : [
                {
                    "id" : "0",
                    "text" : "{speaker_name} loves cheese and chicken pizza",
                    "event" : "UPDATE",
                    "old_memory" : "{speaker_name} really likes cheese pizza"
                },
                {
                    "id" : "1",
                    "text" : "{speaker_name} is a software engineer",
                    "event" : "NONE"
                },
                {
                    "id" : "2",
                    "text" : "{speaker_name} loves to play cricket with friends",
                    "event" : "UPDATE",
                    "old_memory" : "{speaker_name} likes to play cricket"
                }
            ]
        }

3. **Delete**: If the retrieved facts contain information that contradicts the information present in the memory, then you have to delete it.
- **Example**:
    - Old Memory:
        [
            {
                "id" : "0",
                "text" : "{speaker_name}'s name is John"
            },
            {
                "id" : "1",
                "text" : "{speaker_name} loves cheese pizza"
            }
        ]
    - Retrieved facts: ["{speaker_name} dislikes cheese pizza"]
    - New Memory:
        {
        "memory" : [
                {
                    "id" : "0",
                    "text" : "{speaker_name}'s name is John",
                    "event" : "NONE"
                },
                {
                    "id" : "1",
                    "text" : "{speaker_name} loves cheese pizza",
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
                "text" : "{speaker_name}'s name is John"
            },
            {
                "id" : "1",
                "text" : "{speaker_name} loves cheese pizza"
            }
        ]
    - Retrieved facts: ["{speaker_name}'s name is John"]
    - New Memory:
        {
        "memory" : [
                {
                    "id" : "0",
                    "text" : "{speaker_name}'s name is John",
                    "event" : "NONE"
                },
                {
                    "id" : "1",
                    "text" : "{speaker_name} loves cheese pizza",
                    "event" : "NONE"
                }
            ]
        }

Context for {speaker_name}:

Existing memories:
{existing_memories}

New fact to consider:
{new_fact}

Based on the guidelines and context, what action should be taken for the NEW FACT?`
)
