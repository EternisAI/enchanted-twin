# Data processing pipeline: hallucination issues & fixes

## Overview
The data processing pipeline suffers from inconsistent user identification and fact attribution across different sources (Telegram, WhatsApp, Slack, Gmail, X/Twitter, ChatGPT), leading to hallucinated facts and incorrect memory attribution.

## Critical issues & solutions

### Issue 1: Fundamental misunderstanding of User field

**Problem**: Sources are either not setting the `User` field or misunderstanding its purpose:
- The `User` field should contain the **actual user identifier** (username, email, phone)
- NOT the literal string "primaryUser" 
- The normalization to "primaryUser" happens automatically in the `Content()` method

**Impact**: Without proper User field, the Content() method cannot normalize correctly, causing the LLM to see actual names instead of "primaryUser", leading to inconsistent fact attribution.

**Solution**: Set User field to actual user identifier in all sources

#### Implementation steps:
- [ ] **Telegram**: Ensure `User: extractedUsername` (e.g., "john_doe")
- [ ] **WhatsApp**: Set `User: detectedUserPhone` (e.g., "15551234567")
- [ ] **Slack**: Set `User: extractedSlackUsername`
- [ ] **Gmail**: Set `User: detectedEmail` (e.g., "user@example.com")
- [ ] **X/Twitter**: Set `User: extractedTwitterHandle`
- [ ] **ChatGPT**: Set `User: "chatgpt_user"` (consistent identifier)

### Issue 2: Missing or incomplete People list

**Problem**: The `People` list must include ALL conversation participants, including the primary user:
- WhatsApp doesn't populate People list properly
- ChatGPT missing People list entirely
- Some sources don't include the primary user in People

**Impact**: The Content() normalization cannot work if the User is not in the People list.

**Solution**: Ensure People list is complete with all participants

#### Implementation steps:
- [ ] **All sources**: Ensure `People` includes the User value
- [ ] **WhatsApp**: Build People list from all message participants
- [ ] **ChatGPT**: Set `People: ["chatgpt_user", "assistant"]`
- [ ] Add validation: User must be in People list

### Issue 3: Broken message attribution logic

**Problem**: Sources incorrectly identify which messages belong to the primary user:
- Slack's `myMessage` comparison bug (always false)
- WhatsApp's complex JID resolution fails
- Telegram doesn't use stored username consistently

**Impact**: Facts extracted from user's own messages get attributed to other people.

**Solution**: Fix message attribution logic in each source

#### Implementation steps:
- [ ] **Slack**: Fix `myMessage` comparison bug - compare with actual username
- [ ] **WhatsApp**: Use detected user phone for sender comparison
- [ ] **Telegram**: Use stored username for `myMessage` detection
- [ ] **Gmail**: Use detected email for proper `user_role` attribution
- [ ] **X/Twitter**: Use account username to identify user's content

### Issue 4: User detection failures

**Problem**: Each source uses different, often unreliable methods to detect the primary user:
- WhatsApp has no user detection
- Slack compares with empty string
- Gmail frequency analysis can be wrong

**Impact**: Wrong user detection cascades to all other issues.

**Solution**: Implement reliable user detection for each source

#### Implementation steps:
- [ ] **WhatsApp**: Implement user detection from message patterns
- [ ] **Slack**: Extract user from export metadata
- [ ] **Gmail**: Cache detected email after first detection
- [ ] **Telegram**: Validate username exists before using
- [ ] Store detected user in database for consistency

### Issue 5: Validation too late in pipeline

**Problem**: No validation at document creation time - issues only discovered during fact extraction.

**Impact**: Invalid documents propagate through the system.

**Solution**: Add immediate validation at document creation

#### Implementation steps:
- [ ] Create `ValidateConversationDocument()` function
- [ ] Validate: User field is not empty
- [ ] Validate: User field is not "primaryUser" (common mistake)
- [ ] Validate: User is in People list
- [ ] Call validation before returning any ConversationDocument

### Issue 6: Context loss in document chunking

**Problem**: When conversations are chunked, User and People information isn't properly propagated.

**Impact**: Some chunks lose user context, leading to incorrect fact attribution.

**Solution**: Preserve complete document structure in all chunks

#### Implementation steps:
- [ ] Ensure `User` field propagates to all chunks
- [ ] Ensure `People` list propagates to all chunks
- [ ] Validate chunks have same structure as parent

## Correct implementation example

```go
// WRONG - Common mistakes
conversation := &memory.ConversationDocument{
    User: "primaryUser",  // ❌ Should be actual identifier
    People: []string{"Alice", "Bob"}, // ❌ Missing the user
}

// CORRECT - Proper implementation
conversation := &memory.ConversationDocument{
    User: "alice@example.com", // ✅ Actual user identifier
    People: []string{"alice@example.com", "bob@example.com"}, // ✅ Includes user
    // Content() method will automatically normalize "alice@example.com" to "primaryUser"
}
```

## Validation function

```go
func ValidateConversationDocument(doc *ConversationDocument) error {
    // Check User field is set
    if doc.User == "" {
        return fmt.Errorf("User field cannot be empty")
    }
    
    // Check User is not the literal "primaryUser"
    if doc.User == "primaryUser" {
        return fmt.Errorf("User field should contain actual identifier, not 'primaryUser'")
    }
    
    // Check User is in People list
    userInPeople := false
    for _, person := range doc.People {
        if person == doc.User {
            userInPeople = true
            break
        }
    }
    if !userInPeople {
        return fmt.Errorf("User '%s' must be in People list", doc.User)
    }
    
    // Check People list is not empty
    if len(doc.People) == 0 {
        return fmt.Errorf("People list cannot be empty")
    }
    
    return nil
}
```

## Fixes checklist

### Critical fixes (immediate):
- [ ] Fix Slack `myMessage` comparison bug
- [ ] Add User field to WhatsApp conversations
- [ ] Add User field to ChatGPT conversations
- [ ] Add People list to ChatGPT conversations
- [ ] Fix all sources to use actual identifiers, not "primaryUser"

### Infrastructure fixes:
- [ ] Add ValidateConversationDocument function
- [ ] Add validation at document creation in all sources
- [ ] Create unified user detection helpers
- [ ] Ensure User is always in People list

### Quality improvements:
- [ ] Add logging for user detection
- [ ] Add metrics for validation failures
- [ ] Create integration tests for each source
- [ ] Document the User vs primaryUser distinction

## Validation checklist

After implementing fixes, verify:
- [ ] Each source sets User to actual identifier (NOT "primaryUser")
- [ ] All conversation documents include User in People list
- [ ] Content() method correctly normalizes to "primaryUser"
- [ ] Facts are attributed to "primaryUser" not actual names
- [ ] Message attribution correctly identifies user's messages
- [ ] Chunks preserve User and People fields

## Success metrics

- Zero instances of User field containing "primaryUser"
- 100% of conversation documents have non-empty User field with actual identifier
- 100% of conversation documents include User in People list
- Close to zero facts attributed to wrong subjects
- Consistent contact name resolution across sources
- Reliable timestamp accuracy (within expected ranges)