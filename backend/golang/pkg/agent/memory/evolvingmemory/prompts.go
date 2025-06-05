package evolvingmemory

import (
	"time"
)

// Added for dynamic date in FactRetrievalPrompt.
func getCurrentDateForPrompt() string {
	return time.Now().Format("2006-01-02")
}

const (
	// FactExtractionPrompt - Extract facts about a person from any text content.
	FactExtractionPrompt = `You are a fact extractor. Return **only valid JSON**. No commentary.

    Extract atomic, actionable facts that:
    - Are concrete and specific (even if one-time occurrences)
    - Are explicitly stated (no interpretation or psychoanalysis)
    - Have clear supporting evidence
    - Have confidence score of 7+ (on 1-10 scale)
    
    Focus on quality over quantity. Extract only facts with clear value.
    
    ## Output schema
    ` + "```json" + `
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
    ` + "```" + `
    
    ## Categories
    
    | Category        | Description             | Example attributes                     |
    |-----------------|-------------------------|----------------------------------------|
    | profile_stable  | Core identity           | name, age, occupation, location        |
    | preference      | Likes/dislikes          | food, tools, communication_style       |
    | goal_plan       | Targets with timelines  | career_goal, fitness_target            |
    | routine         | Recurring activities    | exercise_time, work_schedule           |
    | skill           | Abilities and expertise | programming_language, tool_proficiency |
    | relationship    | People attributes       | role, meeting_frequency, last_contact  |
    | health          | Physical/mental state   | fitness_metric, medical_condition      |
    | context_env     | Environment             | work_culture, neighborhood             |
    | affective_marker| Emotional patterns      | stress_trigger, joy_source             |
    | event           | Time-bound occurrences  | travel, meetings, appointments         |
    
    ## Extraction rules
    
    1. **Atomic facts only**: "Prefers Thai food" not "likes Asian cuisine"
    2. **No speculation**: Skip "seems stressed" → require "I'm stressed"
    3. **Source conflicts**: Use most recent explicit self-statement
    4. **Relationships**: Emit separate atomic facts for each attribute (role, meeting_frequency, last_contact)
    5. **Confidence threshold**: Only extract facts with confidence 7+ (on 1-10 scale) – filter but don’t include in output
    6. **Sensitivity assessment**: Consider impact across all life domains (personal, professional, social, health, financial)
    7. **Importance scoring**:
       - 1 = Minor detail worth noting
       - 2 = Meaningful information affecting decisions/relationships
       - 3 = Major life factor with significant ongoing impact
    8. **Time format**: Use 24-hour format (06:00, 14:30)
    9. **Skip mundane things**: Facts must be worth remembering over time
    
    ## What to ALWAYS extract (importance 3):
    - **Life milestones**: Moving, job changes, relationship status changes, major purchases
    - **Health developments**: Diagnoses, significant fitness achievements, medical procedures
    - **Major goals/commitments**: Training for events, education plans, career targets
    - **Family changes**: New family members, deaths, major family events
    - **Financial milestones**: Home purchases, debt payoff, major investments
    
    ## Granularity guide
    
    ❌ **Too coarse**: "User is ambitious"  
    ✅ **Just right**: "Targets promotion to Senior Engineer by Q3 2025"  
    ❌ **Too fine**: "Ate sandwich at 12:47"
    
    ## Examples
    
    ### Multiple facts from one input
    **Input**: "Just switched my running to mornings – 6am works way better than evenings for me now. I'm training for the May marathon."
    ` + "```json" + `
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
    ` + "```" + `
    
    ### Relationship with multiple attributes
    **Input**: "Meeting with Sarah from product again tomorrow. She's basically my main collaborator these days – we sync every Tuesday."
    ` + "```json" + `
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
    ` + "```" + `
    
    ### Health fact (moderate sensitivity)
    **Input**: "Crushed my 10k run today in 48 minutes! My VO2 max is up to 52 according to my watch"
    ` + "```json" + `
    {
      "facts": [
        {
          "category": "health",
          "subject": "user",
          "attribute": "10k_time",
          "value": "completed 10k run in 48 minutes showing strong fitness level",
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
    ` + "```" + `
    
    ### Affective marker with high sensitivity
    **Input**: "Presentations always trigger my anxiety – happened again before the board meeting"
    ` + "```json" + `
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
    ` + "```" + `
    
    ### Negative example (no extraction)
    **Input**: "I guess I'm sort of a night owl these days, or maybe not, hard to say"
    ` + "```json" + `
    {
      "facts": []
    }
    ` + "```" + `
    Reason: Ambiguous, unstable claim (confidence below 7)
    
    ### Life milestone example (MUST extract)
    **Input**: "Finally signed the lease! Moving to Brooklyn next month"
    ` + "```json" + `
    {
      "facts": [{
        "category": "event",
        "subject": "user",
        "attribute": "relocation",
        "value": "moving to Brooklyn with lease signed",
        "temporal_context": "next month",
        "sensitivity": "medium",
        "importance": 3
      }]
    }
    ` + "```" + `
    
    ### Job change example (MUST extract)
    **Input**: "Got the offer! Starting as Senior Engineer at TechCorp in January"
    ` + "```json" + `
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
    ` + "```" + `
    
    ### Health milestone example (MUST extract)
    **Input**: "Doctor confirmed I'm fully recovered from the surgery – cleared for all activities"
    ` + "```json" + `
    {
      "facts": [{
        "category": "health",
        "subject": "user",
        "attribute": "recovery_status",
        "value": "fully recovered from surgery with doctor clearance for all activities",
        "sensitivity": "high",
        "importance": 3
      }]
    }
    ` + "```" + `
    
    ### Major purchase example (MUST extract)
    **Input**: "Just bought my first house! Closing was yesterday, keys in hand"
    ` + "```json" + `
    {
      "facts": [{
        "category": "event",
        "subject": "user",
        "attribute": "home_purchase",
        "value": "purchased first house with closing completed",
        "temporal_context": "yesterday",
        "sensitivity": "medium",
        "importance": 3
      }]
    }
    ` + "```" + `
    
    ### Mundane examples (DO NOT extract)
    **Input**: "Grabbed lunch at that new sandwich place downtown"
    ` + "```json" + `
    {
      "facts": []
    }
    ` + "```" + `
    Reason: One-off dining experience without lasting significance
    
    **Input**: "Feeling pretty tired today, long week"
    ` + "```json" + `
    {
      "facts": []
    }
    ` + "```" + `
    Reason: Temporary state, not a lasting pattern or significant development
    
    **Input**: "Thinking I might want to learn Spanish someday"
    ` + "```json" + `
    {
      "facts": []
    }
    ` + "```" + `
    Reason: Vague consideration without commitment or concrete plans
    
    ## Chunk metadata (DO NOT OUTPUT)
    ` + "```json" + `
    {
      "chunk_id": "conv_12345_chunk_3",
      "previous_facts_hash": "a7b9c2...",
      "timestamp": "2024-03-15T10:30:00Z"
    }
    ` + "```" + `
    
    ## Do NOT extract
    - Speculation or interpretation
    - One-off events without pattern
    - Quotes from others about the user
    - Temporary states (<2 weeks)
    - Granular timestamps
    - Value judgments
    
    ## Slack-specific guidelines
    
    When processing Slack conversations:
    - Extract user's work patterns and preferences, not project-specific details
    - For group conversations, focus on user's role and participation style
    - Extract collaboration patterns with specific people
    - Skip temporary project tasks, bug reports, or meeting logistics
    - Note: "user" refers to the primary person whose facts are being extracted
    `

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

	// TextFactExtractionPrompt - Extract facts about a person from any text content.
	TextFactExtractionPrompt = `You are a Personal Information Organizer. Extract comprehensive facts about the primary user from the provided text content.

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

	// ConversationMemoryUpdatePrompt - Comprehensive memory management decision system for conversations.
	ConversationMemoryUpdatePrompt = `You are a smart memory manager which controls the memory of a system for the primary user.
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

	// TextMemoryUpdatePrompt - Memory management optimized for text document context.
	TextMemoryUpdatePrompt = `You are a smart memory manager controlling the memory system for a user based on text content.
You can perform four operations: (1) add into the memory, (2) update the memory, (3) delete from the memory, and (4) no change.

CONTEXT: You are processing facts extracted from text content (emails, articles, notes, documents) that may be:
- Written BY the user (their own content)
- Written ABOUT the user (content mentioning them)
- Content that REFERENCES the user (documents, reports, conversations)

Compare newly retrieved facts with the existing memory. For each new fact, decide whether to:
- ADD: Add it to the memory as a new element
- UPDATE: Update an existing memory element
- DELETE: Delete an existing memory element  
- NONE: Make no change (if the fact is already present or irrelevant)

DECISION GUIDELINES:

1. **Add**: If the retrieved facts contain new information not present in the memory.
- Add facts about the user's preferences, activities, relationships, work, etc.
- Add factual information mentioned about the user
- Add temporal information (events, plans, experiences)

2. **Update**: If the retrieved facts contain more detailed or current information than existing memories.
- Update with more specific details when available
- Update outdated information with current facts
- Combine related information when it adds context
- Preserve the same memory ID when updating

3. **Delete**: If the retrieved facts directly contradict existing memories.
- Remove information that is explicitly contradicted
- Delete outdated facts when newer information conflicts
- Remove information that is proven incorrect

4. **No Change**: If the retrieved facts are already captured in existing memories.
- Skip duplicate information
- Ignore facts that don't add new value
- Leave unchanged when information is equivalent

PROCESSING NOTES:
- Consider that text content may reference the user in first, second, or third person
- Facts may be stated directly or implied from context
- Temporal context matters - newer information may override older facts
- Preserve important relationship and professional information
- Focus on actionable and meaningful facts about the user`
)
