package auth

import (
	"context"

	"github.com/charmbracelet/log"
	"go.temporal.io/sdk/worker"

	"github.com/EternisAI/enchanted-twin/pkg/db"
)

func NewOAuthActivities(authStore *db.Store, logger *log.Logger) *OAuthActivities {
	return &OAuthActivities{
		authStore: authStore,
		logger:    logger,
	}
}

type OAuthActivities struct {
	authStore *db.Store
	logger    *log.Logger
}

func (a *OAuthActivities) RegisterWorkflowsAndActivities(worker *worker.Worker) {
	(*worker).RegisterWorkflow(TokenRefreshWorkflow)
	(*worker).RegisterActivity(a.RefreshTokenActivity)
}

func (w *OAuthActivities) RefreshTokenActivity(
	ctx context.Context,
	provider string,
) (bool, error) {
	return RefreshOAuthToken(ctx, w.logger, w.authStore, provider)
}
