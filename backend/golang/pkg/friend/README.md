# Friend Workflow Improvements

This document describes the improvements made to the friend workflow system.

## New Features

### 1. Random Wait at Workflow Start

The workflow now includes a random wait period at the beginning to make the friend interactions feel more natural and less predictable.

- **Constants**: `MinWaitSeconds = 1`, `MaxWaitSeconds = 10`
- **Implementation**: Uses `GenerateRandomWait` activity followed by `workflow.Sleep()` with the generated duration
- **Purpose**: Prevents the workflow from feeling robotic and adds natural timing variation
- **Deterministic**: Uses Temporal activities to ensure workflow determinism

### 2. Random Activity Selection

Instead of always executing the same activity, the workflow now randomly selects from available activities:

- **Available Activities**:
  - `ActivityTypePokeMessage`: Sends a friendly poke message
  - `ActivityTypeMemoryPicture`: Generates and sends a memory-based picture
- **Implementation**: Uses `SelectRandomActivity` activity to deterministically select an activity type
- **Extensibility**: New activities can be easily added to the `availableActivities` slice
- **Deterministic**: Uses Temporal activities to ensure workflow determinism

### 3. Improved Message Prompts

The prompts have been enhanced to be more fun, witty, and authentic:

- **Poke Messages**: Now use casual, friend-like language with humor and genuine curiosity
- **Avoids Sycophantic Language**: No more generic "hope you're doing well" or overly enthusiastic messages
- **Personality Styles**: Includes cheeky observations, random questions, theories, and hot takes
- **Memory Pictures**: Uses varied, casual message styles when sharing generated images
- **Authentic Voice**: Sounds like a real friend who knows you, not a customer service bot

### 4. User Response Tracking

The workflow now tracks user interactions and stores them in the database for analytics and personalization:

- **Database Table**: `friend_activity_tracking`
- **Tracked Data**:
  - Chat ID
  - Activity type
  - Timestamp
  - Creation time
- **Storage Methods**:
  - `StoreFriendActivity()`: Store a new activity
  - `GetFriendActivitiesByChatID()`: Get activities for a specific chat
  - `GetRecentFriendActivities()`: Get recent activities across all chats

### 5. Enhanced Input/Output Structure

The workflow now supports more detailed input and output:

**Input (`FriendWorkflowInput`)**:
- `UserIdentity`: User identity information (for future use)
- `ChatID`: Specific chat to send messages to

**Output (`FriendWorkflowOutput`)**:
- `ActivityType`: Which activity was executed
- `PokeMessageSent`: Whether poke message was sent successfully
- `MemoryPictureSent`: Whether memory picture was sent successfully
- `UserResponseTracked`: Whether the response was tracked successfully
- `ChatID`: The chat ID used
- `Error`: Any error that occurred

### 6. Temporal Workflow Determinism

The workflow follows Temporal best practices for determinism:

- **Random Generation Activities**: 
  - `GenerateRandomWait`: Generates random wait duration
  - `SelectRandomActivity`: Selects random activity type
- **No Direct Random Calls**: All randomness is generated in activities, not in the workflow
- **Reproducible**: Workflow execution is deterministic and can be replayed

## Database Schema

```sql
CREATE TABLE IF NOT EXISTS friend_activity_tracking (
    id TEXT PRIMARY KEY,
    chat_id TEXT NOT NULL,
    activity_type TEXT NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_friend_activity_chat_id ON friend_activity_tracking(chat_id);
CREATE INDEX IF NOT EXISTS idx_friend_activity_timestamp ON friend_activity_tracking(timestamp DESC);
```

## Activities

### Core Activities
- `FetchMemory`: Retrieves user memories
- `GeneratePokeMessage`: Creates a friendly poke message
- `SendPokeMessage`: Sends the poke message
- `GenerateMemoryPicture`: Creates a memory-based picture description
- `SendMemoryPicture`: Generates and sends a memory picture
- `TrackUserResponse`: Stores activity tracking data

### Random Generation Activities (for Determinism)
- `GenerateRandomWait`: Generates random wait duration between min/max seconds
- `SelectRandomActivity`: Selects random activity from available options

## Usage Example

```go
// Create friend service with all dependencies
friendService := friend.NewFriendService(friend.FriendServiceConfig{
    Logger:          logger,
    MemoryService:   memoryService,
    IdentityService: identityService,
    TwinchatService: twinchatService,
    AiService:       aiService,
    ToolRegistry:    toolRegistry,
    Store:           store, // Required for user response tracking
})

// Execute workflow
input := &friend.FriendWorkflowInput{
    UserIdentity: "user123",
    ChatID:       "chat456",
}

output, err := friendService.FriendWorkflow(ctx, input)
if err != nil {
    log.Error("Workflow failed", "error", err)
    return
}

log.Info("Workflow completed", 
    "activity_type", output.ActivityType,
    "poke_sent", output.PokeMessageSent,
    "picture_sent", output.MemoryPictureSent,
    "tracked", output.UserResponseTracked)
```

## Future Enhancements

1. **More Activity Types**: Add activities like sending quotes, asking questions, sharing news, etc.
2. **Personalization**: Use tracking data to personalize activity selection based on user preferences
3. **Scheduling**: Implement smart scheduling based on user activity patterns
4. **Response Analysis**: Analyze user responses to improve future interactions
5. **A/B Testing**: Test different message types and timing strategies
6. **Weighted Random Selection**: Implement weighted random selection based on user preferences

## Configuration

The workflow uses constants for timing that can be adjusted:

```go
const (
    MinWaitSeconds = 1  // Minimum wait time in seconds
    MaxWaitSeconds = 10 // Maximum wait time in seconds
)
```

For production, you might want to increase these values to make interactions feel more natural (e.g., 300-3600 seconds for 5 minutes to 1 hour).

## Temporal Best Practices

This implementation follows Temporal workflow best practices:

1. **Deterministic Execution**: All random generation is done in activities
2. **Idempotent Activities**: Activities can be safely retried
3. **Proper Error Handling**: Errors are properly propagated and logged
4. **Activity Timeouts**: All activities have appropriate timeouts
5. **Retry Policies**: Activities have retry policies for resilience 