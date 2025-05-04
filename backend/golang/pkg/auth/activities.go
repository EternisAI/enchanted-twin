package auth

import (
	"context"

	"github.com/charmbracelet/log"
	"go.temporal.io/sdk/worker"

	"github.com/EternisAI/enchanted-twin/pkg/db"
)

func NewOAuthActivities(authStore *db.Store) *OAuthActivities {
	return &OAuthActivities{
		authStore: authStore,
	}
}

type OAuthActivities struct {
	authStore *db.Store
}

func (a *OAuthActivities) RegisterWorkflowsAndActivities(worker *worker.Worker) {
	(*worker).RegisterWorkflow(TokenRefreshWorkflow)
	(*worker).RegisterActivity(a.RefreshTokenActivity)
}

func (w *OAuthActivities) RefreshTokenActivity(
	ctx context.Context,
	provider string,
) (bool, error) {
	logger := log.Default()

	return RefreshOAuthToken(ctx, logger, w.authStore, provider)
}
