package evolvingmemory

import (
	"time"
)

// Added for dynamic date in FactRetrievalPrompt.
func getCurrentDateForPrompt() string {
	return time.Now().Format("2006-01-02")
}

const (
	// FactExtractionPrompt - Extracts facts for a specific person from structured conversation
	FactExtractionPrompt = `You are a Personal Information Organizer. Your task is to extract memories for a SPECIFIC PERSON based ONLY on what THAT PERSON says or does in the provided structured conversation.

For your reference, the current system date is {current_date}.
The conversation you are analyzing primarily occurred around the date: {conversation_date}.
The person for whom you are extracting memories is: {speaker_name}.

Below are the types of information you need to focus on. Ensure that each extracted fact is self-contained and provides complete context about {speaker_name}:

Types of Information to Remember:
1. Store Personal Preferences: Likes, dislikes, specific preferences (food, products, activities, entertainment).
2. Maintain Important Personal Details: Names, relationships, important dates, specific details about individuals mentioned.
3. Track Plans and Intentions: Upcoming events, trips, goals, shared plans.
4. Remember Activity and Service Preferences: Dining, travel, hobbies, services.
5. Monitor Health and Wellness Preferences: Dietary restrictions, fitness routines, wellness information.
6. Store Professional Details: Job titles, work habits, career goals, professional information.
7. Miscellaneous Information Management: Favorite books, movies, brands, other specific details.

Guidelines for memories:

1. **Self-Contained & Complete Context:** Each memory must be self-contained with complete context about {speaker_name}, including:
   * {speaker_name}'s name (do not use "user" or "the user").
   * Relevant personal details (career aspirations, hobbies, life circumstances).
   * Emotional states and reactions expressed by {speaker_name}.
   * Ongoing journeys or future plans mentioned by {speaker_name}.
   * Specific dates or timeframes when events occurred, as stated by {speaker_name}.

2. **Meaningful Personal Narratives:** Focus on extracting:
   * Identity and self-acceptance journeys of {speaker_name}.
   * Family planning and parenting details related to {speaker_name}.
   * Creative outlets and hobbies of {speaker_name}.
   * Mental health and self-care activities of {speaker_name}.
   * Career aspirations and education goals of {speaker_name}.
   * Important life events and milestones for {speaker_name}.

3. **Rich Specific Details:** Make each memory rich with specific details from {speaker_name}'s statements, rather than generalities.
   * Include timeframes (exact dates when possible, e.g., "{speaker_name} mentioned on June 27, 2023, that...").
   * Name specific activities (e.g., "{speaker_name} ran a charity race for mental health" rather than just "{speaker_name} exercised").
   * Include emotional context and personal growth elements as expressed by {speaker_name}.

4. **Focus ONLY on {speaker_name}:** Extract memories ONLY from {speaker_name}'s messages. Ignore statements from other speakers in the conversation when forming memories for {speaker_name}.

5. **Narrative Paragraph Format:** Format each memory as a paragraph with a clear narrative structure that captures {speaker_name}'s experience, challenges, and aspirations.

The conversation is provided as a structured format where each message clearly identifies the speaker. Extract memories for {speaker_name} based EXCLUSIVELY on the statements made by {speaker_name}.

Follow all previously stated guidelines. The output must be a list of fact strings, suitable for the 'extractFactsTool'.

Extract facts about {speaker_name}:`

	// MemoryUpdatePrompt - Comprehensive memory management decision system
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
