package google

import (
	"context"
	"fmt"

	"github.com/EternisAI/enchanted-twin/pkg/db"
)

type TimeRange struct {
	From uint64 `json:"from" jsonschema:",description=The start timestamp in seconds of the time range, default is 0"`
	To   uint64 `json:"to"   jsonschema:",description=The end timestamp in seconds of the time range, default is 0"`
}



func GetAccessToken(ctx context.Context, store *db.Store, emailAccount string) (string, error) {
	oauthTokens, err := store.GetOAuthTokensArray(ctx, "google")
	if err != nil {
		return "", err
	}
	var accessToken string
	for _, oauthToken := range oauthTokens {
		if oauthToken.Username == emailAccount {
			accessToken = oauthToken.AccessToken
			break
		}
	}
	if accessToken == "" {
		return "", fmt.Errorf("email account not found")
	}
	return accessToken, nil
}
