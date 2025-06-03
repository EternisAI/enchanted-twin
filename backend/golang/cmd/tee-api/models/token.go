package models

import (
	"time"
)

// TokenResponse represents the structure of OAuth token response
type TokenResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at"`
	Username     string    `json:"username,omitempty"`
	Platform     string    `json:"platform"`
}

// TokenExchangeRequest represents the request structure for token exchange endpoint
type TokenExchangeRequest struct {
	GrantType    string `json:"grant_type" binding:"required"`
	Code         string `json:"code,omitempty"`
	CodeVerifier string `json:"code_verifier,omitempty"`
	Platform     string `json:"platform" binding:"required"`
	RefreshToken string `json:"refresh_token,omitempty"`
	RedirectURI  string `json:"redirect_uri,omitempty"`
}

// RefreshTokenRequest represents the request structure for token refresh
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
	Platform     string `json:"platform" binding:"required"`
}

// OAuthURLRequest represents the request structure for generating OAuth URLs
type OAuthURLRequest struct {
	Platform    string `json:"platform" binding:"required"`
	RedirectURI string `json:"redirect_uri" binding:"required"`
	State       string `json:"state,omitempty"`
	Scopes      string `json:"scopes,omitempty"`
}

// OAuthURLResponse represents the response structure for OAuth URL generation
type OAuthURLResponse struct {
	AuthURL  string `json:"auth_url"`
	Platform string `json:"platform"`
	State    string `json:"state,omitempty"`
}

// ErrorResponse represents error response structure
type ErrorResponse struct {
	Error       string `json:"error"`
	Description string `json:"error_description,omitempty"`
	Code        int    `json:"code"`
}

// OAuthConfig represents OAuth configuration for a provider
type OAuthConfig struct {
	TokenEndpoint string
	ClientID      string
	ClientSecret  string
	RedirectURI   string
}

// UserInfo represents basic user information from OAuth providers
type UserInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name,omitempty"`
	Email    string `json:"email,omitempty"`
	Avatar   string `json:"avatar,omitempty"`
	Platform string `json:"platform"`
}
