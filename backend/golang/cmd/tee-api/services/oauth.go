package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"oauth-proxy/config"
	"oauth-proxy/models"
)

type OAuthService struct {
	logger *log.Logger
}

func NewOAuthService() *OAuthService {
	return &OAuthService{
		logger: log.New(log.Writer(), "[OAuth] ", log.LstdFlags),
	}
}

// getOAuthConfig returns the OAuth configuration for a given provider
func (s *OAuthService) getOAuthConfig(provider string) (models.OAuthConfig, error) {
	switch provider {
	case "google":
		return models.OAuthConfig{
			TokenEndpoint: "https://oauth2.googleapis.com/token",
			ClientID:      config.AppConfig.GoogleClientID,
			ClientSecret:  config.AppConfig.GoogleClientSecret,
			RedirectURI:   "http://localhost:8080/auth/google/callback",
		}, nil
	case "slack":
		return models.OAuthConfig{
			TokenEndpoint: "https://slack.com/api/oauth.v2.access",
			ClientID:      config.AppConfig.SlackClientID,
			ClientSecret:  config.AppConfig.SlackClientSecret,
			RedirectURI:   "http://localhost:8080/auth/slack/callback",
		}, nil
	case "twitter":
		return models.OAuthConfig{
			TokenEndpoint: "https://api.twitter.com/2/oauth2/token",
			ClientID:      config.AppConfig.TwitterClientID,
			ClientSecret:  "",
			RedirectURI:   "http://localhost:8080/auth/twitter/callback",
		}, nil
	default:
		return models.OAuthConfig{}, fmt.Errorf("unsupported provider: %s", provider)
	}
}

// ExchangeToken exchanges authorization code for access token or refreshes a token
func (s *OAuthService) ExchangeToken(req models.TokenExchangeRequest) (*models.TokenResponse, error) {
	oauthConfig, err := s.getOAuthConfig(req.Platform)
	if err != nil {
		return nil, err
	}

	// Prepare request data
	data := url.Values{}
	data.Set("grant_type", req.GrantType)
	data.Set("client_id", oauthConfig.ClientID)

	// Set appropriate params based on grant type
	switch req.GrantType {
	case "authorization_code":
		data.Set("code", req.Code)
		data.Set("redirect_uri", req.RedirectURI)
		if req.CodeVerifier != "" {
			data.Set("code_verifier", req.CodeVerifier)
		}
	case "refresh_token":
		data.Set("refresh_token", req.RefreshToken)
	default:
		return nil, fmt.Errorf("unsupported grant type: %s", req.GrantType)
	}

	// Add client secret if available
	if oauthConfig.ClientSecret != "" {
		data.Set("client_secret", oauthConfig.ClientSecret)
	}

	// Track time before request for accurate expiry calculation
	timeBeforeTokenRequest := time.Now()

	// Create and execute request
	ctx := context.Background()
	httpReq, err := http.NewRequestWithContext(
		ctx,
		"POST",
		oauthConfig.TokenEndpoint,
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send token request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			s.logger.Printf("failed to close token response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to obtain token: %d: %s", resp.StatusCode, body)
	}

	// Parse token response based on provider
	var tokenResp models.TokenResponse
	var expiresIn int

	if req.Platform == "slack" {
		// Special handling for Slack's response format
		var slackTokenResp struct {
			OK         bool `json:"ok"`
			AuthedUser struct {
				ID          string `json:"id"`
				AccessToken string `json:"access_token"`
				TokenType   string `json:"token_type"`
			} `json:"authed_user"`
			AccessToken string `json:"access_token"`
			TokenType   string `json:"token_type"`
		}

		body, _ := io.ReadAll(resp.Body)

		// Print raw response for debugging
		fmt.Printf("Raw slack token response: %s\n", string(body))

		// Reset the reader for JSON decoding
		resp.Body = io.NopCloser(bytes.NewBuffer(body))

		if err := json.NewDecoder(resp.Body).Decode(&slackTokenResp); err != nil {
			return nil, fmt.Errorf("failed to parse slack token response: %w", err)
		}

		// First try authed_user.access_token
		if slackTokenResp.AuthedUser.AccessToken != "" {
			tokenResp.Username = slackTokenResp.AuthedUser.ID
			tokenResp.AccessToken = slackTokenResp.AuthedUser.AccessToken
			tokenResp.TokenType = slackTokenResp.AuthedUser.TokenType
			if tokenResp.TokenType == "" {
				tokenResp.TokenType = "Bearer"
			}
		} else if slackTokenResp.AccessToken != "" {
			// Fall back to top-level access_token
			tokenResp.AccessToken = slackTokenResp.AccessToken
			tokenResp.TokenType = slackTokenResp.TokenType
			if tokenResp.TokenType == "" {
				tokenResp.TokenType = "Bearer"
			}
		}
		// No expiry: set to approx 10 years
		expiresIn = 10 * 365 * 24 * 3600
	} else {
		// Standard OAuth token response
		var stdResp struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token,omitempty"`
			TokenType    string `json:"token_type"`
			ExpiresIn    int    `json:"expires_in,omitempty"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&stdResp); err != nil {
			return nil, fmt.Errorf("failed to parse token response: %w", err)
		}
		tokenResp.AccessToken = stdResp.AccessToken
		tokenResp.RefreshToken = stdResp.RefreshToken
		tokenResp.TokenType = stdResp.TokenType
		expiresIn = stdResp.ExpiresIn
	}

	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("no access token received")
	}

	if expiresIn < 60 {
		return nil, fmt.Errorf("access token expiry too soon: %ds", expiresIn)
	}

	// Calculate expiration
	tokenResp.ExpiresAt = timeBeforeTokenRequest.Add(time.Duration(expiresIn) * time.Second)
	tokenResp.Platform = req.Platform

	return &tokenResp, nil
}

// RefreshToken refreshes an existing access token
func (s *OAuthService) RefreshToken(platform, refreshToken string) (*models.TokenResponse, error) {
	if refreshToken == "" {
		return nil, fmt.Errorf("refresh token is required")
	}

	// Use the same ExchangeToken function with refresh_token grant type
	req := models.TokenExchangeRequest{
		GrantType:    "refresh_token",
		Platform:     platform,
		RefreshToken: refreshToken,
	}

	tokenResp, err := s.ExchangeToken(req)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	return tokenResp, nil
}
