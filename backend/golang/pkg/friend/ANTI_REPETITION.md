# Friend Anti-Repetition System

This document describes the anti-repetition system implemented to prevent the twin from sending similar messages repeatedly.

## Overview

The system tracks all messages sent by the friend workflow and uses semantic similarity to prevent sending messages that are too similar to previously sent ones.

## Key Components

### 1. Message Storage

All sent messages are stored in Weaviate with special metadata:

```go
doc := memory.TextDocument{
    Content:   message,
    Timestamp: &now,
    Tags:      []string{"sent_message", activityType},
    Metadata: map[string]string{
        "type":          "friend",           // Identifies friend messages
        "activity_type": activityType,       // "poke_message", "memory_picture", or "question"
        "sent_at":       now.Format(time.RFC3339),
    },
}
```

### 2. Similarity Checking

Before sending any message, the system:

1. Generates an embedding for the new message
2. Searches Weaviate for similar content using `QueryWithDistance`
3. Filters results to only include messages with `metadata.type = "friend"`
4. Checks if any similar message has a distance below the threshold

### 3. Configuration

```go
const (
    SimilarityThreshold = 0.15  // Messages with distance < 0.15 are considered too similar
    FriendMetadataType  = "friend"  // Metadata type for friend messages
)
```

### 4. New Methods

#### `StoreSentMessage(ctx, message, activityType)`
- Stores a sent message in Weaviate with friend metadata
- Called after successfully sending a message
- Non-blocking - errors are logged but don't fail the workflow

#### `CheckForSimilarMessages(ctx, message)`
- Searches for similar messages using semantic similarity
- Returns `true` if a similar message is found (distance < threshold)
- Returns `false` if no similar messages or memory service unavailable

## Integration with Workflows

### Poke Messages

```go
func (s *FriendService) SendPokeMessage(ctx context.Context, message string) error {
    // Check for similarity before sending
    isSimilar, err := s.CheckForSimilarMessages(ctx, message)
    if err != nil {
        s.logger.Error("Failed to check for similar messages", "error", err)
    }

    if isSimilar {
        s.logger.Info("Skipping poke message due to similarity with previous messages")
        return nil  // Skip sending, but don't fail the workflow
    }

    // Send the message
    _, err = s.twinchatService.SendAssistantMessage(ctx, "", message)
    if err != nil {
        return fmt.Errorf("failed to send poke message: %w", err)
    }

    // Store the sent message for future similarity checks
    err = s.StoreSentMessage(ctx, message, "poke_message")
    if err != nil {
        s.logger.Error("Failed to store sent poke message", "error", err)
    }

    return nil
}
```

### Memory Pictures

Similar integration for memory picture messages, checking similarity before sending and storing after successful send.

### Questions

```go
func (s *FriendService) SendQuestion(ctx context.Context, input SendQuestionInput) error {
    question, err := s.GetRandomQuestion(ctx)
    if err != nil {
        return fmt.Errorf("failed to get random question: %w", err)
    }

    // Check for similarity before sending
    isSimilar, err := s.CheckForSimilarMessages(ctx, question)
    if err != nil {
        s.logger.Error("Failed to check for similar messages", "error", err)
    }

    if isSimilar {
        s.logger.Info("Skipping question due to similarity with previous messages")
        return nil
    }

    // Send the question
    _, err = s.twinchatService.SendAssistantMessage(ctx, input.ChatID, question)
    if err != nil {
        return fmt.Errorf("failed to send question: %w", err)
    }

    // Store the sent question for future similarity checks
    err = s.StoreSentMessage(ctx, question, "question")
    if err != nil {
        s.logger.Error("Failed to store sent question", "error", err)
    }

    return nil
}
```

## Benefits

1. **Prevents Repetition**: Users won't receive the same or very similar messages repeatedly
2. **Maintains Engagement**: Keeps interactions fresh and interesting
3. **Graceful Degradation**: If memory service is unavailable, messages are still sent
4. **Non-Blocking**: Similarity check failures don't prevent message sending
5. **Configurable**: Similarity threshold can be adjusted based on requirements

## Monitoring

The system logs important events:

- When similar messages are found and sending is skipped
- When messages are successfully stored
- When similarity checks fail
- Number of documents checked during similarity search

## Testing

Comprehensive tests verify:

- Similar messages are detected correctly
- Different messages are allowed through
- Non-friend messages don't interfere with similarity checks
- Message storage works correctly with proper metadata

## Future Enhancements

1. **Time-based Decay**: Allow similar messages after a certain time period
2. **User-specific Tracking**: Track similarity per user/chat
3. **Activity-specific Thresholds**: Different thresholds for different activity types
4. **Similarity Metrics**: Track and analyze similarity patterns
5. **Manual Override**: Allow forcing a message even if similar ones exist 