package auth

import "time"

// TokenExchangeRequest represents the request model for token exchange.
type TokenExchangeRequest struct {
	GrantType    string `json:"grant_type" binding:"required"`
	Code         string `json:"code,omitempty"`
	CodeVerifier string `json:"code_verifier,omitempty"`
	Platform     string `json:"platform" binding:"required"`
	RefreshToken string `json:"refresh_token,omitempty"`
	RedirectURI  string `json:"redirect_uri,omitempty"`
}

// RefreshTokenRequest represents the request model for refresh token.
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
	Platform     string `json:"platform" binding:"required"`
}

// TokenRequest represents the parameters for token requests (both authorization and refresh).
type TokenRequest struct {
	GrantType    string
	Code         string
	RefreshToken string
	RedirectURI  string
	ClientID     string
	ClientSecret string
	CodeVerifier string
}

type TokenResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at"`
	Username     string    `json:"username,omitempty"`
	Platform     string    `json:"platform"`
}
