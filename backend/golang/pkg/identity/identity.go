package identity

import (
	"context"
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

func (s *IdentityService) GetPersonality(ctx context.Context) (string, error) {
	var output DerivePersonalityOutput
	run := s.temporalClient.GetWorkflow(ctx, PersonalityWorkflowID, "")
	err := run.Get(ctx, &output)
	if err != nil {
		if strings.Contains(err.Error(), "workflow not found") {
			return "", nil
		}
		return "", err
	}
	return output.Personality, nil
}
