// Owner: slimane@eternis.ai

package friend

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	MinWaitSeconds = 1
	MaxWaitSeconds = 10
)

type ActivityType string

const (
	ActivityTypePokeMessage   ActivityType = "poke_message"
	ActivityTypeMemoryPicture ActivityType = "memory_picture"
)

type FriendWorkflowInput struct {
	UserIdentity string `json:"user_identity,omitempty"`
	ChatID       string `json:"chat_id,omitempty"`
}

type FriendWorkflowOutput struct {
	ActivityType        ActivityType `json:"activity_type"`
	PokeMessageSent     bool         `json:"poke_message_sent"`
	MemoryPictureSent   bool         `json:"memory_picture_sent"`
	UserResponseTracked bool         `json:"user_response_tracked"`
	ChatID              string       `json:"chat_id"`
	Error               string       `json:"error,omitempty"`
}

func (s *FriendService) FriendWorkflow(ctx workflow.Context, input *FriendWorkflowInput) (FriendWorkflowOutput, error) {
	logger := workflow.GetLogger(ctx)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	output := FriendWorkflowOutput{
		ChatID: input.ChatID,
	}

	// Generate random wait duration using activity
	var waitOutput GenerateRandomWaitOutput
	err := workflow.ExecuteActivity(ctx, s.GenerateRandomWait, GenerateRandomWaitInput{
		MinSeconds: MinWaitSeconds,
		MaxSeconds: MaxWaitSeconds,
	}).Get(ctx, &waitOutput)
	if err != nil {
		logger.Error("Failed to generate random wait", "error", err)
		output.Error = err.Error()
		return output, err
	}

	// Random wait at the beginning
	waitDuration := time.Duration(waitOutput.WaitDurationSeconds) * time.Second
	logger.Info("Starting friend workflow with random wait", "wait_duration", waitDuration)
	err = workflow.Sleep(ctx, waitDuration)
	if err != nil {
		logger.Error("Failed to sleep", "error", err)
		output.Error = err.Error()
		return output, err
	}

	// Fetch identity and memories at workflow level
	var identity string
	err = workflow.ExecuteActivity(ctx, s.FetchIdentity).Get(ctx, &identity)
	if err != nil {
		logger.Error("Failed to fetch identity", "error", err)
		output.Error = err.Error()
		return output, err
	}

	var memories string
	err = workflow.ExecuteActivity(ctx, s.FetchMemory).Get(ctx, &memories)
	if err != nil {
		logger.Error("Failed to fetch memories", "error", err)
		output.Error = err.Error()
		return output, err
	}

	availableActivities := []string{string(ActivityTypePokeMessage), string(ActivityTypeMemoryPicture)}
	var activityOutput SelectRandomActivityOutput
	err = workflow.ExecuteActivity(ctx, s.SelectRandomActivity, SelectRandomActivityInput{
		AvailableActivities: availableActivities,
	}).Get(ctx, &activityOutput)
	if err != nil {
		logger.Error("Failed to select random activity", "error", err)
		output.Error = err.Error()
		return output, err
	}

	selectedActivity := ActivityType(activityOutput.SelectedActivity)
	output.ActivityType = selectedActivity

	logger.Info("Selected activity", "activity", selectedActivity)

	switch selectedActivity {
	case ActivityTypePokeMessage:
		err = s.executePokeMessageActivity(ctx, input, &output, identity, memories)
	case ActivityTypeMemoryPicture:
		err = s.executeMemoryPictureActivity(ctx, input, &output, identity, memories)
	}

	if err != nil {
		logger.Error("Failed to execute activity", "activity", selectedActivity, "error", err)
		output.Error = err.Error()
		return output, err
	}

	// Track user response
	err = workflow.ExecuteActivity(ctx, s.TrackUserResponse, TrackUserResponseInput{
		ChatID:       output.ChatID,
		ActivityType: string(selectedActivity),
		Timestamp:    workflow.Now(ctx),
	}).Get(ctx, nil)
	if err != nil {
		logger.Error("Failed to track user response", "error", err)
		// Don't fail the workflow for tracking errors
	} else {
		output.UserResponseTracked = true
	}

	return output, nil
}

func (s *FriendService) executePokeMessageActivity(ctx workflow.Context, input *FriendWorkflowInput, output *FriendWorkflowOutput, identity, memories string) error {
	logger := workflow.GetLogger(ctx)

	var pokeMessage string
	err := workflow.ExecuteActivity(ctx, s.GeneratePokeMessage, GeneratePokeMessageInput{
		Identity: identity,
		Memories: memories,
	}).Get(ctx, &pokeMessage)
	if err != nil {
		logger.Error("Failed to generate poke message", "error", err)
		return err
	}

	err = workflow.ExecuteActivity(ctx, s.SendPokeMessage, pokeMessage).Get(ctx, nil)
	if err != nil {
		logger.Error("Failed to send poke message", "error", err)
		return err
	}
	output.PokeMessageSent = true

	return nil
}

func (s *FriendService) executeMemoryPictureActivity(ctx workflow.Context, input *FriendWorkflowInput, output *FriendWorkflowOutput, identity, memories string) error {
	logger := workflow.GetLogger(ctx)

	var pictureDescription string
	err := workflow.ExecuteActivity(ctx, s.GenerateMemoryPicture, GenerateMemoryPictureInput{
		Identity:     identity,
		RandomMemory: memories,
	}).Get(ctx, &pictureDescription)
	if err != nil {
		logger.Error("Failed to generate memory picture", "error", err)
		return err
	}

	sendInput := SendMemoryPictureInput{
		ChatID:             input.ChatID,
		PictureDescription: pictureDescription,
	}
	err = workflow.ExecuteActivity(ctx, s.SendMemoryPicture, sendInput).Get(ctx, nil)
	if err != nil {
		logger.Error("Failed to send memory picture", "error", err)
		return err
	}
	output.MemoryPictureSent = true

	return nil
}
