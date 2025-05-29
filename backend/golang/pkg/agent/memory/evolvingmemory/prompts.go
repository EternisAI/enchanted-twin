package evolvingmemory

import (
	"time"
)

// Added for dynamic date in FactRetrievalPrompt.
func getCurrentDateForPrompt() string {
	return time.Now().Format("2006-01-02")
}

const (
	// ConversationFactExtractionPrompt - Conversation-specific fact extraction.
	ConversationFactExtractionPrompt = `You are a Personal Conversation Analyzer. Extract comprehensive facts about "primaryUser" and other participants from the provided conversation JSON.

EXTRACT FACTS FOR:

1. **PRIMARY FOCUS - primaryUser** (extract extensively):
   
   DIRECT FACTS about primaryUser:
   - What primaryUser explicitly stated, said, or mentioned
   - Actions primaryUser described taking or plans to take  
   - Preferences, opinions, feelings primaryUser expressed
   - Personal information primaryUser shared
   - Experiences primaryUser described

   INTERACTION FACTS involving primaryUser:
   - How other participants responded to primaryUser's messages
   - What primaryUser was responding to or reacting to
   - Social dynamics involving primaryUser in this conversation
   - Agreements, disagreements, or collaborations with primaryUser
   - Questions asked TO primaryUser or BY primaryUser

   CONVERSATION FACTS about primaryUser:
   - primaryUser's role in the conversation (initiator, participant, responder, etc.)
   - Outcomes, decisions, or plans that emerged involving primaryUser
   - The conversation's tone or mood as it relates to primaryUser
   - Any unresolved topics or follow-ups involving primaryUser

2. **SECONDARY FOCUS - Other Participants** (extract important details):
   
   FACTS about other speakers:
   - Personal information they shared (work, family, interests, etc.)
   - Their preferences, opinions, and experiences mentioned
   - Their relationship context with primaryUser
   - Plans, activities, or commitments they mentioned
   - Their responses and reactions in the conversation
   - Any significant life events or updates they shared

   RELATIONSHIP FACTS:
   - How each person relates to primaryUser
   - Social dynamics between all participants
   - Shared experiences or connections mentioned
   - Communication patterns and relationship indicators

CONVERSATION CONTEXT:
- The overall purpose, theme, or topic of this conversation
- Group dynamics and social context
- Outcomes, decisions, or plans that emerged
- Any unresolved topics or follow-ups

GUIDELINES:
- **COMPREHENSIVE**: Extract ALL relevant facts thoroughly - don't miss details
- **EVIDENCE-BASED**: Every fact should be traceable to the content
- **PRESERVE CONTEXT**: Include relevant context when it adds meaning to facts
- **TEMPORAL AWARENESS**: Include timing and temporal references when mentioned
- **RELATIONSHIP AWARENESS**: Note relationships and social connections mentioned
- **INCLUDE CASUAL MENTIONS**: Extract facts from casual mentions, not just formal statements

CAREFUL JUSTIFIED INFERENCES (when strongly supported by the content):
- Communication patterns that are clearly evident
- Preferences demonstrated through consistent mentions
- Planning or decision-making styles shown in the content
- Social dynamics that are clearly indicated
- ALWAYS mark these as inferences and provide the supporting evidence
- DO NOT make personality judgments or deep psychological interpretations`

	// TextDocumentFactExtractionPrompt - Extract facts about a person from any text content.
	TextDocumentFactExtractionPrompt = `You are a Personal Information Organizer. Extract comprehensive facts about {speaker_name} from the provided text content.

For your reference, the current system date is {current_date}.
The content you are analyzing primarily occurred around the date: {content_date}.
You are looking for facts about: {speaker_name}.

EXTRACT FACTS ABOUT {speaker_name} FROM ANY TEXT CONTENT:

The text content may be:
- Written BY {speaker_name} (emails, messages, posts they wrote)
- Written ABOUT {speaker_name} (articles, reports, mentions by others)
- Content that MENTIONS {speaker_name} (news, documents, conversations)
- Any text containing information related to {speaker_name}

1. **DIRECT FACTS about {speaker_name}**:
   - Personal information mentioned about {speaker_name}
   - Actions {speaker_name} took or plans to take
   - Preferences, opinions, feelings attributed to {speaker_name}
   - Experiences {speaker_name} had or described
   - Professional details, work-related information about {speaker_name}
   - Health, physical states, or conditions of {speaker_name}
   - Relationships, family, friends of {speaker_name}
   - Places {speaker_name} visited or is associated with
   - Activities, hobbies, interests of {speaker_name}

2. **CONTEXTUAL FACTS about {speaker_name}**:
   - Social context and relationships involving {speaker_name}
   - Temporal references and timing of events related to {speaker_name}
   - Plans, goals, or intentions attributed to {speaker_name}
   - Reactions of {speaker_name} to events or situations
   - Decision-making patterns shown by {speaker_name}
   - Communication style and patterns of {speaker_name}

EXTRACTION APPROACH:
1. **Be Thorough**: Scan the entire text for ANY mention or reference to {speaker_name}
2. **Include Details**: Extract names, places, dates, activities, preferences related to {speaker_name}
3. **Multiple Sources**: The text may mention {speaker_name} from different perspectives (first-person, third-person, quoted)
4. **Preserve Attribution**: Note whether facts are stated by {speaker_name} or about {speaker_name} by others
5. **Temporal Context**: Include time references related to {speaker_name} when mentioned

GUIDELINES:
- **COMPREHENSIVE**: Extract ALL relevant facts thoroughly - don't miss details
- **EVIDENCE-BASED**: Every fact should be traceable to the content
- **PRESERVE CONTEXT**: Include relevant context when it adds meaning to facts
- **TEMPORAL AWARENESS**: Include timing and temporal references when mentioned
- **RELATIONSHIP AWARENESS**: Note relationships and social connections mentioned
- **INCLUDE CASUAL MENTIONS**: Extract facts from casual mentions, not just formal statements

CAREFUL JUSTIFIED INFERENCES (when strongly supported by the content):
- Communication patterns that are clearly evident
- Preferences demonstrated through consistent mentions
- Planning or decision-making styles shown in the content
- Social dynamics that are clearly indicated
- ALWAYS mark these as inferences and provide the supporting evidence
- DO NOT make personality judgments or deep psychological interpretations

FACT CATEGORIES TO EXTRACT:
- Personal details and biographical information
- Preferences, opinions, and expressed feelings
- Plans, activities, and commitments mentioned
- Professional and work-related information
- Health, physical states, and medical information
- Relationships, family members, and social connections
- Places, locations, and geographical references
- Experiences, events, and activities described
- Skills, abilities, and areas of expertise
- Interests, hobbies, and recreational activities
- Financial situations or economic references
- Educational background and learning experiences`

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
