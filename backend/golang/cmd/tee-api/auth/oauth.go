package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
)

// TokenInfo represents the response from Google's tokeninfo endpoint
type TokenInfo struct {
	Audience      string `json:"aud"`
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"`
	ExpiresIn     string `json:"expires_in"`
	Scope         string `json:"scope"`
}

// VerifyGoogleAccessToken verifies a Google access token and returns user email
func VerifyGoogleAccessToken(accessToken string) (tokenInfo *TokenInfo, err error) {
	// Use Google's tokeninfo endpoint to verify the token
	tokenInfoURL := fmt.Sprintf("https://oauth2.googleapis.com/tokeninfo?access_token=%s",
		url.QueryEscape(accessToken))

	resp, httpErr := http.Get(tokenInfoURL)
	if httpErr != nil {
		return nil, fmt.Errorf("failed to verify token: %v", httpErr)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid or expired access token")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	var token TokenInfo
	if unmarshalErr := json.Unmarshal(body, &token); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to parse token info: %v", unmarshalErr)
	}

	// Check if token was issued for your client ID
	expectedClientID := os.Getenv("GOOGLE_CLIENT_ID")

	if expectedClientID == "" {
		return nil, fmt.Errorf("GOOGLE_CLIENT_ID is not set")
	}

	if token.Audience != expectedClientID {
		return nil, fmt.Errorf("token audience mismatch")
	}

	// Confirm email is present and verified
	if token.Email == "" || token.EmailVerified != "true" {
		return nil, fmt.Errorf("email scope missing or not verified")
	}

	return &token, nil
}
