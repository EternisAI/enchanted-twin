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
	TextDocumentFactExtractionPrompt = `You are a Personal Information Organizer. Extract comprehensive facts about the primary user from the provided text content.

EXTRACT FACTS ABOUT THE PRIMARY USER FROM ANY TEXT CONTENT:

The text content may be:
- Written BY the primary user (emails, messages, posts they wrote)
- Written ABOUT the primary user (articles, reports, mentions by others)
- Content that MENTIONS the primary user (news, documents, conversations)
- Any text containing information related to the primary user

1. **DIRECT FACTS about the primary user**:
   - Personal information mentioned about the primary user
   - Actions the primary user took or plans to take
   - Preferences, opinions, feelings attributed to the primary user
   - Experiences the primary user had or described
   - Professional details, work-related information about the primary user
   - Health, physical states, or conditions of the primary user
   - Relationships, family, friends of the primary user
   - Places the primary user visited or is associated with
   - Activities, hobbies, interests of the primary user

2. **CONTEXTUAL FACTS about the primary user**:
   - Social context and relationships involving the primary user
   - Temporal references and timing of events related to the primary user
   - Plans, goals, or intentions attributed to the primary user
   - Reactions of the primary user to events or situations
   - Decision-making patterns shown by the primary user
   - Communication style and patterns of the primary user

EXTRACTION APPROACH:
1. **Be Thorough**: Scan the entire text for ANY mention or reference to the primary user
2. **Include Details**: Extract names, places, dates, activities, preferences related to the primary user
3. **Multiple Sources**: The text may mention the primary user from different perspectives (first-person, third-person, quoted)
4. **Preserve Attribution**: Note whether facts are stated by the primary user or about the primary user by others
5. **Temporal Context**: Include time references related to the primary user when mentioned

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
        }`
)
