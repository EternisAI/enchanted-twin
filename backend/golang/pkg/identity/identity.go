package identity

import (
	"context"
	"fmt"
	"strings"

	"go.temporal.io/sdk/client"
)

func NewIdentityService(temporalClient client.Client) *IdentityService {
	return &IdentityService{
		temporalClient: temporalClient,
	}
}

type IdentityService struct {
	temporalClient client.Client
}

func (s *IdentityService) GetUserProfile(ctx context.Context) (string, error) {
	scheduleHandle := s.temporalClient.ScheduleClient().GetHandle(ctx, PersonalityWorkflowID)

	desc, err := scheduleHandle.Describe(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "schedule not found") || strings.Contains(err.Error(), "not found") {
			return "", nil
		}
		return "", err
	}

	if len(desc.Info.RecentActions) == 0 {
		return "", nil
	}

	recentAction := desc.Info.RecentActions[len(desc.Info.RecentActions)-1]
	if recentAction.StartWorkflowResult == nil {
		return "", nil
	}

	workflowID := recentAction.StartWorkflowResult.WorkflowID
	runID := recentAction.StartWorkflowResult.FirstExecutionRunID

	fmt.Println("==workflowID", workflowID)
	fmt.Println("==runID", runID)

	var output DerivePersonalityOutput
	run := s.temporalClient.GetWorkflow(ctx, workflowID, runID)
	err = run.Get(ctx, &output)
	if err != nil {
		if strings.Contains(err.Error(), "workflow not found") {
			return "", nil
		}
		return "", err
	}

	return output.Personality, nil
}
