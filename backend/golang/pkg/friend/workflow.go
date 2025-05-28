// Owner: slimane@eternis.ai

package friend

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type FriendWorkflowInput struct{}

type FriendWorkflowOutput struct {
	PokeMessageSent   bool   `json:"poke_message_sent"`
	MemoryPictureSent bool   `json:"memory_picture_sent"`
	Error             string `json:"error,omitempty"`
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

	output := FriendWorkflowOutput{}

	// var pokeMessage string
	// err := workflow.ExecuteActivity(ctx, s.GeneratePokeMessage).Get(ctx, &pokeMessage)
	// if err != nil {
	// 	logger.Error("Failed to generate poke message", "error", err)
	// 	output.Error = err.Error()
	// 	return output, err
	// }

	// err = workflow.ExecuteActivity(ctx, s.SendPokeMessage, pokeMessage).Get(ctx, nil)
	// if err != nil {
	// 	logger.Error("Failed to send poke message", "error", err)
	// 	output.Error = err.Error()
	// 	return output, err
	// }
	// output.PokeMessageSent = true

	var pictureDescription string
	err := workflow.ExecuteActivity(ctx, s.GenerateMemoryPicture).Get(ctx, &pictureDescription)
	if err != nil {
		logger.Error("Failed to generate memory picture", "error", err)

		logger.Warn("Continuing without memory picture")
	} else {
		input := SendMemoryPictureInput{
			ChatID:             "",
			PictureDescription: pictureDescription,
		}
		err = workflow.ExecuteActivity(ctx, s.SendMemoryPicture, input).Get(ctx, nil)
		if err != nil {
			logger.Error("Failed to send memory picture", "error", err)

			logger.Warn("Failed to send memory picture, but poke message was sent successfully")
		} else {
			output.MemoryPictureSent = true
		}
	}

	return output, nil
}
